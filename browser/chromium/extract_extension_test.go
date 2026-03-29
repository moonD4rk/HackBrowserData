package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractExtensions(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{
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

	got, err := extractExtensions(path)
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

	got, err := extractExtensions(path)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestExtractExtensions_MissingSettingsPath(t *testing.T) {
	path := createTestJSON(t, "Secure Preferences", `{"something": "else"}`)
	_, err := extractExtensions(path)
	require.Error(t, err)
}
