package browser

import (
	"bytes"
	"errors"
	"runtime"
	"testing"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/types"
)

type mockBrowser struct {
	name, profile, profileDir, userDataDir string
}

func (m *mockBrowser) BrowserName() string { return m.name }
func (m *mockBrowser) ProfileName() string { return m.profile }
func (m *mockBrowser) ProfileDir() string  { return m.profileDir }
func (m *mockBrowser) UserDataDir() string { return m.userDataDir }

func (m *mockBrowser) Extract(_ []types.Category) (*types.BrowserData, error) {
	return &types.BrowserData{}, nil
}

func (m *mockBrowser) CountEntries(_ []types.Category) (map[types.Category]int, error) {
	return nil, nil
}

type mockChromiumBrowser struct {
	mockBrowser
	keys      keyretriever.MasterKeys
	exportErr error
}

func (m *mockChromiumBrowser) SetKeyRetrievers(_ keyretriever.Retrievers) {}

func (m *mockChromiumBrowser) ExportKeys() (keyretriever.MasterKeys, error) {
	return m.keys, m.exportErr
}

func TestBuildDump_Empty(t *testing.T) {
	dump := BuildDump(nil)
	if dump.Version != keyretriever.DumpVersion {
		t.Errorf("Version = %q, want %q", dump.Version, keyretriever.DumpVersion)
	}
	if dump.Host.OS != runtime.GOOS {
		t.Errorf("Host.OS = %q, want %q", dump.Host.OS, runtime.GOOS)
	}
	if len(dump.Installations) != 0 {
		t.Errorf("Installations len = %d, want 0", len(dump.Installations))
	}
}

func TestBuildDump_SingleChromium(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Chrome", profile: "Default", profileDir: "/p/Default", userDataDir: "/p"},
		keys:        keyretriever.MasterKeys{V10: []byte("v10-key")},
	}

	dump := BuildDump([]Browser{b})

	if len(dump.Installations) != 1 {
		t.Fatalf("Installations len = %d, want 1", len(dump.Installations))
	}
	inst := dump.Installations[0]
	if inst.Browser != "Chrome" || inst.UserDataDir != "/p" {
		t.Errorf("inst metadata = %+v", inst)
	}
	if len(inst.Profiles) != 1 || inst.Profiles[0] != "Default" {
		t.Errorf("Profiles = %v", inst.Profiles)
	}
	if string(inst.Keys.V10) != "v10-key" {
		t.Errorf("Keys.V10 = %q", inst.Keys.V10)
	}
}

func TestBuildDump_MultipleProfilesSameInstallation(t *testing.T) {
	p1 := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Chrome", profile: "Default", userDataDir: "/p"},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
	}
	p2 := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Chrome", profile: "Profile 1", userDataDir: "/p"},
		exportErr:   errors.New("ExportKeys should not be called for second profile"),
	}

	dump := BuildDump([]Browser{p1, p2})

	if len(dump.Installations) != 1 {
		t.Fatalf("Installations len = %d, want 1 (same installation grouping)", len(dump.Installations))
	}
	if len(dump.Installations[0].Profiles) != 2 {
		t.Errorf("Profiles = %v, want both Default and Profile 1", dump.Installations[0].Profiles)
	}
}

func TestBuildDump_SkipsNonKeyManager(t *testing.T) {
	chrome := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Chrome", profile: "Default", userDataDir: "/chrome"},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
	}
	firefox := &mockBrowser{name: "Firefox", profile: "default-release", userDataDir: "/ff"}

	dump := BuildDump([]Browser{chrome, firefox})

	if len(dump.Installations) != 1 {
		t.Fatalf("Installations len = %d, want 1 (Firefox skipped)", len(dump.Installations))
	}
	if dump.Installations[0].Browser != "Chrome" {
		t.Errorf("Browser = %q, want Chrome", dump.Installations[0].Browser)
	}
}

func TestBuildDump_SkipsExportError(t *testing.T) {
	good := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Chrome", profile: "Default", userDataDir: "/chrome"},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
	}
	failing := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Edge", profile: "Default", userDataDir: "/edge"},
		exportErr:   errors.New("retriever failed"),
	}

	dump := BuildDump([]Browser{good, failing})

	if len(dump.Installations) != 1 {
		t.Fatalf("Installations len = %d, want 1 (Edge skipped on export error)", len(dump.Installations))
	}
	if dump.Installations[0].Browser != "Chrome" {
		t.Errorf("Browser = %q, want Chrome", dump.Installations[0].Browser)
	}
}

func TestBuildDump_JSONRoundTrip(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: "Chrome", profile: "Default", userDataDir: "/p"},
		keys:        keyretriever.MasterKeys{V10: []byte{0x01, 0x02, 0x03}, V20: []byte{0xff, 0xee}},
	}

	dump := BuildDump([]Browser{b})

	var buf bytes.Buffer
	if err := dump.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	parsed, err := keyretriever.ReadJSON(&buf)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	if parsed.Version != dump.Version {
		t.Errorf("Version round-trip: got %q, want %q", parsed.Version, dump.Version)
	}
	if len(parsed.Installations) != 1 {
		t.Fatalf("Installations len = %d", len(parsed.Installations))
	}
	if !bytes.Equal(parsed.Installations[0].Keys.V10, dump.Installations[0].Keys.V10) {
		t.Errorf("V10 round-trip mismatch")
	}
	if !bytes.Equal(parsed.Installations[0].Keys.V20, dump.Installations[0].Keys.V20) {
		t.Errorf("V20 round-trip mismatch")
	}
	if parsed.Installations[0].Keys.V11 != nil {
		t.Errorf("V11 should be omitted (nil), got %v", parsed.Installations[0].Keys.V11)
	}
}
