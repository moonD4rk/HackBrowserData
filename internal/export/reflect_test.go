package export

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var refTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

// allRecordTypes must list every record type, or a new one escapes these checks.
var allRecordTypes = []record.Record{
	&record.Login{},
	&record.Cookie{},
	&record.Bookmark{},
	&record.History{},
	&record.Download{},
	&record.CreditCard{},
	&record.Extension{},
	&record.LocalStorage{},
	&record.SessionStorage{},
}

// TestAllRecordFieldsHaveCSVTag catches a field without a csv tag, which would be silently left out of the CSV output.
func TestAllRecordFieldsHaveCSVTag(t *testing.T) {
	for _, row := range allRecordTypes {
		recordType := reflect.TypeOf(row).Elem()
		t.Run(recordType.Name(), func(t *testing.T) {
			for i := 0; i < recordType.NumField(); i++ {
				field := recordType.Field(i)
				if !field.IsExported() {
					continue
				}
				_, ok := field.Tag.Lookup("csv")
				assert.True(t, ok, "field %s.%s missing csv tag", recordType.Name(), field.Name)
			}
		})
	}
}

func TestStructCSVHeader(t *testing.T) {
	tests := []struct {
		name   string
		record record.Record
		expect []string
	}{
		{"Login", &record.Login{}, []string{"browser", "profile", "url", "username", "password", "created_at", "store"}},
		{"Cookie", &record.Cookie{}, []string{
			"browser", "profile", "host", "path", "name", "value",
			"is_secure", "is_http_only", "has_expire", "is_persistent", "expire_at", "created_at", "same_site",
		}},
		{"Bookmark", &record.Bookmark{}, []string{"browser", "profile", "id", "name", "type", "url", "folder", "created_at"}},
		{"History", &record.History{}, []string{"browser", "profile", "url", "title", "visit_count", "last_visit"}},
		{"Download", &record.Download{}, []string{
			"browser", "profile", "url", "target_path", "mime_type", "total_bytes", "start_time", "end_time",
		}},
		{"CreditCard", &record.CreditCard{}, []string{
			"browser", "profile", "guid", "name", "number", "exp_month", "exp_year", "nick_name", "address", "cvc", "comment",
		}},
		{"LocalStorage", &record.LocalStorage{}, []string{"browser", "profile", "is_meta", "url", "key", "value"}},
		{"SessionStorage", &record.SessionStorage{}, []string{"browser", "profile", "is_meta", "url", "key", "value"}},
		{"Extension", &record.Extension{}, []string{
			"browser", "profile", "name", "id", "description", "version", "homepage_url", "enabled",
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expect, structCSVHeader(testCase.record))
		})
	}
}

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
