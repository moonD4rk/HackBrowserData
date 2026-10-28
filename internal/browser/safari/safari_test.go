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
	libraryDir  = browser.UserDataDir{Def: testDef(), Profiles: []browser.Profile{defaultProfile, workProfile}}
)

// sourcePaths is where each category's candidates land under ~/Library once joined with the profile's Dir.
func sourcePaths(p browser.Profile, ex browser.Extractors) map[category.Category][]string {
	got := make(map[category.Category][]string, len(ex))
	for c, e := range ex {
		for _, src := range e.Sources {
			for _, rel := range src.Paths {
				got[c] = append(got[c], path.Join(p.Dir, rel))
			}
		}
	}
	return got
}

func TestExtractors_DefaultProfileSources(t *testing.T) {
	ex := Extractors(libraryDir, defaultProfile, nil)
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
	}, sourcePaths(defaultProfile, ex))
	require.Contains(t, ex, category.Password)
	assert.Empty(t, ex[category.Password].Sources, "passwords come from the keychain, not a file")
}

func TestExtractors_NamedProfileSources(t *testing.T) {
	store := containerDir + "/WebKit/WebsiteDataStore/" + strings.ToLower(workUUID)
	assert.Equal(t, map[category.Category][]string{
		category.History:      {namedProfilesDir + "/" + workUUID + "/History.db"},
		category.Cookie:       {store + "/Cookies/Cookies.binarycookies"},
		category.Download:     {"Safari/Downloads.plist"},
		category.LocalStorage: {store + "/Origins"},
	}, sourcePaths(workProfile, Extractors(libraryDir, workProfile, nil)))
}

// snapshotOf lays fixture files out as the Safari user data dir's snapshot, keyed by their path under ~/Library.
func snapshotOf(t *testing.T, files map[string]string) snapshot.Snapshot {
	t.Helper()
	root := t.TempDir()
	for rel, src := range files {
		data, err := os.ReadFile(src)
		require.NoError(t, err)
		dst := filepath.Join(root, "safari", filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
		require.NoError(t, os.WriteFile(dst, data, 0o644))
	}
	return snapshot.Open(root).Snapshot("safari")
}

func writeTestPlist(t *testing.T, v any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fixture.plist")
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, plist.NewBinaryEncoder(f).Encode(v))
	return p
}

func extract(t *testing.T, snap snapshot.Snapshot, p browser.Profile, kc LoginKeychain, c category.Category) (*testCollector, error) {
	t.Helper()
	ex, ok := Extractors(libraryDir, p, kc)[c]
	require.Truef(t, ok, "no %s extractor for %s", c, p.Name)
	var out testCollector
	err := ex.Extract(context.Background(), browser.NewProfileSnapshot(snap, p, nil), &out)
	return &out, err
}

// Each profile keeps its own entries of the shared Downloads.plist; an entry without an owner is the default's.
func TestExtractors_DownloadsBelongToTheirProfile(t *testing.T) {
	snap := snapshotOf(t, map[string]string{"Safari/Downloads.plist": writeTestPlist(t, map[string]any{
		"DownloadHistory": []any{
			map[string]any{"DownloadEntryURL": "https://a.test/home", "DownloadEntryProfileUUIDStringKey": "DefaultProfile"},
			map[string]any{"DownloadEntryURL": "https://a.test/work", "DownloadEntryProfileUUIDStringKey": workUUID},
			map[string]any{"DownloadEntryURL": "https://a.test/legacy"},
		},
	})})
	urls := func(p browser.Profile) []string {
		out, err := extract(t, snap, p, nil, category.Download)
		require.NoError(t, err)
		var got []string
		for _, d := range collected[record.Download](t, out) {
			got = append(got, d.URL)
		}
		return got
	}
	assert.Equal(t, []string{"https://a.test/home", "https://a.test/legacy"}, urls(defaultProfile))
	assert.Equal(t, []string{"https://a.test/work"}, urls(workProfile))
}

// The extension plists are read from the snapshot; a missing one is skipped.
func TestExtractors_ExtensionsFromTheSnapshot(t *testing.T) {
	snap := snapshotOf(t, map[string]string{
		containerDir + "/Safari/WebExtensions/Extensions.plist": writeTestPlist(t, map[string]any{
			"com.example.Blocker.Extension (ABCDE12345)": map[string]any{"Enabled": false},
		}),
	})
	out, err := extract(t, snap, defaultProfile, nil, category.Extension)
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
	kc := &fakeKeychain{passwords: []keychainbreaker.InternetPassword{
		{Protocol: "htps", Server: "github.com", Port: 443, Account: "alice", PlainPassword: "s3cret", Created: created},
		{Protocol: "htps", Server: "locked.test", Account: "bob"},
		{Protocol: "htps", Server: "", Account: "nobody"},
		{Protocol: "htps", Server: "anonymous.test", Account: ""},
	}}
	out, err := extract(t, snapshotOf(t, nil), defaultProfile, kc, category.Password)
	require.NoError(t, err)
	assert.Equal(t, []record.Login{
		{URL: "https://github.com", Username: "alice", Password: "s3cret", CreatedAt: created.UTC(), Store: record.StoreLocal},
		{URL: "https://locked.test", Username: "bob", Store: record.StoreLocal},
	}, collected[record.Login](t, out))
	assert.Equal(t, 1, kc.calls)
}

func TestExtractors_PasswordsWithoutAKeychain(t *testing.T) {
	_, err := extract(t, snapshotOf(t, nil), defaultProfile, nil, category.Password)
	require.ErrorIs(t, err, errNoKeychain)

	broken := errors.New("keychain unreadable")
	_, err = extract(t, snapshotOf(t, nil), defaultProfile, &fakeKeychain{err: broken}, category.Password)
	require.ErrorIs(t, err, broken)
}

// The keychain, Bookmarks.plist, and the extension plists carry no profile tag, so only the default profile reads them.
func TestExtractors_NamedProfileSkipsUserWideSources(t *testing.T) {
	ex := Extractors(libraryDir, workProfile, &fakeKeychain{})
	for _, c := range []category.Category{category.Password, category.Bookmark, category.Extension} {
		assert.NotContains(t, ex, c)
	}
}
