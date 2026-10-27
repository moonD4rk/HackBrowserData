package browser

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/moond4rk/hackbrowserdata/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
	"github.com/moond4rk/hackbrowserdata/types"
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
	n, err := WriteArchive([]Browser{b}, []category.Category{category.History}, zipPath)
	if err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteArchive captured 0 entries")
	}

	d, err := snapshot.Unzip(zipPath)
	if err != nil {
		t.Fatalf("Unzip: %v", err)
	}
	defer d.Close()
	extracted := d.Path()
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

// WriteArchive copies each source from UserDataDir/LayoutRel, so that path must be exactly the source's AbsPath.
func TestArchiveSources_AbsPathIsUserDataDirPlusLayoutRel(t *testing.T) {
	nested := t.TempDir()
	for _, rel := range []string{
		"Local State", "Default/Preferences", "Default/History", "Default/Login Data",
		"Default/Login Data For Account", "Profile 1/Preferences", "Profile 1/Network/Cookies",
	} {
		writeArchiveFile(t, nested, rel)
	}
	flat := t.TempDir()
	writeArchiveFile(t, flat, "History")

	for _, udd := range []string{nested, flat} {
		b, err := chromium.NewBrowser(types.BrowserConfig{Key: "chrome", Name: "chrome", Kind: types.Chromium, UserDataDir: udd})
		if err != nil || b == nil {
			t.Fatalf("NewBrowser(%s): b=%v err=%v", udd, b, err)
		}
		srcs := b.ArchiveSources(category.All())
		if len(srcs) == 0 {
			t.Fatalf("%s: no archive sources", udd)
		}
		for _, src := range srcs {
			if want := filepath.Join(b.UserDataDir(), filepath.FromSlash(src.LayoutRel)); src.AbsPath != want {
				t.Errorf("AbsPath = %q, want %q", src.AbsPath, want)
			}
		}
	}
}

func TestWriteArchive_CountsOnlyCapturedSources(t *testing.T) {
	origin := t.TempDir()
	for _, rel := range []string{"Local State", "Default/Preferences", "Default/History", "Default/Bookmarks"} {
		writeArchiveFile(t, origin, rel)
	}
	b, err := chromium.NewBrowser(types.BrowserConfig{Key: "chrome", Name: "chrome", Kind: types.Chromium, UserDataDir: origin})
	if err != nil || b == nil {
		t.Fatalf("NewBrowser: b=%v err=%v", b, err)
	}
	if err := os.Remove(filepath.Join(origin, "Default", "History")); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "data.zip")
	n, err := WriteArchive([]Browser{b}, []category.Category{category.History, category.Bookmark}, zipPath)
	if err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}
	if n != 3 {
		t.Errorf("WriteArchive counted %d entries, want 3: the vanished History must not count", n)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var got []string
	for _, f := range r.File {
		got = append(got, f.Name)
	}
	sort.Strings(got)
	want := []string{"chrome/Default/Bookmarks", "chrome/Default/Preferences", "chrome/Local State"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("archive entries = %q, want %q", got, want)
	}
}

func writeArchiveFile(t *testing.T, root, rel string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rel), 0o600); err != nil {
		t.Fatal(err)
	}
}
