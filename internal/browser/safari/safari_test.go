package safari

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moond4rk/keychainbreaker"
	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

const workUUID = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"

var (
	workProfile = named("work", workUUID)
	libraryDir  = browser.UserDataDir{Browser: testBrowser(), Profiles: []browser.Profile{defaultProfile, workProfile}}
)

func sourcePaths(profile browser.Profile, extractors browser.Extractors) map[category.Category][]string {
	got := make(map[category.Category][]string, len(extractors))
	for dataCategory, extractor := range extractors {
		for _, source := range extractor.Sources {
			for _, relativePath := range source.Paths() {
				got[dataCategory] = append(got[dataCategory], path.Join(profile.RelativePath, relativePath))
			}
		}
	}
	return got
}

func TestExtractors_DefaultProfileSources(t *testing.T) {
	extractors := Extractors(libraryDir, defaultProfile, nil)
	assert.Equal(t, map[category.Category][]string{
		category.History:  {"Safari/History.db"},
		category.Cookie:   {containerDir + "/Cookies/Cookies.binarycookies", "Cookies/Cookies.binarycookies"},
		category.Bookmark: {"Safari/Bookmarks.plist"},
		category.Download: {"Safari/Downloads.plist"},
		category.LocalStorage: {
			containerDir + "/WebKit/WebsiteData/Default",
		},
		category.Extension: {
			containerDir + "/Safari/AppExtensions/Extensions.plist",
			containerDir + "/Safari/WebExtensions/Extensions.plist",
		},
	}, sourcePaths(defaultProfile, extractors))
	require.Contains(t, extractors, category.Password)
	assert.Empty(t, extractors[category.Password].Sources, "passwords come from the keychain, not a file")
}

func TestExtractors_NamedProfileSources(t *testing.T) {
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(workUUID)
	assert.Equal(t, map[category.Category][]string{
		category.History:      {namedProfilesDir + "/" + workUUID + "/History.db"},
		category.Cookie:       {store + "/Cookies/Cookies.binarycookies"},
		category.Download:     {"Safari/Downloads.plist"},
		category.LocalStorage: {store + "/Origins"},
	}, sourcePaths(workProfile, Extractors(libraryDir, workProfile, nil)))

	copied := browser.UserDataDir{Profiles: []browser.Profile{{Name: "default", RelativePath: "SafariCopy"}, workProfile}}
	assert.Equal(t, []string{"SafariCopy/Downloads.plist"}, sourcePaths(workProfile, Extractors(copied, workProfile, nil))[category.Download],
		"a named profile's downloads sit in the default profile's dir, whatever -p named it")
}

func snapshotOf(t *testing.T, files map[string]string) snapshot.Snapshot {
	t.Helper()
	root := t.TempDir()
	for relativePath, fixturePath := range files {
		data, err := os.ReadFile(fixturePath)
		require.NoError(t, err)
		target := filepath.Join(root, "safari", filepath.FromSlash(relativePath))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, data, 0o644))
	}
	return snapshot.Open(root).Snapshot("safari")
}

func writeTestPlist(t *testing.T, value any) string {
	t.Helper()
	plistPath := filepath.Join(t.TempDir(), "fixture.plist")
	file, err := os.Create(plistPath)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, plist.NewBinaryEncoder(file).Encode(value))
	return plistPath
}

func extract(t *testing.T, dirSnapshot snapshot.Snapshot, profile browser.Profile, loginKeychain LoginKeychain, dataCategory category.Category) (*testCollector, error) {
	t.Helper()
	extractor, ok := Extractors(libraryDir, profile, loginKeychain)[dataCategory]
	require.Truef(t, ok, "no %s extractor for %s", dataCategory, profile.Name)
	var out testCollector
	err := extractor.Extract(context.Background(), browser.NewProfileSnapshot(dirSnapshot, profile, nil), &out)
	return &out, err
}

// TestExtractors_DownloadsBelongToTheirProfile splits the shared Downloads.plist; an untagged entry is the default's.
func TestExtractors_DownloadsBelongToTheirProfile(t *testing.T) {
	dirSnapshot := snapshotOf(t, map[string]string{"Safari/Downloads.plist": writeTestPlist(t, map[string]any{
		"DownloadHistory": []any{
			map[string]any{"DownloadEntryURL": "https://a.test/home", "DownloadEntryProfileUUIDStringKey": "DefaultProfile"},
			map[string]any{"DownloadEntryURL": "https://a.test/work", "DownloadEntryProfileUUIDStringKey": workUUID},
			map[string]any{"DownloadEntryURL": "https://a.test/legacy"},
		},
	})})
	urls := func(profile browser.Profile) []string {
		out, err := extract(t, dirSnapshot, profile, nil, category.Download)
		require.NoError(t, err)
		var got []string
		for _, download := range collected[record.Download](t, out) {
			got = append(got, download.URL)
		}
		return got
	}
	assert.Equal(t, []string{"https://a.test/home", "https://a.test/legacy"}, urls(defaultProfile))
	assert.Equal(t, []string{"https://a.test/work"}, urls(workProfile))
}

// TestExtractors_ExtensionsFromTheSnapshot reads the extension plists from the snapshot and skips a missing one.
func TestExtractors_ExtensionsFromTheSnapshot(t *testing.T) {
	dirSnapshot := snapshotOf(t, map[string]string{
		containerDir + "/Safari/WebExtensions/Extensions.plist": writeTestPlist(t, map[string]any{
			"com.example.Blocker.Extension (ABCDE12345)": map[string]any{"Enabled": false},
		}),
	})
	out, err := extract(t, dirSnapshot, defaultProfile, nil, category.Extension)
	require.NoError(t, err)
	got := collected[record.Extension](t, out)
	require.Len(t, got, 1)
	assert.Equal(t, "com.example.Blocker.Extension", got[0].Name)
	assert.False(t, got[0].Enabled)
}

type fakeKeychain struct {
	passwords []keychainbreaker.InternetPassword
	err       error
	calls     int
}

func (k *fakeKeychain) InternetPasswords(context.Context) ([]keychainbreaker.InternetPassword, error) {
	k.calls++
	return k.passwords, k.err
}

func TestExtractors_PasswordsFromTheKeychain(t *testing.T) {
	created := time.Date(2024, 3, 1, 9, 0, 0, 0, time.FixedZone("CET", 3600))
	loginKeychain := &fakeKeychain{passwords: []keychainbreaker.InternetPassword{
		{Protocol: "htps", Server: "github.com", Port: 443, Account: "alice", PlainPassword: "s3cret", Created: created},
		{Protocol: "htps", Server: "locked.test", Account: "bob"},
		{Protocol: "htps", Server: "", Account: "nobody"},
		{Protocol: "htps", Server: "anonymous.test", Account: ""},
	}}
	out, err := extract(t, snapshotOf(t, nil), defaultProfile, loginKeychain, category.Password)
	require.NoError(t, err)
	assert.Equal(t, []record.Login{
		{URL: "https://github.com", Username: "alice", Password: "s3cret", CreatedAt: created.UTC(), Store: record.StoreLocal},
		{URL: "https://locked.test", Username: "bob", Store: record.StoreLocal},
	}, collected[record.Login](t, out))
	assert.Equal(t, 1, loginKeychain.calls)
}

func TestExtractors_PasswordsWithoutAKeychain(t *testing.T) {
	_, err := extract(t, snapshotOf(t, nil), defaultProfile, nil, category.Password)
	require.ErrorIs(t, err, errNoKeychain)

	broken := errors.New("keychain unreadable")
	_, err = extract(t, snapshotOf(t, nil), defaultProfile, &fakeKeychain{err: broken}, category.Password)
	require.ErrorIs(t, err, broken)
}

// TestExtractors_NamedProfileSkipsUserWideSources leaves the keychain and the untagged plists to the default profile.
func TestExtractors_NamedProfileSkipsUserWideSources(t *testing.T) {
	extractors := Extractors(libraryDir, workProfile, &fakeKeychain{})
	for _, dataCategory := range []category.Category{category.Password, category.Bookmark, category.Extension} {
		assert.NotContains(t, extractors, dataCategory)
	}
}
