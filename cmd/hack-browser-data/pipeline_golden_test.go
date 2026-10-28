package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	ibrowser "github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// Until the switch-over the pipeline runs beside today's extract path: on the same fixtures it must write the same
// goldens byte for byte.
func TestPipelineGolden(t *testing.T) {
	home := t.TempDir()
	keys := make(map[ibrowser.DirID]masterkey.Set)
	for _, inst := range chromiumFixtureInstalls {
		def, ok := ibrowser.Lookup(inst.key)
		require.True(t, ok, inst.key)
		require.Equal(t, inst.name, def.Name, "records are stamped with the table's display name")
		chromiumFixtureUserData(t, filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0])), inst.key)
		keys[ibrowser.DirID(inst.key)] = chromiumFixtureKeys
	}

	assertExtractGoldens(t, "extract", func(t *testing.T, format string) string {
		return pipelineGoldenRun(t, home, keys, format)
	})
}

func pipelineGoldenRun(t *testing.T, home string, keys map[ibrowser.DirID]masterkey.Set, format string) string {
	t.Helper()
	out := t.TempDir()
	req, err := app.Validate(app.Flags{Browser: "all", Categories: "all", Format: format, Dir: out})
	require.NoError(t, err)
	req.Home, req.Keys = home, keys

	plan := app.NewPlan(app.Discover(req), req)
	_, err = app.Execute(context.Background(), plan, req)
	require.NoError(t, err)
	return out
}
