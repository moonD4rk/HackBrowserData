package masterkey

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRetriever records its calls so tests can tell which retrievers ran, and with what.
type fakeRetriever struct {
	version Version
	key     []byte
	err     error

	calls  int
	gotLoc Locator
}

func (r *fakeRetriever) Version() Version { return r.version }

func (r *fakeRetriever) Retrieve(_ context.Context, loc Locator) ([]byte, error) {
	r.calls++
	r.gotLoc = loc
	return r.key, r.err
}

var (
	k10 = bytes.Repeat([]byte{0x10}, 32)
	k11 = bytes.Repeat([]byte{0x11}, 32)
	k20 = bytes.Repeat([]byte{0x20}, 32)
)

func TestRetrieveAll_OneKeyPerVersion(t *testing.T) {
	tests := []struct {
		name       string
		retrievers []*fakeRetriever
		want       Set
	}{
		{
			name:       "Windows shape: v10 and v20",
			retrievers: []*fakeRetriever{{version: V10, key: k10}, {version: V20, key: k20}},
			want:       Set{V10: k10, V20: k20},
		},
		{
			name:       "Linux shape: v10 and v11",
			retrievers: []*fakeRetriever{{version: V10, key: k10}, {version: V11, key: k11}},
			want:       Set{V10: k10, V11: k11},
		},
		{
			name:       "macOS shape: v10 only",
			retrievers: []*fakeRetriever{{version: V10, key: k10}},
			want:       Set{V10: k10},
		},
		{
			name:       "no retrievers",
			retrievers: nil,
			want:       Set{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := retrieveAll(context.Background(), Locator{}, asRetrievers(tt.retrievers))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetrieveAll_FirstKeyWinsAndStopsTheVersion(t *testing.T) {
	failing := &fakeRetriever{version: V10, err: errors.New("needs root")}
	winner := &fakeRetriever{version: V10, key: k10}
	later := &fakeRetriever{version: V10, key: bytes.Repeat([]byte{0xFF}, 16)}

	got, err := retrieveAll(context.Background(), Locator{}, []Retriever{failing, winner, later})
	require.NoError(t, err, "a strategy that failed before a later one succeeded is not an error")
	assert.Equal(t, Set{V10: k10}, got)
	assert.Equal(t, []int{1, 1, 0}, []int{failing.calls, winner.calls, later.calls})
}

func TestRetrieveAll_VersionsAreIndependent(t *testing.T) {
	v20 := &fakeRetriever{version: V20, err: errors.New("abe: inject failed")}
	got, err := retrieveAll(context.Background(), Locator{}, []Retriever{
		&fakeRetriever{version: V10, key: k10},
		v20,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v20: abe: inject failed")
	assert.Equal(t, Set{V10: k10}, got, "a v20 failure keeps the v10 key")
}

func TestRetrieveAll_ErrorOnlyWhenEveryRetrieverFails(t *testing.T) {
	first := errors.New("keychain: not found")
	second := errors.New("security command: denied")
	_, err := retrieveAll(context.Background(), Locator{}, []Retriever{
		&fakeRetriever{version: V10, err: first},
		&fakeRetriever{version: V10, err: second},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, first)
	require.ErrorIs(t, err, second)
}

func TestRetrieveAll_NotApplicableIsSilent(t *testing.T) {
	got, err := retrieveAll(context.Background(), Locator{}, []Retriever{
		&fakeRetriever{version: V10, err: ErrNotApplicable},
		&fakeRetriever{version: V20, err: ErrNotApplicable},
		&fakeRetriever{version: V11},
	})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRetrieveAll_PassesTheLocator(t *testing.T) {
	r := &fakeRetriever{version: V10, key: k10}
	loc := Locator{LocalState: "/tmp/Local State", KeychainLabel: "Chrome", Exe: "chrome.exe", ABE: ABEChromeBase}
	_, err := retrieveAll(context.Background(), loc, []Retriever{r})
	require.NoError(t, err)
	assert.Equal(t, loc, r.gotLoc)
}

func asRetrievers(fakes []*fakeRetriever) []Retriever {
	rs := make([]Retriever, 0, len(fakes))
	for _, f := range fakes {
		rs = append(rs, f)
	}
	return rs
}
