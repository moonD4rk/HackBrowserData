package browser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

// TestWriteArchive_RoundTrip exercises the archive path: ArchiveSources -> WriteArchive (stage+zip)
// -> Unzip, asserting the archive's internal layout is <key>/<User Data layout>.
func TestWriteArchive_RoundTrip(t *testing.T) {
	origin := t.TempDir()
	def := filepath.Join(origin, "Default")
	if err := os.MkdirAll(def, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(def, "Preferences"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(def, "History"), []byte("hist"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(origin, "Local State"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := chromium.NewBrowser(types.BrowserConfig{Key: "chrome", Name: "chrome", Kind: types.Chromium, UserDataDir: origin})
	if err != nil || b == nil {
		t.Fatalf("NewBrowser: b=%v err=%v", b, err)
	}

	zipPath := filepath.Join(t.TempDir(), "data.zip")
	n, err := WriteArchive([]Browser{b}, []types.Category{types.History}, zipPath)
	if err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteArchive captured 0 entries")
	}

	extracted := t.TempDir()
	if err := fileutil.Unzip(zipPath, extracted); err != nil {
		t.Fatalf("Unzip: %v", err)
	}
	for _, rel := range []string{
		filepath.Join("chrome", "Default", "History"),
		filepath.Join("chrome", "Default", "Preferences"),
		filepath.Join("chrome", "Local State"),
	} {
		if _, err := os.Stat(filepath.Join(extracted, rel)); err != nil {
			t.Errorf("expected %s in archive layout: %v", rel, err)
		}
	}
}
