package browser

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/moond4rk/hackbrowserdata/masterkey"
	"github.com/moond4rk/hackbrowserdata/types"
)

const (
	testProfileDefault = "Default"
	testProfile1       = "Profile 1"
	testUDD            = "/p"
	testEdgeName       = "Edge"
)

// mockBrowser is one installation holding zero or more profile names.
type mockBrowser struct {
	name, userDataDir string
	profiles          []string
}

func (m *mockBrowser) BrowserName() string { return m.name }
func (m *mockBrowser) UserDataDir() string { return m.userDataDir }

func (m *mockBrowser) Profiles() []types.Profile {
	out := make([]types.Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		out = append(out, types.Profile{Name: p, Dir: m.userDataDir + "/" + p})
	}
	return out
}

func (m *mockBrowser) Extract(_ []types.Category) ([]types.ExtractResult, error) {
	return nil, nil
}

func (m *mockBrowser) CountEntries(_ []types.Category) ([]types.CountResult, error) {
	return nil, nil
}

type mockChromiumBrowser struct {
	mockBrowser
	key                string
	kind               types.BrowserKind
	keys               masterkey.MasterKeys
	exportErr          error
	calls              int
	receivedRetrievers masterkey.Retrievers
}

func (m *mockChromiumBrowser) SetRetrievers(r masterkey.Retrievers) {
	m.receivedRetrievers = r
}

func (m *mockChromiumBrowser) ExportKeys() (masterkey.MasterKeys, error) {
	m.calls++
	return m.keys, m.exportErr
}

func (m *mockChromiumBrowser) BrowserKey() string {
	if m.key != "" {
		return m.key
	}
	return strings.ToLower(m.name)
}

func (m *mockChromiumBrowser) Kind() types.BrowserKind { return m.kind }

func TestBuildDump_Empty(t *testing.T) {
	dump := BuildDump(nil)
	if dump.Version != masterkey.DumpVersion {
		t.Errorf("Version = %q, want %q", dump.Version, masterkey.DumpVersion)
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
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
		keys:        masterkey.MasterKeys{V10: []byte("v10-key")},
	}

	dump := BuildDump([]Browser{b})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1", len(dump.Vaults))
	}
	inst := dump.Vaults[0]
	if !strings.EqualFold(inst.Browser, chromeName) || inst.UserDataDir != testUDD {
		t.Errorf("inst metadata = %+v", inst)
	}
	if inst.Kind != "chromium" {
		t.Errorf("Kind = %q, want chromium", inst.Kind)
	}
	if len(inst.Profiles) != 1 || inst.Profiles[0] != testProfileDefault {
		t.Errorf("Profiles = %v", inst.Profiles)
	}
	if string(inst.Keys.V10) != "v10-key" {
		t.Errorf("Keys.V10 = %q", inst.Keys.V10)
	}
}

// TestBuildDump_MultipleProfilesOneVault verifies that one installation holding
// multiple profiles produces a single vault with all profile names, deriving the
// key exactly once.
func TestBuildDump_MultipleProfilesOneVault(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault, testProfile1}},
		keys:        masterkey.MasterKeys{V10: []byte("v10")},
	}

	dump := BuildDump([]Browser{b})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (one installation = one vault)", len(dump.Vaults))
	}
	if len(dump.Vaults[0].Profiles) != 2 {
		t.Errorf("Profiles = %v, want both profiles", dump.Vaults[0].Profiles)
	}
	if b.calls != 1 {
		t.Errorf("ExportKeys calls = %d, want 1 (one call per installation)", b.calls)
	}
}

func TestBuildDump_SkipsNonKeyManager(t *testing.T) {
	chrome := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: "/chrome", profiles: []string{testProfileDefault}},
		keys:        masterkey.MasterKeys{V10: []byte("v10")},
	}
	firefox := &mockBrowser{name: firefoxName, userDataDir: "/ff", profiles: []string{"default-release"}}

	dump := BuildDump([]Browser{chrome, firefox})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (firefox skipped)", len(dump.Vaults))
	}
	if !strings.EqualFold(dump.Vaults[0].Browser, chromeName) {
		t.Errorf("Browser = %q, want %q", dump.Vaults[0].Browser, strings.ToLower(chromeName))
	}
}

func TestBuildDump_SkipsExportError(t *testing.T) {
	good := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: "/chrome", profiles: []string{testProfileDefault}},
		keys:        masterkey.MasterKeys{V10: []byte("v10")},
	}
	failing := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: testEdgeName, userDataDir: "/edge", profiles: []string{testProfileDefault}},
		exportErr:   errors.New("retriever failed"),
	}

	dump := BuildDump([]Browser{good, failing})

	if len(dump.Vaults) != 1 {
		t.Fatalf("Vaults len = %d, want 1 (failing browser skipped)", len(dump.Vaults))
	}
	if !strings.EqualFold(dump.Vaults[0].Browser, chromeName) {
		t.Errorf("Browser = %q, want %q", dump.Vaults[0].Browser, strings.ToLower(chromeName))
	}
}

func TestBuildDump_JSONRoundTrip(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
		keys:        masterkey.MasterKeys{V10: []byte{0x01, 0x02, 0x03}, V20: []byte{0xff, 0xee}},
	}

	dump := BuildDump([]Browser{b})

	var buf bytes.Buffer
	if err := dump.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	parsed, err := masterkey.ReadJSON(&buf)
	if err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	if parsed.Version != dump.Version {
		t.Errorf("Version round-trip: got %q, want %q", parsed.Version, dump.Version)
	}
	if len(parsed.Vaults) != 1 {
		t.Fatalf("Vaults len = %d", len(parsed.Vaults))
	}
	if parsed.Vaults[0].Kind != "chromium" {
		t.Errorf("Kind round-trip: got %q, want chromium", parsed.Vaults[0].Kind)
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
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
		keys:        masterkey.MasterKeys{V10: []byte("v10")},
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

func TestKindDumpRoundTrip(t *testing.T) {
	for _, k := range []types.BrowserKind{types.Chromium, types.ChromiumYandex, types.ChromiumOpera} {
		s, err := kindToDump(k)
		if err != nil {
			t.Fatalf("kindToDump(%d): %v", k, err)
		}
		got, err := kindFromDump(s)
		if err != nil || got != k {
			t.Errorf("round trip %d -> %q -> %d (err %v)", k, s, got, err)
		}
	}
	if _, err := kindToDump(types.Firefox); err == nil {
		t.Error("kindToDump(Firefox) should error")
	}
	if _, err := kindFromDump("nope"); err == nil {
		t.Error("kindFromDump(nope) should error")
	}
}

func TestRetrieversFromKeys(t *testing.T) {
	r := retrieversFromKeys(masterkey.MasterKeys{V10: []byte("k10"), V20: []byte("k20")})

	if r.V10 == nil || r.V20 == nil {
		t.Fatal("V10 and V20 retrievers should be set from non-empty keys")
	}
	if r.V11 != nil {
		t.Error("V11 retriever should be nil when the key is absent")
	}
	if got, _ := r.V10.RetrieveKey(masterkey.Hints{}); string(got) != "k10" {
		t.Errorf("V10 key = %q, want k10", got)
	}
	if got, _ := r.V20.RetrieveKey(masterkey.Hints{}); string(got) != "k20" {
		t.Errorf("V20 key = %q, want k20", got)
	}
}

// makeUserData writes a minimal Chromium profile tree: a Preferences marker plus History (a real
// extraction source, so the profile resolves) under each named profile dir.
func makeUserData(t *testing.T, root string, profiles ...string) {
	t.Helper()
	for _, p := range profiles {
		dir := filepath.Join(root, p)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"Preferences", "History"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestBuildFromDump_ConventionMultiBrowser(t *testing.T) {
	dataDir := t.TempDir()
	makeUserData(t, filepath.Join(dataDir, "chrome"), testProfileDefault)
	makeUserData(t, filepath.Join(dataDir, "edge"), testProfileDefault)
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "chrome", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("c")}},
		{Browser: "edge", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("e")}},
	}}

	browsers, err := BuildFromDump(dump, dataDir, "")
	if err != nil {
		t.Fatalf("BuildFromDump: %v", err)
	}
	if len(browsers) != 2 {
		t.Fatalf("got %d browsers, want 2", len(browsers))
	}
}

// TestBuildFromDump_ForeignKindNoPlatformTable proves restore never consults platformBrowsers():
// sogou is Windows-only yet reconstructs from its vault on any OS.
func TestBuildFromDump_ForeignKindNoPlatformTable(t *testing.T) {
	dataDir := t.TempDir()
	makeUserData(t, filepath.Join(dataDir, "sogou"), testProfileDefault)
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "sogou", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("k")}},
	}}

	browsers, err := BuildFromDump(dump, dataDir, "")
	if err != nil {
		t.Fatalf("BuildFromDump: %v", err)
	}
	if len(browsers) != 1 {
		t.Fatalf("got %d browsers, want 1", len(browsers))
	}
	if browsers[0].BrowserName() != "sogou" {
		t.Errorf("BrowserName = %q, want sogou", browsers[0].BrowserName())
	}
}

func TestBuildFromDump_RawSingleBrowser(t *testing.T) {
	dataDir := t.TempDir()
	makeUserData(t, dataDir, testProfileDefault)
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "chrome", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("c")}},
	}}

	browsers, err := BuildFromDump(dump, dataDir, "chrome")
	if err != nil {
		t.Fatalf("BuildFromDump: %v", err)
	}
	if len(browsers) != 1 {
		t.Fatalf("got %d browsers, want 1", len(browsers))
	}
	if browsers[0].UserDataDir() != dataDir {
		t.Errorf("UserDataDir = %q, want %q (raw root)", browsers[0].UserDataDir(), dataDir)
	}
}

func TestBuildFromDump_UnknownBrowserErrors(t *testing.T) {
	dataDir := t.TempDir()
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "chrome", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("c")}},
	}}
	if _, err := BuildFromDump(dump, dataDir, "sogou"); err == nil {
		t.Fatal("expected error for -b matching no vault")
	}
}

func TestBuildFromDump_RawAmbiguousErrors(t *testing.T) {
	dataDir := t.TempDir()
	makeUserData(t, dataDir, testProfileDefault)
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "chrome", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("c")}},
		{Browser: "edge", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("e")}},
	}}
	if _, err := BuildFromDump(dump, dataDir, ""); err == nil {
		t.Fatal("expected ambiguity error for raw multi-vault restore without -b")
	}
}

func TestBuildFromDump_MissingDataDirErrors(t *testing.T) {
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "chrome", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("c")}},
	}}
	if _, err := BuildFromDump(dump, "/no/such/dir", ""); err == nil {
		t.Fatal("expected error when data dir does not exist")
	}
}

// TestBuildFromDump_MarkerlessTreeStillResolves covers an archive/copy that omitted Preferences:
// the source-bearing-subdir fallback in discoverProfiles must still find the profile.
func TestBuildFromDump_MarkerlessTreeStillResolves(t *testing.T) {
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "chrome", testProfileDefault)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "History"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dump := masterkey.Dump{Vaults: []masterkey.Vault{
		{Browser: "chrome", Kind: "chromium", Keys: masterkey.MasterKeys{V10: []byte("c")}},
	}}

	browsers, err := BuildFromDump(dump, dataDir, "")
	if err != nil {
		t.Fatalf("BuildFromDump: %v", err)
	}
	if len(browsers) != 1 {
		t.Fatalf("got %d browsers, want 1 (marker-less profile must resolve via source fallback)", len(browsers))
	}
}
