package export

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var refTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

func TestStructCSVRow(t *testing.T) {
	tests := []struct {
		name   string
		record record.Record
		expect []string
	}{
		{
			"Login",
			&record.Login{
				Browser: "Chrome", Profile: "Default",
				URL: "https://example.com", Username: "alice", Password: "secret", CreatedAt: refTime, Store: record.StoreLocal,
			},
			[]string{"Chrome", "Default", "https://example.com", "alice", "secret", "2026-01-15T10:30:00Z", "local"},
		},
		{
			"Cookie",
			&record.Cookie{
				Host: ".example.com", Path: "/", Name: "session", Value: "abc",
				IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: false,
				ExpireAt: refTime, CreatedAt: refTime,
			},
			[]string{"", "", ".example.com", "/", "session", "abc", "true", "true", "true", "false", "2026-01-15T10:30:00Z", "2026-01-15T10:30:00Z", ""},
		},
		{
			"History_int",
			&record.History{URL: "https://a.com", Title: "A", VisitCount: 42, LastVisit: refTime},
			[]string{"", "", "https://a.com", "A", "42", "2026-01-15T10:30:00Z"},
		},
		{
			"Download_int64",
			&record.Download{URL: "https://a.com", TargetPath: "/tmp/f", MimeType: "text/plain", TotalBytes: 1024, StartTime: refTime, EndTime: refTime},
			[]string{"", "", "https://a.com", "/tmp/f", "text/plain", "1024", "2026-01-15T10:30:00Z", "2026-01-15T10:30:00Z"},
		},
		{
			"Extension_bool",
			&record.Extension{Name: "ext", ID: "abc", Description: "desc", Version: "1.0", HomepageURL: "https://x.com", Enabled: true},
			[]string{"", "", "ext", "abc", "desc", "1.0", "https://x.com", "true"},
		},
		{
			"zero_time",
			&record.Login{URL: "https://a.com"},
			[]string{"", "", "https://a.com", "", "", "", ""},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expect, structCSVRow(testCase.record))
		})
	}
}
