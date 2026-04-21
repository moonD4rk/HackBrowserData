package safari

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testExtensionEntry mirrors the shape of one entry in Safari's Extensions.plist:
// an untyped dictionary keyed by string. Using a map (instead of safariExtension)
// lets tests omit keys like Enabled for AppExtension-style fixtures, matching
// what Safari actually writes.
type testExtensionEntry map[string]any

// writeTestExtensionsPlist writes an Extensions.plist under
// <container>/Safari/<subdir>/Extensions.plist. subdir is either
// "AppExtensions" or "WebExtensions".
func writeTestExtensionsPlist(t *testing.T, container, subdir string, entries map[string]testExtensionEntry) {
	t.Helper()
	dir := filepath.Join(container, safariExtensionsSubdir, subdir)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	f, err := os.Create(filepath.Join(dir, safariExtensionsPlistFile))
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, plist.NewBinaryEncoder(f).Encode(entries))
}

func TestExtractExtensions_AppAndWebMerged(t *testing.T) {
	container := t.TempDir()
	writeTestExtensionsPlist(t, container, safariAppExtensionsSubdir, map[string]testExtensionEntry{
		"com.colliderli.iina.OpenInIINA (67CQ77V27R)": {},
	})
	writeTestExtensionsPlist(t, container, safariWebExtensionsSubdir, map[string]testExtensionEntry{
		"com.1password.safari.extension (2BUA8C4S2C)": {"Enabled": true},
	})

	got, err := extractExtensions(container)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Results are sorted by key, so 1Password (com.1…) comes before iina (com.c…).
	assert.Equal(t, "com.1password.safari.extension", got[0].Name)
	assert.Equal(t, "com.1password.safari.extension (2BUA8C4S2C)", got[0].ID)
	assert.True(t, got[0].Enabled)

	assert.Equal(t, "com.colliderli.iina.OpenInIINA", got[1].Name)
	assert.Equal(t, "com.colliderli.iina.OpenInIINA (67CQ77V27R)", got[1].ID)
	// AppExtensions omit the Enabled field — defaults to true (present == enabled).
	assert.True(t, got[1].Enabled)
}

func TestExtractExtensions_EnabledFlag(t *testing.T) {
	container := t.TempDir()
	writeTestExtensionsPlist(t, container, safariWebExtensionsSubdir, map[string]testExtensionEntry{
		"com.example.a (AAAAAAAAAA)": {"Enabled": true},
		"com.example.b (BBBBBBBBBB)": {"Enabled": false},
	})

	got, err := extractExtensions(container)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.True(t, got[0].Enabled)
	assert.False(t, got[1].Enabled)
}

func TestExtractExtensions_BundleIDFallbackOnUnexpectedKey(t *testing.T) {
	container := t.TempDir()
	writeTestExtensionsPlist(t, container, safariAppExtensionsSubdir, map[string]testExtensionEntry{
		"legacy-key-without-team-id": {},
	})

	got, err := extractExtensions(container)
	require.NoError(t, err)
	require.Len(t, got, 1)
	// Regex miss → fall back to the full trimmed key.
	assert.Equal(t, "legacy-key-without-team-id", got[0].Name)
	assert.Equal(t, "legacy-key-without-team-id", got[0].ID)
}

func TestExtractExtensions_OnlyAppExt(t *testing.T) {
	container := t.TempDir()
	writeTestExtensionsPlist(t, container, safariAppExtensionsSubdir, map[string]testExtensionEntry{
		"com.example.only (XXXXXXXXX1)": {},
	})

	got, err := extractExtensions(container)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "com.example.only", got[0].Name)
}

func TestExtractExtensions_OnlyWebExt(t *testing.T) {
	container := t.TempDir()
	writeTestExtensionsPlist(t, container, safariWebExtensionsSubdir, map[string]testExtensionEntry{
		"com.example.web (XXXXXXXXX2)": {"Enabled": true},
	})

	got, err := extractExtensions(container)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "com.example.web", got[0].Name)
}

func TestExtractExtensions_NoPlists(t *testing.T) {
	container := t.TempDir()
	got, err := extractExtensions(container)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCountExtensions(t *testing.T) {
	container := t.TempDir()
	writeTestExtensionsPlist(t, container, safariAppExtensionsSubdir, map[string]testExtensionEntry{
		"com.example.a (AAAAAAAAAA)": {},
		"com.example.b (BBBBBBBBBB)": {},
	})
	writeTestExtensionsPlist(t, container, safariWebExtensionsSubdir, map[string]testExtensionEntry{
		"com.example.c (CCCCCCCCCC)": {"Enabled": true},
	})

	count, err := countExtensions(container)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}
