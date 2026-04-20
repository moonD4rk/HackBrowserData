package keyretriever

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingRetriever captures call count and arguments so tests can verify each tier's retriever
// is invoked exactly once with the expected storage and localStatePath.
type recordingRetriever struct {
	key []byte
	err error

	calls      int
	gotStorage string
	gotPath    string
}

func (r *recordingRetriever) RetrieveKey(storage, localStatePath string) ([]byte, error) {
	r.calls++
	r.gotStorage = storage
	r.gotPath = localStatePath
	return r.key, r.err
}

func TestNewMasterKeys_Matrix(t *testing.T) {
	k10 := bytes.Repeat([]byte{0x10}, 32)
	k11 := bytes.Repeat([]byte{0x11}, 32)
	k20 := bytes.Repeat([]byte{0x20}, 32)

	tests := []struct {
		name         string
		v10          *recordingRetriever
		v11          *recordingRetriever
		v20          *recordingRetriever
		wantV10      []byte
		wantV11      []byte
		wantV20      []byte
		wantErrParts []string // substrings that must all appear in the joined error; nil = no error
	}{
		{
			name:    "Windows happy path (V10+V20 ok, V11 not configured)",
			v10:     &recordingRetriever{key: k10},
			v20:     &recordingRetriever{key: k20},
			wantV10: k10, wantV20: k20,
		},
		{
			name:    "Linux happy path (V10+V11 ok, V20 not configured)",
			v10:     &recordingRetriever{key: k10},
			v11:     &recordingRetriever{key: k11},
			wantV10: k10, wantV11: k11,
		},
		{
			name:    "macOS happy path (V10 only)",
			v10:     &recordingRetriever{key: k10},
			wantV10: k10,
		},
		{
			name:    "all three tiers succeed",
			v10:     &recordingRetriever{key: k10},
			v11:     &recordingRetriever{key: k11},
			v20:     &recordingRetriever{key: k20},
			wantV10: k10, wantV11: k11, wantV20: k20,
		},
		{
			name:         "one tier errors, others succeed (degraded)",
			v10:          &recordingRetriever{key: k10},
			v20:          &recordingRetriever{err: errors.New("inject failed")},
			wantV10:      k10,
			wantErrParts: []string{"v20: inject failed"},
		},
		{
			name:         "two tiers error, one succeeds",
			v10:          &recordingRetriever{key: k10},
			v11:          &recordingRetriever{err: errors.New("dbus failed")},
			v20:          &recordingRetriever{err: errors.New("inject failed")},
			wantV10:      k10,
			wantErrParts: []string{"v11: dbus failed", "v20: inject failed"},
		},
		{
			name:         "all three tiers error (total failure)",
			v10:          &recordingRetriever{err: errors.New("dpapi failed")},
			v11:          &recordingRetriever{err: errors.New("dbus failed")},
			v20:          &recordingRetriever{err: errors.New("inject failed")},
			wantErrParts: []string{"v10: dpapi failed", "v11: dbus failed", "v20: inject failed"},
		},
		{
			name:    "tier returns (nil, nil) — not applicable, silent",
			v10:     &recordingRetriever{key: k10},
			v20:     &recordingRetriever{}, // ABERetriever on non-ABE fork
			wantV10: k10,
		},
		{
			name: "all tiers (nil, nil) — no keys, no errors",
			v10:  &recordingRetriever{},
			v11:  &recordingRetriever{},
			v20:  &recordingRetriever{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Retrievers
			if tt.v10 != nil {
				r.V10 = tt.v10
			}
			if tt.v11 != nil {
				r.V11 = tt.v11
			}
			if tt.v20 != nil {
				r.V20 = tt.v20
			}

			keys, err := NewMasterKeys(r, "chrome", "/tmp/Local State")
			assert.Equal(t, tt.wantV10, keys.V10)
			assert.Equal(t, tt.wantV11, keys.V11)
			assert.Equal(t, tt.wantV20, keys.V20)

			if len(tt.wantErrParts) == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				for _, part := range tt.wantErrParts {
					assert.Contains(t, err.Error(), part, "joined error should mention each failing tier")
				}
			}

			// Every configured retriever must be called exactly once — this is the property
			// that prevents any regression where a tier is silently bypassed.
			for name, mock := range map[string]*recordingRetriever{"V10": tt.v10, "V11": tt.v11, "V20": tt.v20} {
				if mock == nil {
					continue
				}
				assert.Equal(t, 1, mock.calls, "%s retriever should be called exactly once", name)
				assert.Equal(t, "chrome", mock.gotStorage)
				assert.Equal(t, "/tmp/Local State", mock.gotPath)
			}
		})
	}
}

func TestNewMasterKeys_AllNilRetrievers(t *testing.T) {
	// All slots nil — macOS/Linux with no retriever wiring, or Windows with neither tier set up.
	keys, err := NewMasterKeys(Retrievers{}, "chrome", "/tmp/Local State")
	require.NoError(t, err)
	assert.Nil(t, keys.V10)
	assert.Nil(t, keys.V11)
	assert.Nil(t, keys.V20)
}

func TestNewMasterKeys_PartialNil(t *testing.T) {
	// Only V10 wired — typical macOS shape. V11/V20 left nil.
	k10 := []byte("v10-key-bytes-for-testing")
	r := &recordingRetriever{key: k10}
	keys, err := NewMasterKeys(Retrievers{V10: r}, "Chrome", "")

	require.NoError(t, err)
	assert.Equal(t, k10, keys.V10)
	assert.Nil(t, keys.V11)
	assert.Nil(t, keys.V20)
	assert.Equal(t, 1, r.calls)
	assert.Equal(t, "Chrome", r.gotStorage)
}

func TestNewMasterKeys_ErrorWrapping(t *testing.T) {
	// errors.Is should traverse errors.Join to find the original error — useful for callers
	// that want to check for specific error types without string matching.
	sentinel := errors.New("sentinel")
	r := Retrievers{V20: &recordingRetriever{err: sentinel}}

	_, err := NewMasterKeys(r, "chrome", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel, "errors.Is should find wrapped sentinel error")
}
