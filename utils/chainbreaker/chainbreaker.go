package chainbreaker

// Logic ported from https://github.com/n0fate/chainbreaker

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	atomSize                         = 4
	headerSize                       = 20
	schemaSize                       = 8
	tableHeaderSize                  = 28
	keyBlobRecordHeaderSize          = 132
	keyBlobStructSize                = 24
	genericPasswordHeaderSize        = 22 * 4
	blockSize                        = 8
	keyLength                        = 24
	metadataOffsetAdjustment         = 0x38
	keyBlobMagic              uint32 = 0xFADE0711
	keychainSignature                = "kych"
	secureStorageGroup               = "ssgp"
	keychainLockedSignature          = "[Invalid Password / Keychain Locked]"
)

const (
	cssmDBRecordTypeAppDefinedStart uint32 = 0x80000000
	cssmGenericPassword                    = cssmDBRecordTypeAppDefinedStart + 0
	cssmMetadata                           = cssmDBRecordTypeAppDefinedStart + 0x8000
	cssmDBRecordTypeOpenGroupStart  uint32 = 0x0000000A
	cssmSymmetricKey                       = cssmDBRecordTypeOpenGroupStart + 7
)

const dbBlobSize = 92

var magicCMSIV = []byte{0x4a, 0xdd, 0xa2, 0x2c, 0x79, 0xe8, 0x21, 0x05}

type Keychain struct {
	buf       []byte
	header    applDBHeader
	tableList []uint32
	tableEnum map[uint32]int
	dbblob    dbBlob
	baseAddr  int
	dbKey     []byte
	keyList   map[string][]byte
}

type applDBHeader struct {
	Signature    [4]byte
	Version      uint32
	HeaderSize   uint32
	SchemaOffset uint32
	AuthOffset   uint32
}

type applDBSchema struct {
	SchemaSize uint32
	TableCount uint32
}

type tableHeader struct {
	TableSize          uint32
	TableID            uint32
	RecordCount        uint32
	Records            uint32
	IndexesOffset      uint32
	FreeListHead       uint32
	RecordNumbersCount uint32
}

type dbBlob struct {
	StartCryptoBlob uint32
	TotalLength     uint32
	Salt            []byte
	IV              []byte
}

type keyBlobRecordHeader struct {
	RecordSize uint32
}

type keyBlob struct {
	Magic           uint32
	StartCryptoBlob uint32
	TotalLength     uint32
	IV              []byte
}

type genericPasswordHeader struct {
	RecordSize   uint32
	SSGPArea     uint32
	CreationDate uint32
	ModDate      uint32
	Description  uint32
	Comment      uint32
	Creator      uint32
	Type         uint32
	PrintName    uint32
	Alias        uint32
	Account      uint32
	Service      uint32
}

type ssgpBlock struct {
	Magic             []byte
	Label             []byte
	IV                []byte
	EncryptedPassword []byte
}

type genericPassword struct {
	Description    string
	Creator        string
	Type           string
	PrintName      string
	Alias          string
	Account        string
	Service        string
	Created        string
	LastModified   string
	Password       string
	PasswordBase64 bool
}

func New(path, unlockHex string) (*Keychain, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hdr, err := parseHeader(buf)
	if err != nil {
		return nil, err
	}
	if string(hdr.Signature[:]) != keychainSignature {
		return nil, fmt.Errorf("invalid keychain signature: %q", hdr.Signature)
	}

	schema, tableList, err := parseSchema(buf, hdr.SchemaOffset)
	if err != nil {
		return nil, err
	}
	if schema.TableCount == 0 {
		return nil, errors.New("schema does not list any tables")
	}

	kc := &Keychain{
		buf:       buf,
		header:    hdr,
		tableList: tableList,
		tableEnum: make(map[uint32]int),
		keyList:   make(map[string][]byte),
	}
	if err := kc.buildTableIndex(); err != nil {
		return nil, err
	}

	metaOffset, err := kc.getTableOffset(cssmMetadata)
	if err != nil {
		return nil, err
	}

	kc.baseAddr = headerSize + int(metaOffset) + metadataOffsetAdjustment
	if kc.baseAddr+dbBlobSize > len(kc.buf) {
		return nil, errors.New("db blob exceeds file size")
	}
	blob, err := parseDBBlob(kc.buf[kc.baseAddr : kc.baseAddr+dbBlobSize])
	if err != nil {
		return nil, err
	}
	kc.dbblob = blob

	masterKey, err := decodeUnlockKey(unlockHex)
	if err != nil {
		return nil, err
	}

	dbKey, err := kc.findWrappingKey(masterKey)
	if err != nil {
		return nil, err
	}
	kc.dbKey = dbKey

	if err := kc.generateKeyList(); err != nil {
		return nil, err
	}

	return kc, nil
}

func parseHeader(buf []byte) (applDBHeader, error) {
	if len(buf) < headerSize {
		return applDBHeader{}, errors.New("file too small for header")
	}
	hdr := applDBHeader{}
	copy(hdr.Signature[:], buf[:4])
	hdr.Version = binary.BigEndian.Uint32(buf[4:8])
	hdr.HeaderSize = binary.BigEndian.Uint32(buf[8:12])
	hdr.SchemaOffset = binary.BigEndian.Uint32(buf[12:16])
	hdr.AuthOffset = binary.BigEndian.Uint32(buf[16:20])
	return hdr, nil
}

func parseSchema(buf []byte, offset uint32) (applDBSchema, []uint32, error) {
	if int(offset)+schemaSize > len(buf) {
		return applDBSchema{}, nil, errors.New("schema offset exceeds file size")
	}
	schema := applDBSchema{}
	start := int(offset)
	schema.SchemaSize = binary.BigEndian.Uint32(buf[start : start+4])
	schema.TableCount = binary.BigEndian.Uint32(buf[start+4 : start+8])

	baseAddr := headerSize + schemaSize
	tableList := make([]uint32, schema.TableCount)
	for i := 0; i < int(schema.TableCount); i++ {
		pos := baseAddr + i*atomSize
		if pos+atomSize > len(buf) {
			return applDBSchema{}, nil, errors.New("table list exceeds file size")
		}
		tableList[i] = binary.BigEndian.Uint32(buf[pos : pos+atomSize])
	}
	return schema, tableList, nil
}

func parseDBBlob(buf []byte) (dbBlob, error) {
	if len(buf) < dbBlobSize {
		return dbBlob{}, errors.New("db blob buffer too small")
	}
	blob := dbBlob{}
	blob.StartCryptoBlob = binary.BigEndian.Uint32(buf[8:12])
	blob.TotalLength = binary.BigEndian.Uint32(buf[12:16])
	// Salt and IV are located after the random signature (16 bytes), sequence (4 bytes),
	// and DB parameters (8 bytes) inside the blob structure.
	blob.Salt = append([]byte{}, buf[44:64]...)
	blob.IV = append([]byte{}, buf[64:72]...)
	return blob, nil
}

func decodeUnlockKey(hexKey string) ([]byte, error) {
	cleaned := strings.TrimSpace(hexKey)
	cleaned = strings.TrimPrefix(cleaned, "0x")
	key, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("unable to decode unlock key: %w", err)
	}
	if len(key) != keyLength {
		return nil, fmt.Errorf("unlock key must be %d bytes (got %d)", keyLength, len(key))
	}
	return key, nil
}

func (kc *Keychain) buildTableIndex() error {
	for idx, offset := range kc.tableList {
		if offset == 0 {
			continue
		}
		meta, _, err := kc.getTable(offset)
		if err != nil {
			continue
		}
		if _, exists := kc.tableEnum[meta.TableID]; !exists {
			kc.tableEnum[meta.TableID] = idx
		}
	}
	if len(kc.tableEnum) == 0 {
		return errors.New("unable to derive table index")
	}
	return nil
}

func (kc *Keychain) getTableOffset(tableID uint32) (uint32, error) {
	idx, ok := kc.tableEnum[tableID]
	if !ok || idx >= len(kc.tableList) {
		return 0, fmt.Errorf("table id %d not present", tableID)
	}
	return kc.tableList[idx], nil
}

func (kc *Keychain) getTableFromType(tableID uint32) (tableHeader, []uint32, error) {
	offset, err := kc.getTableOffset(tableID)
	if err != nil {
		return tableHeader{}, nil, err
	}
	return kc.getTable(offset)
}

func (kc *Keychain) getTable(offset uint32) (tableHeader, []uint32, error) {
	base := headerSize + int(offset)
	if base < 0 || base+tableHeaderSize > len(kc.buf) {
		return tableHeader{}, nil, errors.New("table header exceeds file size")
	}
	meta := tableHeader{}
	data := kc.buf[base : base+tableHeaderSize]
	meta.TableSize = binary.BigEndian.Uint32(data[0:4])
	meta.TableID = binary.BigEndian.Uint32(data[4:8])
	meta.RecordCount = binary.BigEndian.Uint32(data[8:12])
	meta.Records = binary.BigEndian.Uint32(data[12:16])
	meta.IndexesOffset = binary.BigEndian.Uint32(data[16:20])
	meta.FreeListHead = binary.BigEndian.Uint32(data[20:24])
	meta.RecordNumbersCount = binary.BigEndian.Uint32(data[24:28])

	recordBase := base + tableHeaderSize
	recordList := make([]uint32, 0, meta.RecordCount)
	for idx := 0; idx < int(meta.RecordCount); idx++ {
		pos := recordBase + idx*atomSize
		if pos+atomSize > len(kc.buf) {
			return meta, recordList, errors.New("record offset exceeds file size")
		}
		value := binary.BigEndian.Uint32(kc.buf[pos : pos+atomSize])
		if value != 0 && value%4 == 0 {
			recordList = append(recordList, value)
		}
	}
	return meta, recordList, nil
}

func (kc *Keychain) findWrappingKey(master []byte) ([]byte, error) {
	start := kc.baseAddr + int(kc.dbblob.StartCryptoBlob)
	end := kc.baseAddr + int(kc.dbblob.TotalLength)
	if start < 0 || end > len(kc.buf) || start >= end {
		return nil, errors.New("db blob cipher bounds invalid")
	}
	plain, err := kcdecrypt(master, kc.dbblob.IV, kc.buf[start:end])
	if err != nil {
		return nil, err
	}
	if len(plain) < keyLength {
		return nil, errors.New("db key shorter than expected")
	}
	return append([]byte{}, plain[:keyLength]...), nil
}

func (kc *Keychain) generateKeyList() error {
	_, records, err := kc.getTableFromType(cssmSymmetricKey)
	if err != nil {
		return err
	}
	for _, recordOffset := range records {
		index, ciphertext, iv, err := kc.getKeyblobRecord(recordOffset)
		if err != nil {
			continue
		}
		key, err := keyblobDecryption(ciphertext, iv, kc.dbKey)
		if err != nil || len(key) == 0 {
			continue
		}
		kc.keyList[string(index)] = key
	}
	if len(kc.keyList) == 0 {
		return errors.New("no symmetric keys recovered")
	}
	return nil
}

func (kc *Keychain) getKeyblobRecord(recordOffset uint32) ([]byte, []byte, []byte, error) {
	base, err := kc.getBaseAddress(cssmSymmetricKey, recordOffset)
	if err != nil {
		return nil, nil, nil, err
	}
	if base+keyBlobRecordHeaderSize > len(kc.buf) {
		return nil, nil, nil, errors.New("keyblob header exceeds file size")
	}
	hdr := keyBlobRecordHeader{}
	hdr.RecordSize = binary.BigEndian.Uint32(kc.buf[base : base+4])
	_ = binary.BigEndian.Uint32(kc.buf[base+4 : base+8]) // Skip RecordCount

	recordStart := base + keyBlobRecordHeaderSize
	recordEnd := base + int(hdr.RecordSize)
	if recordEnd > len(kc.buf) {
		return nil, nil, nil, errors.New("keyblob record exceeds file size")
	}
	record := kc.buf[recordStart:recordEnd]
	if len(record) < keyBlobStructSize {
		return nil, nil, nil, errors.New("keyblob structure incomplete")
	}
	blob, err := parseKeyBlob(record[:keyBlobStructSize])
	if err != nil {
		return nil, nil, nil, err
	}
	if blob.Magic != keyBlobMagic {
		return nil, nil, nil, errors.New("unexpected keyblob magic")
	}
	if secureStorageGroup != readASCII(record, int(blob.TotalLength)+8, 4) {
		return nil, nil, nil, errors.New("keyblob not part of secure storage group")
	}

	cipherStart := int(blob.StartCryptoBlob)
	cipherEnd := int(blob.TotalLength)
	if cipherEnd > len(record) || cipherStart >= cipherEnd {
		return nil, nil, nil, errors.New("invalid cipher bounds")
	}
	cipherText := append([]byte{}, record[cipherStart:cipherEnd]...)

	indexStart := int(blob.TotalLength) + 8
	indexEnd := indexStart + 20
	if indexEnd > len(record) {
		return nil, nil, nil, errors.New("key index exceeds record length")
	}
	index := append([]byte{}, record[indexStart:indexEnd]...)
	iv := append([]byte{}, blob.IV...)
	return index, cipherText, iv, nil
}

func parseKeyBlob(buf []byte) (keyBlob, error) {
	if len(buf) < keyBlobStructSize {
		return keyBlob{}, errors.New("key blob buffer too small")
	}
	kb := keyBlob{}
	kb.Magic = binary.BigEndian.Uint32(buf[0:4])
	kb.StartCryptoBlob = binary.BigEndian.Uint32(buf[8:12])
	kb.TotalLength = binary.BigEndian.Uint32(buf[12:16])
	kb.IV = append([]byte{}, buf[16:24]...)
	return kb, nil
}

func (kc *Keychain) getBaseAddress(tableID uint32, offset uint32) (int, error) {
	switch tableID {
	case 23972, 30912:
		tableID = 16
	}
	tableOffset, err := kc.getTableOffset(tableID)
	if err != nil {
		return 0, err
	}
	base := headerSize + int(tableOffset)
	if offset != 0 {
		base += int(offset)
	}
	if base > len(kc.buf) {
		return 0, errors.New("base address exceeds buffer")
	}
	return base, nil
}

func keyblobDecryption(encryptedblob, iv, dbkey []byte) ([]byte, error) {
	plain, err := kcdecrypt(dbkey, magicCMSIV, encryptedblob)
	if err != nil {
		return nil, err
	}
	if len(plain) == 0 {
		return nil, errors.New("empty plain blob")
	}
	if len(plain) < 32 {
		return nil, errors.New("wrapped blob too short")
	}
	rev := make([]byte, 32)
	for i := 0; i < 32; i++ {
		rev[i] = plain[31-i]
	}
	finalPlain, err := kcdecrypt(dbkey, iv, rev)
	if err != nil {
		return nil, err
	}
	if len(finalPlain) < 4 {
		return nil, errors.New("final plain too short")
	}
	key := finalPlain[4:]
	if len(key) != keyLength {
		return nil, errors.New("invalid unwrapped key length")
	}
	return append([]byte{}, key...), nil
}

func kcdecrypt(key, iv, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("ciphertext is empty")
	}
	if len(data)%blockSize != 0 {
		return nil, errors.New("ciphertext not aligned to block size")
	}
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != blockSize {
		return nil, errors.New("invalid IV length")
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, data)

	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > blockSize {
		return nil, errors.New("invalid padding value")
	}
	for _, b := range plain[len(plain)-pad:] {
		if int(b) != pad {
			return nil, errors.New("padding verification failed")
		}
	}
	return plain[:len(plain)-pad], nil
}

func (kc *Keychain) DumpGenericPasswords() ([]genericPassword, error) {
	_, records, err := kc.getTableFromType(cssmGenericPassword)
	if err != nil {
		return nil, err
	}
	results := make([]genericPassword, 0, len(records))
	for _, offset := range records {
		rec, err := kc.parseGenericPasswordRecord(offset)
		if err != nil {
			continue
		}
		results = append(results, rec)
	}
	return results, nil
}

func (kc *Keychain) parseGenericPasswordRecord(recordOffset uint32) (genericPassword, error) {
	base, err := kc.getBaseAddress(cssmGenericPassword, recordOffset)
	if err != nil {
		return genericPassword{}, err
	}
	if base+genericPasswordHeaderSize > len(kc.buf) {
		return genericPassword{}, errors.New("generic password header exceeds file size")
	}
	header, err := parseGenericPasswordHeader(kc.buf[base : base+genericPasswordHeaderSize])
	if err != nil {
		return genericPassword{}, err
	}
	recordEnd := base + int(header.RecordSize)
	if recordEnd > len(kc.buf) {
		return genericPassword{}, errors.New("generic password record exceeds file size")
	}
	buffer := kc.buf[base+genericPasswordHeaderSize : recordEnd]

	ssgp, dbkey := kc.extractSSGP(header, buffer)
	password, base64Encoded := decryptSSGP(ssgp, dbkey)

	rec := genericPassword{
		Description:    kc.readLV(base, header.Description),
		Creator:        kc.readFourChar(base, header.Creator),
		Type:           kc.readFourChar(base, header.Type),
		PrintName:      kc.readLV(base, header.PrintName),
		Alias:          kc.readLV(base, header.Alias),
		Account:        kc.readLV(base, header.Account),
		Service:        kc.readLV(base, header.Service),
		Created:        kc.readKeychainTime(base, header.CreationDate),
		LastModified:   kc.readKeychainTime(base, header.ModDate),
		Password:       password,
		PasswordBase64: base64Encoded,
	}
	return rec, nil
}

func parseGenericPasswordHeader(buf []byte) (genericPasswordHeader, error) {
	if len(buf) < genericPasswordHeaderSize {
		return genericPasswordHeader{}, errors.New("generic password header too small")
	}
	vals := make([]uint32, 22)
	for i := 0; i < 22; i++ {
		start := i * 4
		vals[i] = binary.BigEndian.Uint32(buf[start : start+4])
	}
	hdr := genericPasswordHeader{
		RecordSize:   vals[0],
		SSGPArea:     vals[4],
		CreationDate: vals[6],
		ModDate:      vals[7],
		Description:  vals[8],
		Comment:      vals[9],
		Creator:      vals[10],
		Type:         vals[11],
		PrintName:    vals[13],
		Alias:        vals[14],
		Account:      vals[19],
		Service:      vals[20],
	}
	return hdr, nil
}

func (kc *Keychain) extractSSGP(header genericPasswordHeader, buffer []byte) (*ssgpBlock, []byte) {
	if header.SSGPArea == 0 || int(header.SSGPArea) > len(buffer) {
		return nil, nil
	}
	block, err := parseSSGP(buffer[:header.SSGPArea])
	if err != nil {
		return nil, nil
	}
	keyIndex := make([]byte, 0, len(block.Magic)+len(block.Label))
	keyIndex = append(keyIndex, block.Magic...)
	keyIndex = append(keyIndex, block.Label...)
	dbkey, ok := kc.keyList[string(keyIndex)]
	if !ok {
		return block, nil
	}
	return block, dbkey
}

func parseSSGP(buf []byte) (*ssgpBlock, error) {
	if len(buf) < 28 {
		return nil, errors.New("ssgp buffer too small")
	}
	block := &ssgpBlock{
		Magic:             append([]byte{}, buf[0:4]...),
		Label:             append([]byte{}, buf[4:20]...),
		IV:                append([]byte{}, buf[20:28]...),
		EncryptedPassword: append([]byte{}, buf[28:]...),
	}
	return block, nil
}

func decryptSSGP(block *ssgpBlock, dbkey []byte) (string, bool) {
	if block == nil || len(dbkey) == 0 {
		return keychainLockedSignature, false
	}
	plain, err := kcdecrypt(dbkey, block.IV, block.EncryptedPassword)
	if err != nil || len(plain) == 0 {
		return keychainLockedSignature, false
	}
	if utf8.Valid(plain) {
		return string(plain), false
	}
	return base64.StdEncoding.EncodeToString(plain), true
}

func (kc *Keychain) readKeychainTime(base int, ptr uint32) string {
	if ptr == 0 {
		return ""
	}
	offset := base + maskedPointer(ptr)
	if offset < 0 || offset+16 > len(kc.buf) {
		return ""
	}
	raw := bytes.TrimRight(kc.buf[offset:offset+16], "\x00")
	if len(raw) == 0 {
		return ""
	}
	parsed, err := time.Parse("20060102150405Z", string(raw))
	if err != nil {
		return string(raw)
	}
	return parsed.Format(time.RFC3339)
}

func (kc *Keychain) readFourChar(base int, ptr uint32) string {
	if ptr == 0 {
		return ""
	}
	offset := base + maskedPointer(ptr)
	if offset < 0 || offset+4 > len(kc.buf) {
		return ""
	}
	return strings.TrimRight(string(kc.buf[offset:offset+4]), "\x00")
}

func (kc *Keychain) readLV(base int, ptr uint32) string {
	if ptr == 0 {
		return ""
	}
	offset := base + maskedPointer(ptr)
	if offset < 0 || offset+4 > len(kc.buf) {
		return ""
	}
	length := int(binary.BigEndian.Uint32(kc.buf[offset : offset+4]))
	padded := alignToWord(length)
	start := offset + 4
	end := start + padded
	if end > len(kc.buf) {
		return ""
	}
	data := kc.buf[start : start+length]
	data = bytes.TrimRight(data, "\x00")
	return string(data)
}

func maskedPointer(value uint32) int {
	return int(value & 0xFFFFFFFE)
}

func alignToWord(value int) int {
	if value%4 == 0 {
		return value
	}
	return ((value / 4) + 1) * 4
}

func readASCII(buf []byte, start, length int) string {
	if start < 0 || start+length > len(buf) {
		return ""
	}
	return string(buf[start : start+length])
}
