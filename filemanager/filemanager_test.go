package filemanager

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/profile"
	"github.com/moond4rk/hackbrowserdata/types2"
)

func TestNewFileManager(t *testing.T) {
	fm, err := NewFileManager()
	assert.NoError(t, err)
	defer fm.Cleanup()
	fmt.Println(fm.TempDir)
}

func TestFileManager_CopyProfile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping test on non-darwin system")
	}
	paths, err := filepath.Glob(`/Users/*/Library/Application Support/Firefox/Profiles`)
	assert.NoError(t, err)
	if len(paths) == 0 {
		t.Skip("no chrome profile found")
	}
	rootPath := paths[0]
	browserType := types2.FirefoxType
	dataTypes := types2.AllDataTypes
	finder := profile.NewFinder()
	profiles, err := finder.FindProfiles(rootPath, browserType, dataTypes)
	assert.NoError(t, err)
	assert.NotNil(t, profiles)
	fmt.Println(profiles)
	fm, err := NewFileManager()
	assert.NoError(t, err)
	fmt.Println(fm.TempDir)
	defer fm.Cleanup()
	newProfiles, err := fm.CopyProfiles(profiles)
	assert.NoError(t, err)
	_ = newProfiles
}
