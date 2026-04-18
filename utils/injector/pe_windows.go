//go:build windows

package injector

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
)

func FindExportFileOffset(dllBytes []byte, exportName string) (uint32, error) {
	rva, err := findExportRVA(dllBytes, exportName)
	if err != nil {
		return 0, err
	}
	f, err := pe.NewFile(bytes.NewReader(dllBytes))
	if err != nil {
		return 0, fmt.Errorf("parse PE: %w", err)
	}
	off, ok := rvaToFileOffset(f, rva)
	if !ok {
		return 0, fmt.Errorf("RVA 0x%x (%s) has no raw file mapping", rva, exportName)
	}
	return off, nil
}

func findExportRVA(dllBytes []byte, exportName string) (uint32, error) {
	view, err := loadExportSection(dllBytes)
	if err != nil {
		return 0, err
	}
	edOff, err := view.rvaToOff(view.dirRVA)
	if err != nil {
		return 0, err
	}
	var ed imageExportDirectory
	if err := binary.Read(bytes.NewReader(view.raw[edOff:]), binary.LittleEndian, &ed); err != nil {
		return 0, fmt.Errorf("read export directory: %w", err)
	}
	if ed.NumberOfNames == 0 {
		return 0, fmt.Errorf("PE has no named exports")
	}
	return findNamedExport(view, &ed, exportName)
}

func rvaToFileOffset(f *pe.File, rva uint32) (uint32, bool) {
	for _, s := range f.Sections {
		if rva >= s.VirtualAddress && rva < s.VirtualAddress+s.VirtualSize {
			return rva - s.VirtualAddress + s.Offset, true
		}
	}
	return 0, false
}

type exportSectionView struct {
	raw      []byte
	sectBase uint32
	sectSize uint32
	sectName string
	dirRVA   uint32
	dirSize  uint32
}

func (v *exportSectionView) rvaToOff(rva uint32) (uint32, error) {
	if rva < v.sectBase || rva >= v.sectBase+v.sectSize {
		return 0, fmt.Errorf("RVA 0x%x outside section %q", rva, v.sectName)
	}
	off := rva - v.sectBase
	if int(off) >= len(v.raw) {
		return 0, fmt.Errorf("RVA 0x%x beyond raw section data", rva)
	}
	return off, nil
}

func loadExportSection(dllBytes []byte) (*exportSectionView, error) {
	f, err := pe.NewFile(bytes.NewReader(dllBytes))
	if err != nil {
		return nil, fmt.Errorf("parse PE: %w", err)
	}
	oh, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		return nil, fmt.Errorf("expected PE32+ (64-bit) image")
	}
	if len(oh.DataDirectory) == 0 {
		return nil, fmt.Errorf("PE has no data directories")
	}
	exp := oh.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_EXPORT]
	if exp.Size == 0 || exp.VirtualAddress == 0 {
		return nil, fmt.Errorf("PE has no export directory")
	}
	sect := findSectionForRVA(f, exp.VirtualAddress)
	if sect == nil {
		return nil, fmt.Errorf("export directory RVA 0x%x not in any section", exp.VirtualAddress)
	}
	raw, err := sect.Data()
	if err != nil {
		return nil, fmt.Errorf("read section %q: %w", sect.Name, err)
	}
	return &exportSectionView{
		raw:      raw,
		sectBase: sect.VirtualAddress,
		sectSize: sect.VirtualSize,
		sectName: sect.Name,
		dirRVA:   exp.VirtualAddress,
		dirSize:  exp.Size,
	}, nil
}

func findNamedExport(view *exportSectionView, ed *imageExportDirectory, name string) (uint32, error) {
	namesOff, err := view.rvaToOff(ed.AddressOfNames)
	if err != nil {
		return 0, err
	}
	funcsOff, err := view.rvaToOff(ed.AddressOfFunctions)
	if err != nil {
		return 0, err
	}
	ordOff, err := view.rvaToOff(ed.AddressOfNameOrdinals)
	if err != nil {
		return 0, err
	}

	for i := uint32(0); i < ed.NumberOfNames; i++ {
		nameRVA := binary.LittleEndian.Uint32(view.raw[namesOff+i*4 : namesOff+i*4+4])
		nameOff, err := view.rvaToOff(nameRVA)
		if err != nil {
			continue
		}
		if readCString(view.raw[nameOff:]) != name {
			continue
		}
		ord := binary.LittleEndian.Uint16(view.raw[ordOff+i*2 : ordOff+i*2+2])
		fnSlot := funcsOff + uint32(ord)*4
		if int(fnSlot)+4 > len(view.raw) {
			return 0, fmt.Errorf("function slot for %q out of range", name)
		}
		return binary.LittleEndian.Uint32(view.raw[fnSlot : fnSlot+4]), nil
	}
	return 0, fmt.Errorf("export %q not found", name)
}

type imageExportDirectory struct {
	Characteristics       uint32
	TimeDateStamp         uint32
	MajorVersion          uint16
	MinorVersion          uint16
	Name                  uint32
	Base                  uint32
	NumberOfFunctions     uint32
	NumberOfNames         uint32
	AddressOfFunctions    uint32
	AddressOfNames        uint32
	AddressOfNameOrdinals uint32
}

func findSectionForRVA(f *pe.File, rva uint32) *pe.Section {
	for _, s := range f.Sections {
		if rva >= s.VirtualAddress && rva < s.VirtualAddress+s.VirtualSize {
			return s
		}
	}
	return nil
}

func readCString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
