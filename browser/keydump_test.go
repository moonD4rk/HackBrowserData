package browser

import (
	"bytes"
	"errors"
	"runtime"
	"testing"

	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
	"github.com/moond4rk/hackbrowserdata/types"
)

const (
	testProfileDefault = "Default"
	testProfile1       = "Profile 1"
	testUDD            = "/p"
	testEdgeName       = "Edge"
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
	calls     int
}

func (m *mockChromiumBrowser) SetKeyRetrievers(_ keyretriever.Retrievers) {}

func (m *mockChromiumBrowser) ExportKeys() (keyretriever.MasterKeys, error) {
	m.calls++
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
	if len(dump.Vaults) != 0 {
		t.Errorf("Vaults len = %d, want 0", len(dump.Vaults))
	}
}

func TestBuildDump_SingleChromium(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, profileDir: "/p/Default", userDataDir: testUDD},
		keys:        keyretriever.MasterKeys{V10: []byte("v10-key")},
	}

	dump := BuildDump([]Browser{b})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1", len(dump.Vaults))
	}
	inst := dump.Vaults[0]
	if inst.Browser != chromeName || inst.UserDataDir != testUDD {
		t.Errorf("inst metadata = %+v", inst)
	}
	if len(inst.Profiles) != 1 || inst.Profiles[0] != testProfileDefault {
		t.Errorf("Profiles = %v", inst.Profiles)
	}
	if string(inst.Keys.V10) != "v10-key" {
		t.Errorf("Keys.V10 = %q", inst.Keys.V10)
	}
}

func TestBuildDump_MultipleProfilesSameInstallation(t *testing.T) {
	p1 := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, userDataDir: testUDD},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
	}
	p2 := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfile1, userDataDir: testUDD},
		exportErr:   errors.New("ExportKeys should not be called for second profile"),
	}

	dump := BuildDump([]Browser{p1, p2})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (same installation grouping)", len(dump.Vaults))
	}
	if len(dump.Vaults[0].Profiles) != 2 {
		t.Errorf("Profiles = %v, want both profiles", dump.Vaults[0].Profiles)
	}
}

func TestBuildDump_SkipsNonKeyManager(t *testing.T) {
	chrome := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, userDataDir: "/chrome"},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
	}
	firefox := &mockBrowser{name: firefoxName, profile: "default-release", userDataDir: "/ff"}

	dump := BuildDump([]Browser{chrome, firefox})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (firefox skipped)", len(dump.Vaults))
	}
	if dump.Vaults[0].Browser != chromeName {
		t.Errorf("Browser = %q, want %q", dump.Vaults[0].Browser, chromeName)
	}
}

func TestBuildDump_SkipsExportError(t *testing.T) {
	good := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, userDataDir: "/chrome"},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
	}
	failing := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: testEdgeName, profile: testProfileDefault, userDataDir: "/edge"},
		exportErr:   errors.New("retriever failed"),
	}

	dump := BuildDump([]Browser{good, failing})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (failing browser skipped)", len(dump.Vaults))
	}
	if dump.Vaults[0].Browser != chromeName {
		t.Errorf("Browser = %q, want %q", dump.Vaults[0].Browser, chromeName)
	}
}

func TestBuildDump_JSONRoundTrip(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, userDataDir: testUDD},
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
	if len(parsed.Vaults) != 1 {
		t.Fatalf("Vaults len = %d", len(parsed.Vaults))
	}
	if !bytes.Equal(parsed.Vaults[0].Keys.V10, dump.Vaults[0].Keys.V10) {
		t.Errorf("V10 round-trip mismatch")
	}
	if !bytes.Equal(parsed.Vaults[0].Keys.V20, dump.Vaults[0].Keys.V20) {
		t.Errorf("V20 round-trip mismatch")
	}
	if parsed.Vaults[0].Keys.V11 != nil {
		t.Errorf("V11 should be omitted (nil), got %v", parsed.Vaults[0].Keys.V11)
	}
}

func TestBuildDump_PartialKeys(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, userDataDir: testUDD},
		keys:        keyretriever.MasterKeys{V10: []byte("v10")},
		exportErr:   errors.New("v20: ABE failed"),
	}

	dump := BuildDump([]Browser{b})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (partial result must be preserved)", len(dump.Vaults))
	}
	if string(dump.Vaults[0].Keys.V10) != "v10" {
		t.Errorf("V10 should be preserved despite V20 error, got %q", dump.Vaults[0].Keys.V10)
	}
	if dump.Vaults[0].Keys.V20 != nil {
		t.Errorf("V20 should remain nil, got %v", dump.Vaults[0].Keys.V20)
	}
}

func TestBuildDump_GroupingOrderIndependent(t *testing.T) {
	for _, name := range []string{"p1 first", "p2 first"} {
		t.Run(name, func(t *testing.T) {
			p1 := &mockChromiumBrowser{
				mockBrowser: mockBrowser{name: chromeName, profile: testProfileDefault, userDataDir: testUDD},
				keys:        keyretriever.MasterKeys{V10: []byte("v10")},
			}
			p2 := &mockChromiumBrowser{
				mockBrowser: mockBrowser{name: chromeName, profile: testProfile1, userDataDir: testUDD},
				keys:        keyretriever.MasterKeys{V10: []byte("v10")},
			}
			list := []Browser{p1, p2}
			if name == "p2 first" {
				list = []Browser{p2, p1}
			}

			dump := BuildDump(list)

			if len(dump.Vaults) != 1 {
				t.Fatalf("Vaults len = %d, want 1", len(dump.Vaults))
			}
			if len(dump.Vaults[0].Profiles) != 2 {
				t.Errorf("Profiles = %v, want 2", dump.Vaults[0].Profiles)
			}
			if calls := p1.calls + p2.calls; calls != 1 {
				t.Errorf("ExportKeys total calls = %d, want 1 (one call per installation)", calls)
			}
		})
	}
}
