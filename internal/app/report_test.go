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
	cookie := Extraction{UserDataDirID: "chrome", ProfileName: "Default", Category: category.Cookie}
	var buffer bytes.Buffer
	require.NoError(t, Report(&buffer, Summary{
		Results: []Result{
			{Extraction: cookie, Outcome: OutcomePartial, Records: 224, Failures: 4},
			{
				Extraction: Extraction{UserDataDirID: "chrome", ProfileName: "Default", Category: category.Password},
				Outcome:    OutcomeError, Failures: 2, Err: errors.New("password not decrypted"),
			},
			{Extraction: Extraction{UserDataDirID: "edge", ProfileName: "Default", Category: category.Password}},
		},
		DirFailures: []DirFailure{{UserDataDirID: "arc-2", Err: errors.New("permission denied")}},
	}))

	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	require.Len(t, lines, 5)
	assert.Equal(t, []string{"arc-2", "-", "-", "error", "0", "permission", "denied"}, strings.Fields(lines[1]), "a dir failure leads")
	assert.Equal(t, []string{"chrome", "Default", "cookie", "partial", "224", "4", "not", "recovered"}, strings.Fields(lines[2]))
	assert.Equal(t, []string{"chrome", "Default", "password", "error", "0", "password", "not", "decrypted"}, strings.Fields(lines[3]),
		"an error's message wins over the failure count")
	assert.Equal(t, "unset", strings.Fields(lines[4])[3], "a result whose outcome was never set must not read as ok")
}
