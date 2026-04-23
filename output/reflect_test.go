package output

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

var refTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

// allEntryTypes lists every entry type that appears in BrowserData.
// If a new entry type is added, it must be included here.
var allEntryTypes = []any{
	types.LoginEntry{},
	types.CookieEntry{},
	types.BookmarkEntry{},
	types.HistoryEntry{},
	types.DownloadEntry{},
	types.CreditCardEntry{},
	types.StorageEntry{},
	types.ExtensionEntry{},
}

// TestAllEntryFieldsHaveCSVTag verifies that every exported field
// in every entry type has a csv tag. A missing tag means the field
// will be silently omitted from CSV output.
func TestAllEntryFieldsHaveCSVTag(t *testing.T) {
	for _, entry := range allEntryTypes {
		et := reflect.TypeOf(entry)
		t.Run(et.Name(), func(t *testing.T) {
			for i := 0; i < et.NumField(); i++ {
				f := et.Field(i)
				if !f.IsExported() {
					continue
				}
				_, ok := f.Tag.Lookup("csv")
				assert.True(t, ok, "field %s.%s missing csv tag", et.Name(), f.Name)
			}
		})
	}
}

func TestStructCSVHeader(t *testing.T) {
	tests := []struct {
		name   string
		entry  any
		expect []string
	}{
		{"LoginEntry", types.LoginEntry{}, []string{"url", "username", "password", "created_at"}},
		{"CookieEntry", types.CookieEntry{}, []string{"host", "path", "name", "value", "is_secure", "is_http_only", "has_expire", "is_persistent", "expire_at", "created_at"}},
		{"BookmarkEntry", types.BookmarkEntry{}, []string{"id", "name", "type", "url", "folder", "created_at"}},
		{"HistoryEntry", types.HistoryEntry{}, []string{"url", "title", "visit_count", "last_visit"}},
		{"DownloadEntry", types.DownloadEntry{}, []string{"url", "target_path", "mime_type", "total_bytes", "start_time", "end_time"}},
		{"CreditCardEntry", types.CreditCardEntry{}, []string{"guid", "name", "number", "exp_month", "exp_year", "nick_name", "address", "cvc", "comment"}},
		{"StorageEntry", types.StorageEntry{}, []string{"is_meta", "url", "key", "value"}},
		{"ExtensionEntry", types.ExtensionEntry{}, []string{"name", "id", "description", "version", "homepage_url", "enabled"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, structCSVHeader(tt.entry))
		})
	}
}

func TestStructCSVRow(t *testing.T) {
	tests := []struct {
		name   string
		entry  any
		expect []string
	}{
		{
			"LoginEntry",
			types.LoginEntry{URL: "https://example.com", Username: "alice", Password: "secret", CreatedAt: refTime},
			[]string{"https://example.com", "alice", "secret", "2026-01-15T10:30:00Z"},
		},
		{
			"CookieEntry",
			types.CookieEntry{
				Host: ".example.com", Path: "/", Name: "session", Value: "abc",
				IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: false,
				ExpireAt: refTime, CreatedAt: refTime,
			},
			[]string{".example.com", "/", "session", "abc", "true", "true", "true", "false", "2026-01-15T10:30:00Z", "2026-01-15T10:30:00Z"},
		},
		{
			"HistoryEntry_int",
			types.HistoryEntry{URL: "https://a.com", Title: "A", VisitCount: 42, LastVisit: refTime},
			[]string{"https://a.com", "A", "42", "2026-01-15T10:30:00Z"},
		},
		{
			"DownloadEntry_int64",
			types.DownloadEntry{URL: "https://a.com", TargetPath: "/tmp/f", MimeType: "text/plain", TotalBytes: 1024, StartTime: refTime, EndTime: refTime},
			[]string{"https://a.com", "/tmp/f", "text/plain", "1024", "2026-01-15T10:30:00Z", "2026-01-15T10:30:00Z"},
		},
		{
			"ExtensionEntry_bool",
			types.ExtensionEntry{Name: "ext", ID: "abc", Description: "desc", Version: "1.0", HomepageURL: "https://x.com", Enabled: true},
			[]string{"ext", "abc", "desc", "1.0", "https://x.com", "true"},
		},
		{
			"zero_time",
			types.LoginEntry{URL: "https://a.com"},
			[]string{"https://a.com", "", "", ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, structCSVRow(tt.entry))
		})
	}
}

// TestRowMarshalJSON verifies that row.MarshalJSON produces flat JSON
// with browser/profile first, followed by entry fields in declaration order.
func TestRowMarshalJSON(t *testing.T) {
	t.Run("flat_structure", func(t *testing.T) {
		r := row{
			Browser: "Chrome",
			Profile: "Default",
			entry: types.LoginEntry{
				URL: "https://example.com", Username: "alice",
				Password: "secret", CreatedAt: refTime,
			},
		}

		data, err := json.Marshal(r)
		require.NoError(t, err)

		// Verify flat JSON (all keys at top level, no nesting).
		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))
		assert.Equal(t, "Chrome", m["browser"])
		assert.Equal(t, "Default", m["profile"])
		assert.Equal(t, "https://example.com", m["url"])
		assert.Equal(t, "alice", m["username"])
		assert.Equal(t, "secret", m["password"])
		assert.Len(t, m, 6) // browser + profile + 4 entry fields

		// Verify field order: browser, profile come before entry fields.
		raw := string(data)
		browserIdx := strings.Index(raw, `"browser"`)
		profileIdx := strings.Index(raw, `"profile"`)
		urlIdx := strings.Index(raw, `"url"`)
		assert.Less(t, browserIdx, urlIdx)
		assert.Less(t, profileIdx, urlIdx)
	})

	t.Run("bool_and_time_fields", func(t *testing.T) {
		r := row{
			Browser: "Firefox",
			Profile: "test",
			entry: types.CookieEntry{
				Host: ".example.com", IsSecure: true, IsHTTPOnly: false,
				ExpireAt: refTime,
			},
		}

		data, err := json.Marshal(r)
		require.NoError(t, err)

		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))
		assert.Equal(t, "Firefox", m["browser"])
		assert.Equal(t, ".example.com", m["host"])
		assert.Equal(t, true, m["is_secure"])
		assert.Equal(t, false, m["is_http_only"])
	})

	t.Run("special_characters", func(t *testing.T) {
		r := row{
			Browser: `Ch"rome`,
			Profile: "Default",
			entry: types.LoginEntry{
				URL:      `https://example.com/path?q="hello"&x=1`,
				Password: `pass"word\with<special>`,
			},
		}

		data, err := json.Marshal(r)
		require.NoError(t, err)

		var m map[string]any
		require.NoError(t, json.Unmarshal(data, &m))
		assert.Equal(t, `Ch"rome`, m["browser"])
		assert.Equal(t, `https://example.com/path?q="hello"&x=1`, m["url"])
		assert.Equal(t, `pass"word\with<special>`, m["password"])
	})
}
