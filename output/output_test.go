package output

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

var testTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

func chromeData() *types.BrowserData {
	return &types.BrowserData{
		Passwords: []types.LoginEntry{
			{URL: "https://example.com", Username: "alice", Password: "secret", CreatedAt: testTime},
		},
		Cookies: []types.CookieEntry{
			{
				Host: ".example.com", Path: "/", Name: "session", Value: "abc123",
				IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: true,
				ExpireAt: testTime, CreatedAt: testTime,
			},
		},
		Histories: []types.HistoryEntry{
			{URL: "https://example.com", Title: "Example", VisitCount: 5, LastVisit: testTime},
		},
	}
}

func firefoxData() *types.BrowserData {
	return &types.BrowserData{
		Passwords: []types.LoginEntry{
			{URL: "https://reddit.com", Username: "bob", Password: "hunter2", CreatedAt: testTime},
		},
		Cookies: []types.CookieEntry{
			{
				Host: ".reddit.com", Path: "/", Name: "token", Value: "xyz789",
				IsSecure: true, IsHTTPOnly: false, ExpireAt: testTime, CreatedAt: testTime,
			},
		},
	}
}

// --- New ---

func TestNew(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"csv", false},
		{"json", false},
		{"cookie-editor", false},
		{"unknown", true},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			out, err := NewWriter(t.TempDir(), tt.format)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, out)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, out)
			}
		})
	}
}

// --- CSV output ---

func TestWrite_CSV_Password(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "csv")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	out.Add("Firefox", "abc123", firefoxData())
	require.NoError(t, out.Write())

	records := readCSV(t, filepath.Join(dir, "password.csv"))
	require.Len(t, records, 3) // header + 2 rows

	assert.Equal(t, []string{"browser", "profile", "url", "username", "password", "created_at"}, records[0])
	assert.Equal(t, []string{"Chrome", "Default", "https://example.com", "alice", "secret", "2026-01-15T10:30:00Z"}, records[1])
	assert.Equal(t, []string{"Firefox", "abc123", "https://reddit.com", "bob", "hunter2", "2026-01-15T10:30:00Z"}, records[2])
}

func TestWrite_CSV_Cookie(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "csv")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	require.NoError(t, out.Write())

	records := readCSV(t, filepath.Join(dir, "cookie.csv"))
	require.Len(t, records, 2)

	assert.Equal(t,
		[]string{
			"browser", "profile", "host", "path", "name", "value",
			"is_secure", "is_http_only", "has_expire", "is_persistent", "expire_at", "created_at",
		},
		records[0],
	)
	assert.Equal(t,
		[]string{
			"Chrome", "Default", ".example.com", "/", "session", "abc123",
			"true", "true", "true", "true", "2026-01-15T10:30:00Z", "2026-01-15T10:30:00Z",
		},
		records[1],
	)
}

func TestWrite_CSV_History(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "csv")
	require.NoError(t, err)
	out.Add("Chrome", "Profile 1", chromeData())
	require.NoError(t, out.Write())

	records := readCSV(t, filepath.Join(dir, "history.csv"))
	require.Len(t, records, 2)

	assert.Equal(t, []string{"browser", "profile", "url", "title", "visit_count", "last_visit"}, records[0])
	assert.Equal(t, []string{"Chrome", "Profile 1", "https://example.com", "Example", "5", "2026-01-15T10:30:00Z"}, records[1])
}

func TestWrite_CSV_UTF8BOM(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "csv")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	require.NoError(t, out.Write())

	raw, err := os.ReadFile(filepath.Join(dir, "password.csv"))
	require.NoError(t, err)
	require.True(t, len(raw) >= 3)
	assert.Equal(t, utf8BOM, raw[:3], "CSV should start with UTF-8 BOM")
}

// --- JSON output ---

func TestWrite_JSON_Password(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "json")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	out.Add("Firefox", "abc123", firefoxData())
	require.NoError(t, out.Write())

	type pwJSON struct {
		Browser   string    `json:"browser"`
		Profile   string    `json:"profile"`
		URL       string    `json:"url"`
		Username  string    `json:"username"`
		Password  string    `json:"password"`
		CreatedAt time.Time `json:"created_at"`
	}
	var rows []pwJSON
	readJSON(t, filepath.Join(dir, "password.json"), &rows)
	require.Len(t, rows, 2)

	assert.Equal(t, pwJSON{
		Browser: "Chrome", Profile: "Default",
		URL: "https://example.com", Username: "alice", Password: "secret", CreatedAt: testTime,
	}, rows[0])
	assert.Equal(t, pwJSON{
		Browser: "Firefox", Profile: "abc123",
		URL: "https://reddit.com", Username: "bob", Password: "hunter2", CreatedAt: testTime,
	}, rows[1])
}

func TestWrite_JSON_Cookie(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "json")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	require.NoError(t, out.Write())

	type ckJSON struct {
		Browser      string    `json:"browser"`
		Profile      string    `json:"profile"`
		Host         string    `json:"host"`
		Path         string    `json:"path"`
		Name         string    `json:"name"`
		Value        string    `json:"value"`
		IsSecure     bool      `json:"is_secure"`
		IsHTTPOnly   bool      `json:"is_http_only"`
		HasExpire    bool      `json:"has_expire"`
		IsPersistent bool      `json:"is_persistent"`
		ExpireAt     time.Time `json:"expire_at"`
		CreatedAt    time.Time `json:"created_at"`
	}
	var rows []ckJSON
	readJSON(t, filepath.Join(dir, "cookie.json"), &rows)
	require.Len(t, rows, 1)

	assert.Equal(t, ckJSON{
		Browser: "Chrome", Profile: "Default",
		Host: ".example.com", Path: "/", Name: "session", Value: "abc123",
		IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: true,
		ExpireAt: testTime, CreatedAt: testTime,
	}, rows[0])
}

func TestWrite_JSON_NoBOM(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "json")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	require.NoError(t, out.Write())

	raw, err := os.ReadFile(filepath.Join(dir, "password.json"))
	require.NoError(t, err)
	if len(raw) >= 3 {
		assert.NotEqual(t, utf8BOM, raw[:3], "JSON should NOT have BOM")
	}
}

// --- CookieEditor output ---

func TestWrite_CookieEditor(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "cookie-editor")
	require.NoError(t, err)
	out.Add("Chrome", "Default", chromeData())
	require.NoError(t, out.Write())

	var entries []cookieEditorEntry
	readJSON(t, filepath.Join(dir, "cookie.json"), &entries)
	require.Len(t, entries, 1)

	assert.Equal(t, cookieEditorEntry{
		Domain:         ".example.com",
		Name:           "session",
		Value:          "abc123",
		Path:           "/",
		Secure:         true,
		HTTPOnly:       true,
		ExpirationDate: float64(testTime.Unix()),
	}, entries[0])
}

func TestWrite_CookieEditor_SkipsNonCookie(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "cookie-editor")
	require.NoError(t, err)
	out.Add("Chrome", "Default", &types.BrowserData{
		Passwords: []types.LoginEntry{{URL: "https://a.com"}},
	})
	require.NoError(t, out.Write())

	// password file should not be created (cookie-editor only exports cookies)
	_, err = os.Stat(filepath.Join(dir, "password.json"))
	assert.True(t, os.IsNotExist(err))
}

// --- File creation ---

func TestWrite_EmptyCategoryNoFile(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "csv")
	require.NoError(t, err)
	out.Add("Chrome", "Default", &types.BrowserData{
		Passwords: []types.LoginEntry{{URL: "https://a.com"}},
	})
	require.NoError(t, out.Write())

	assert.FileExists(t, filepath.Join(dir, "password.csv"))
	_, err = os.Stat(filepath.Join(dir, "cookie.csv"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(dir, "history.csv"))
	assert.True(t, os.IsNotExist(err))
}

func TestWrite_NoData(t *testing.T) {
	dir := t.TempDir()
	out, err := NewWriter(dir, "csv")
	require.NoError(t, err)
	require.NoError(t, out.Write())

	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "no files should be created when no data added")
}

// --- helpers ---

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	// Skip UTF-8 BOM if present
	content := string(raw)
	if strings.HasPrefix(content, string(utf8BOM)) {
		content = content[len(utf8BOM):]
	}
	reader := csv.NewReader(strings.NewReader(content))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	return records
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, v))
}
