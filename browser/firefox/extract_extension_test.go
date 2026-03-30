package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractExtensions(t *testing.T) {
	path := createTestJSON(t, "extensions.json", `{
		"addons": [
			{
				"id": "ublock@gorhill.org",
				"location": "app-profile",
				"version": "1.52.0",
				"active": true,
				"defaultLocale": {
					"name": "uBlock Origin",
					"description": "An efficient blocker"
				}
			},
			{
				"id": "system@mozilla.org",
				"location": "app-system-defaults",
				"version": "1.0",
				"defaultLocale": {"name": "System Addon"}
			},
			{
				"id": "bitwarden@bitwarden.com",
				"location": "app-profile",
				"version": "2024.1.0",
				"active": true,
				"defaultLocale": {
					"name": "Bitwarden",
					"description": "Password manager"
				}
			}
		]
	}`)

	got, err := extractExtensions(path)
	require.NoError(t, err)
	require.Len(t, got, 2) // system addon filtered out

	ids := map[string]bool{}
	for _, ext := range got {
		ids[ext.ID] = true
		assert.NotEmpty(t, ext.Name)
		assert.NotEmpty(t, ext.Version)
	}
	assert.True(t, ids["ublock@gorhill.org"])
	assert.True(t, ids["bitwarden@bitwarden.com"])
	assert.False(t, ids["system@mozilla.org"])
}

func TestExtractExtensions_EmptyAddons(t *testing.T) {
	path := createTestJSON(t, "extensions.json", `{"addons": []}`)
	got, err := extractExtensions(path)
	require.NoError(t, err)
	assert.Empty(t, got)
}
