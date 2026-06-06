package fileutil

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZipDirUnzip_RoundTrip(t *testing.T) {
	src := t.TempDir()
	files := map[string][]byte{
		"empty.txt":               {},
		"small.txt":               []byte("hello"),
		"Default/Network/Cookies": []byte("cookie-bytes"),
		"sub/big.bin":             bytes.Repeat([]byte("A"), 3<<20), // 3 MiB: exercises the chunked copy loop
	}
	for rel, data := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	zipPath := filepath.Join(t.TempDir(), "out.zip")
	if err := ZipDir(zipPath, src); err != nil {
		t.Fatalf("ZipDir: %v", err)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if strings.Contains(f.Name, `\`) {
			t.Errorf("zip entry name must be forward-slash, got %q", f.Name)
		}
	}
	if err := zr.Close(); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := Unzip(zipPath, dst); err != nil {
		t.Fatalf("Unzip: %v", err)
	}
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("missing %s after Unzip: %v", rel, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: content mismatch (got %d bytes, want %d)", rel, len(got), len(want))
		}
	}
}

func TestUnzip_RejectsZipSlip(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("pwned")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Unzip(zipPath, t.TempDir()); err == nil {
		t.Fatal("Unzip must reject an entry that escapes the destination")
	}
}
