package browser

import (
	"bytes"
	"errors"
	"runtime"
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
	if dump.Vaults[0].Browser != chromeName {
		t.Errorf("Browser = %q, want %q", dump.Vaults[0].Browser, chromeName)
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
	if dump.Vaults[0].Browser != chromeName {
		t.Errorf("Browser = %q, want %q", dump.Vaults[0].Browser, chromeName)
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

func TestApplyDump_Match(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
	}
	dump := masterkey.Dump{
		Vaults: []masterkey.Vault{
			{Browser: chromeName, UserDataDir: testUDD, Keys: masterkey.MasterKeys{V10: []byte("v10-from-dump")}},
		},
	}
	ApplyDump([]Browser{b}, dump)

	if b.receivedRetrievers.V10 == nil {
		t.Fatal("V10 retriever should be set from matching vault")
	}
	got, err := b.receivedRetrievers.V10.RetrieveKey(masterkey.Hints{})
	if err != nil || string(got) != "v10-from-dump" {
		t.Errorf("V10.RetrieveKey() = %q, err = %v, want %q", got, err, "v10-from-dump")
	}
	if b.receivedRetrievers.V11 != nil {
		t.Errorf("V11 should be nil (tier not in dump), got %v", b.receivedRetrievers.V11)
	}
}

func TestApplyDump_MissingVault(t *testing.T) {
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
	}
	dump := masterkey.Dump{
		Vaults: []masterkey.Vault{
			{Browser: testEdgeName, UserDataDir: "/edge", Keys: masterkey.MasterKeys{V10: []byte("v10")}},
		},
	}
	ApplyDump([]Browser{b}, dump)

	if b.receivedRetrievers.V10 != nil {
		t.Errorf("V10 should remain nil when no matching vault, got %v", b.receivedRetrievers.V10)
	}
}

func TestApplyDump_NonKeyManagerSkipped(t *testing.T) {
	firefox := &mockBrowser{name: firefoxName, userDataDir: "/ff", profiles: []string{"default-release"}}
	dump := masterkey.Dump{
		Vaults: []masterkey.Vault{
			{Browser: firefoxName, UserDataDir: "/ff", Keys: masterkey.MasterKeys{V10: []byte("v10")}},
		},
	}
	// firefox does not implement KeyManager; ApplyDump must not panic and must not attempt injection.
	ApplyDump([]Browser{firefox}, dump)
}

func TestApplyDump_RoundTrip(t *testing.T) {
	src := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
		keys:        masterkey.MasterKeys{V10: []byte("v10-rt"), V20: []byte("v20-rt")},
	}
	dump := BuildDump([]Browser{src})

	dst := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: testUDD, profiles: []string{testProfileDefault}},
	}
	ApplyDump([]Browser{dst}, dump)

	v10, _ := dst.receivedRetrievers.V10.RetrieveKey(masterkey.Hints{})
	if string(v10) != "v10-rt" {
		t.Errorf("V10 round-trip: got %q, want v10-rt", v10)
	}
	v20, _ := dst.receivedRetrievers.V20.RetrieveKey(masterkey.Hints{})
	if string(v20) != "v20-rt" {
		t.Errorf("V20 round-trip: got %q, want v20-rt", v20)
	}
	if dst.receivedRetrievers.V11 != nil {
		t.Errorf("V11 should be nil (not in source keys), got %v", dst.receivedRetrievers.V11)
	}
}

func TestApplyDump_FallbackOnPathMismatch(t *testing.T) {
	// Cross-host scenario: dump was created on Windows but is applied on Linux/macOS where the
	// UserDataDir literally differs. With a single vault for the browser, ApplyDump should still
	// inject — otherwise the primary cross-host use case fails silently.
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: "/local/chrome", profiles: []string{testProfileDefault}},
	}
	dump := masterkey.Dump{
		Vaults: []masterkey.Vault{
			{
				Browser:     chromeName,
				UserDataDir: `C:\Users\foo\AppData\Local\Google\Chrome\User Data`,
				Keys:        masterkey.MasterKeys{V10: []byte("v10-fallback")},
			},
		},
	}
	ApplyDump([]Browser{b}, dump)

	if b.receivedRetrievers.V10 == nil {
		t.Fatal("V10 retriever should be set via single-vault fallback")
	}
	got, err := b.receivedRetrievers.V10.RetrieveKey(masterkey.Hints{})
	if err != nil || string(got) != "v10-fallback" {
		t.Errorf("V10.RetrieveKey() = %q, err = %v, want %q", got, err, "v10-fallback")
	}
}

func TestApplyDump_NoFallbackWhenAmbiguous(t *testing.T) {
	// Two Chrome vaults in the dump and no exact path match — ApplyDump must not guess which
	// installation the local browser corresponds to.
	b := &mockChromiumBrowser{
		mockBrowser: mockBrowser{name: chromeName, userDataDir: "/local/chrome", profiles: []string{testProfileDefault}},
	}
	dump := masterkey.Dump{
		Vaults: []masterkey.Vault{
			{Browser: chromeName, UserDataDir: "/path/a", Keys: masterkey.MasterKeys{V10: []byte("a")}},
			{Browser: chromeName, UserDataDir: "/path/b", Keys: masterkey.MasterKeys{V10: []byte("b")}},
		},
	}
	ApplyDump([]Browser{b}, dump)

	if b.receivedRetrievers.V10 != nil {
		t.Errorf("V10 should remain nil when fallback is ambiguous, got %v", b.receivedRetrievers.V10)
	}
}
