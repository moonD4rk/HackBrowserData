package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func setupExtensionJSON(t *testing.T) string {
	t.Helper()
	return createTestJSON(t, "Secure Preferences", `{
		"extensions": {
			"settings": {
				"abc123": {
					"location": 1,
					"manifest": {
						"name": "React DevTools",
						"description": "React debugging",
						"version": "4.28.0"
					}
				},
				"system-ext": {
					"location": 5,
					"manifest": {"name": "System", "version": "1.0"}
				},
				"component-ext": {
					"location": 10,
					"manifest": {"name": "Component", "version": "1.0"}
				},
				"def456": {
					"location": 1,
					"manifest": {
						"name": "uBlock Origin",
						"description": "Ad blocker",
						"version": "1.52.0"
					}
				}
			}
		}
	}`)
}

func TestExtractExtensions(t *testing.T) {
	path := setupExtensionJSON(t)

	var out testCollector
	err := extractExtensions(path, &out)
	got := collected[record.Extension](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 2) // system (location=5) and component (location=10) skipped

	// Verify field mapping (order may vary since gjson.ForEach iterates map)
	ids := map[string]bool{}
	for _, ext := range got {
		ids[ext.ID] = true
		assert.NotEmpty(t, ext.Name)
		assert.NotEmpty(t, ext.Version)
		assert.NotEmpty(t, ext.Description)
	}
	assert.True(t, ids["abc123"])
	assert.True(t, ids["def456"])
	assert.False(t, ids["system-ext"])
}

func TestExtractExtensions_NoManifestSkipped(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{
		"extensions": {
			"settings": {
				"no-manifest": {"location": 1}
			}
		}
	}`)

	var out testCollector
	err := extractExtensions(path, &out)
	got := collected[record.Extension](t, &out)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestExtractExtensions_MissingSettingsPath(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{"something": "else"}`)
	err := extractExtensions(path, &testCollector{})
	require.Error(t, err)
}

func TestExtractOperaExtensions(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{
		"extensions": {
			"opsettings": {
				"opera-ext-1": {
					"location": 1,
					"manifest": {
						"name": "Opera Ad Blocker",
						"description": "Blocks ads in Opera",
						"version": "2.0.0"
					},
					"state": 1
				},
				"system-ext": {
					"location": 5,
					"manifest": {"name": "System", "version": "1.0"}
				}
			}
		}
	}`)

	// extractOperaExtensions should find extensions under opsettings
	var out testCollector
	err := extractOperaExtensions(path, &out)
	got := collected[record.Extension](t, &out)
	require.NoError(t, err)
	require.Len(t, got, 1) // system extension skipped
	assert.Equal(t, "Opera Ad Blocker", got[0].Name)
	assert.Equal(t, "2.0.0", got[0].Version)
	assert.True(t, got[0].Enabled)

	// Standard extractExtensions should fail on the same file (no "extensions.settings")
	err = extractExtensions(path, &testCollector{})
	require.Error(t, err)
}
