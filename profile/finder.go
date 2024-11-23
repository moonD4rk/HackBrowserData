package profile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types2"
)

type Finder struct {
	// // FileSystem is the file system interface.
	// // Default is os package, can be replaced with a mock for testing
	FileSystem FileSystem
}

func NewFinder() *Finder {
	manager := &Finder{
		FileSystem: osFS{},
	}
	return manager
}

func (m *Finder) FindProfiles(rootPath string, browserType types2.BrowserType, dataTypes []types2.DataType) (Profiles, error) {
	var err error
	var profiles Profiles
	switch browserType {
	case types2.FirefoxType:
		profiles, err = m.findFirefoxProfiles(rootPath, browserType, dataTypes)
	case types2.ChromiumType, types2.YandexType:
		profiles, err = m.findChromiumProfiles(rootPath, browserType, dataTypes)
	default:
		return nil, fmt.Errorf("unsupported browser type: %s", browserType.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find profiles: %w", err)
	}
	return profiles, nil
}

var defaultExcludeDirs = []string{"Snapshot", "System Profile", "Crash Reports", "def"}

func (m *Finder) findChromiumProfiles(rootPath string, browserType types2.BrowserType, dataTypes []types2.DataType) (Profiles, error) {
	profiles := NewProfiles()
	var masterKeyPath string

	err := m.FileSystem.WalkDir(rootPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Debugf("skipping walk chromium path permission error: %s", err)
				return nil
			}
			return err
		}
		// Skip directories that should not be included
		if entry.IsDir() && skipDirs(path, defaultExcludeDirs) {
			return fs.SkipDir
		}
		for _, dataType := range dataTypes {
			dataTypeFilename := dataType.Filename(browserType)
			if dataType == types2.MasterKey && entry.Name() == dataTypeFilename {
				masterKeyPath = path
				break
			}
			if !isEntryMatchesDataType(browserType, entry, dataType, path) {
				continue
			}
			// Calculate relative path from baseDir path
			profileName := extractProfileName(rootPath, path, entry.IsDir())
			if profileName == "" {
				continue
			}
			profiles.SetDataTypePath(profileName, dataType, path)
		}
		return nil
	})
	profiles.SetMasterKey(masterKeyPath)
	return profiles, err
}

func (m *Finder) findFirefoxProfiles(rootPath string, browserType types2.BrowserType, dataTypes []types2.DataType) (Profiles, error) {
	profiles := NewProfiles()
	err := m.FileSystem.WalkDir(rootPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				log.Debugf("skipping walk firefox path permission error: %s", err)
				return nil
			}
			return err
		}
		for _, dataType := range dataTypes {
			if !isEntryMatchesDataType(browserType, entry, dataType, path) {
				continue
			}
			// Calculate relative path from Firefox baseDir path
			profileName := extractProfileName(rootPath, path, entry.IsDir())
			if profileName == "" {
				continue
			}
			profiles.SetDataTypePath(profileName, dataType, path)
		}
		return nil
	})
	profiles.AssignMasterKey()
	return profiles, err
}

// skipDirs returns true if the path should be excluded from processing.
// `dirs` is a slice of directory names to skip.
func skipDirs(path string, dirs []string) bool {
	base := filepath.Base(path)
	for _, dir := range dirs {
		if strings.Contains(base, dir) {
			return true
		}
	}
	return false
}

// extractProfileName extracts the profile name from the path relative to the baseDir path.
// The profile name is the first directory in the relative path.
// If the path is not relative to the baseDir, an empty string is returned.
func extractProfileName(basePath, currentPath string, isDir bool) string {
	relativePath, err := filepath.Rel(basePath, currentPath)
	// If the path is not relative to the baseDir, return empty string
	if err != nil || strings.HasPrefix(relativePath, "..") || relativePath == "." {
		return ""
	}
	pathParts := strings.Split(relativePath, string(filepath.Separator))
	if len(pathParts) == 0 {
		return ""
	}
	if isDir {
		return pathParts[0]
	}
	if len(pathParts) > 1 {
		return pathParts[0]
	}
	return ""
}

func isEntryMatchesDataType(browserType types2.BrowserType, entry fs.DirEntry, dataType types2.DataType, path string) bool {
	// if dataType and entry type (file or directory) do not match, return false
	if entry.IsDir() != dataType.IsDir(browserType) {
		return false
	}

	dataTypeFilename := dataType.Filename(browserType)
	// if entry is a directory, check if path ends with dataTypeFilename
	// e.g. for Chrome, "Local Storage / leveldb" is a directory, so we check if path ends with "leveldb"
	if entry.IsDir() {
		return strings.HasSuffix(path, dataTypeFilename)
	}
	// if entry is a file, check if entry name matches dataTypeFilename
	return entry.Name() == dataTypeFilename
}
