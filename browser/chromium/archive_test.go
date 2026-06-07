package chromium

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moond4rk/hackbrowserdata/types"
)

func TestArchiveSources_ForwardSlashLayout(t *testing.T) {
	udd := t.TempDir()
	networkDir := filepath.Join(udd, "Default", "Network")
	if err := os.MkdirAll(networkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(udd, "Default", "Preferences"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(networkDir, "Cookies"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(udd, "Local State"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := NewBrowser(types.BrowserConfig{Key: "chrome", Name: "chrome", Kind: types.Chromium, UserDataDir: udd})
	if err != nil || b == nil {
		t.Fatalf("NewBrowser: b=%v err=%v", b, err)
	}

	srcs := b.ArchiveSources([]types.Category{types.Cookie})

	var gotCookie, gotMarker, gotLocalState bool
	for _, s := range srcs {
		if strings.Contains(s.LayoutRel, `\`) {
			t.Errorf("LayoutRel must be forward-slash, got %q", s.LayoutRel)
		}
		switch s.LayoutRel {
		case "Default/Network/Cookies":
			gotCookie = true
		case "Default/Preferences":
			gotMarker = true
		case "Local State":
			gotLocalState = true
		}
	}
	if !gotCookie {
		t.Errorf("missing Cookies entry with layout path, got %+v", srcs)
	}
	if !gotMarker {
		t.Errorf("missing Preferences marker entry (needed for restore profile discovery), got %+v", srcs)
	}
	if !gotLocalState {
		t.Errorf("missing Local State entry (User Data root file), got %+v", srcs)
	}
}

func TestArchiveSources_FlatLayoutNoExtraLevel(t *testing.T) {
	// Flat-layout install: data lives directly under UserDataDir with no Default/ subdir, so
	// discoverProfiles falls back to UserDataDir itself as the profile (profileDir == root).
	udd := t.TempDir()
	if err := os.WriteFile(filepath.Join(udd, "History"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := NewBrowser(types.BrowserConfig{Key: "opera", Name: "opera", Kind: types.Chromium, UserDataDir: udd})
	if err != nil || b == nil {
		t.Fatalf("NewBrowser: b=%v err=%v", b, err)
	}

	srcs := b.ArchiveSources([]types.Category{types.History})

	phantom := filepath.Base(udd) + "/"
	var gotHistory bool
	for _, s := range srcs {
		if strings.HasPrefix(s.LayoutRel, phantom) {
			t.Errorf("flat layout must not insert a %q level, got %q", phantom, s.LayoutRel)
		}
		if s.LayoutRel == "History" {
			gotHistory = true
		}
	}
	if !gotHistory {
		t.Errorf("expected History at archive root, got %+v", srcs)
	}
}
