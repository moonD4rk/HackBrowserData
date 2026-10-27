package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
)

func TestReport(t *testing.T) {
	cookie := Extraction{DirID: "chrome", Profile: "Default", Category: category.Cookie}
	var buf bytes.Buffer
	require.NoError(t, Report(&buf, Summary{Results: []Result{
		{Extraction: cookie, Outcome: OutcomePartial, Count: 224, Failed: 4},
		dirFailure("arc-2", errors.New("permission denied")),
		{Extraction: Extraction{DirID: "edge", Profile: "Default", Category: category.Password}},
	}}))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, []string{"chrome", "Default", "cookie", "partial", "224", "4", "not", "recovered"}, strings.Fields(lines[1]))
	assert.Equal(t, []string{"arc-2", "-", "-", "error", "0", "permission", "denied"}, strings.Fields(lines[2]))
	assert.Equal(t, "unset", strings.Fields(lines[3])[3], "a result whose outcome was never set must not read as ok")
}
