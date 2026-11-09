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

func (r *fakeRetriever) Retrieve(_ context.Context, locator Locator) ([]byte, error) {
	r.calls++
	r.gotLoc = locator
	return r.key, r.err
}

var (
	v10Key = bytes.Repeat([]byte{0x10}, 32)
	v11Key = bytes.Repeat([]byte{0x11}, 32)
	v20Key = bytes.Repeat([]byte{0x20}, 32)
)

func TestRetrieveAll_OneKeyPerVersion(t *testing.T) {
	tests := []struct {
		name       string
		retrievers []*fakeRetriever
		want       Set
	}{
		{
			name:       "Windows shape: v10 and v20",
			retrievers: []*fakeRetriever{{version: V10, key: v10Key}, {version: V20, key: v20Key}},
			want:       Set{V10: v10Key, V20: v20Key},
		},
		{
			name:       "Linux shape: v10 and v11",
			retrievers: []*fakeRetriever{{version: V10, key: v10Key}, {version: V11, key: v11Key}},
			want:       Set{V10: v10Key, V11: v11Key},
		},
		{
			name:       "macOS shape: v10 only",
			retrievers: []*fakeRetriever{{version: V10, key: v10Key}},
			want:       Set{V10: v10Key},
		},
		{
			name:       "no retrievers",
			retrievers: nil,
			want:       Set{},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := retrieveAll(context.Background(), Locator{}, asRetrievers(testCase.retrievers))
			require.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestRetrieveAll_FirstKeyWinsAndStopsTheVersion(t *testing.T) {
	failing := &fakeRetriever{version: V10, err: errors.New("needs root")}
	winner := &fakeRetriever{version: V10, key: v10Key}
	later := &fakeRetriever{version: V10, key: bytes.Repeat([]byte{0xFF}, 16)}

	got, err := retrieveAll(context.Background(), Locator{}, []Retriever{failing, winner, later})
	require.NoError(t, err, "a strategy that failed before a later one succeeded is not an error")
	assert.Equal(t, Set{V10: v10Key}, got)
	assert.Equal(t, []int{1, 1, 0}, []int{failing.calls, winner.calls, later.calls})
}

func TestRetrieveAll_VersionsAreIndependent(t *testing.T) {
	v20Retriever := &fakeRetriever{version: V20, err: errors.New("abe: inject failed")}
	got, err := retrieveAll(context.Background(), Locator{}, []Retriever{
		&fakeRetriever{version: V10, key: v10Key},
		v20Retriever,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v20: abe: inject failed")
	assert.Equal(t, Set{V10: v10Key}, got, "a v20 failure keeps the v10 key")
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
	retriever := &fakeRetriever{version: V10, key: v10Key}
	locator := Locator{LocalState: "/tmp/Local State", SecretLabel: "Chrome", Executable: "chrome.exe", ABE: ABEChromeBase}
	_, err := retrieveAll(context.Background(), locator, []Retriever{retriever})
	require.NoError(t, err)
	assert.Equal(t, locator, retriever.gotLoc)
}

func asRetrievers(fakes []*fakeRetriever) []Retriever {
	retrievers := make([]Retriever, 0, len(fakes))
	for _, fake := range fakes {
		retrievers = append(retrievers, fake)
	}
	return retrievers
}
