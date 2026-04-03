package output

import (
	"bytes"
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

func testData() *types.BrowserData {
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

// --- NewFormatter ---

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"csv", false},
		{"json", false},
		{"cookie-editor", false},
		{"unknown", true},
	}
	for _, tt := range tests {
		f, err := NewFormatter(tt.name)
		if tt.wantErr {
			assert.Error(t, err)
			assert.Nil(t, f)
		} else {
			assert.NoError(t, err)
			assert.NotNil(t, f)
		}
	}
}

// --- CSVFormatter ---

func TestCSVFormatter(t *testing.T) {
	f := &CSVFormatter{}
	assert.Equal(t, "csv", f.Ext())

	cd := types.CategoryData{
		Category: types.Password,
		Data:     testData().Passwords,
		Len:      1,
	}

	var buf bytes.Buffer
	err := f.Format(&buf, cd, "Chrome", "Default")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2) // header + 1 row

	// Header has browser/profile prefix
	assert.True(t, strings.HasPrefix(lines[0], "browser,profile,"))
	assert.Contains(t, lines[0], "url,username,password")

	// Row has correct values
	assert.True(t, strings.HasPrefix(lines[1], "Chrome,Default,"))
	assert.Contains(t, lines[1], "https://example.com")
	assert.Contains(t, lines[1], "alice")
	assert.Contains(t, lines[1], "secret")
}

func TestCSVFormatter_HeaderOnlyOnce(t *testing.T) {
	f := &CSVFormatter{}
	cd := types.CategoryData{
		Category: types.Password,
		Data:     testData().Passwords,
		Len:      1,
	}

	var buf bytes.Buffer
	// Write twice (simulates two browsers appending to same file)
	require.NoError(t, f.Format(&buf, cd, "Chrome", "Default"))
	require.NoError(t, f.Format(&buf, cd, "Firefox", "abc123"))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 3) // 1 header + 2 rows

	// First line is header
	assert.Contains(t, lines[0], "browser,profile")
	// Second and third are data rows
	assert.True(t, strings.HasPrefix(lines[1], "Chrome,Default,"))
	assert.True(t, strings.HasPrefix(lines[2], "Firefox,abc123,"))
}

// --- JSONFormatter ---

func TestJSONFormatter(t *testing.T) {
	f := &JSONFormatter{}
	assert.Equal(t, "json", f.Ext())

	cd := types.CategoryData{
		Category: types.Password,
		Data:     testData().Passwords,
		Len:      1,
	}

	var buf bytes.Buffer
	err := f.Format(&buf, cd, "Chrome", "Default")
	require.NoError(t, err)

	// Verify it's valid JSON
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "Chrome", result["browser"])
	assert.Equal(t, "Default", result["profile"])
	assert.Equal(t, "password", result["category"])

	data, ok := result["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 1)

	entry := data[0].(map[string]interface{})
	assert.Equal(t, "https://example.com", entry["url"])
	assert.Equal(t, "alice", entry["username"])
}

// --- CookieEditorFormatter ---

func TestCookieEditorFormatter(t *testing.T) {
	f := &CookieEditorFormatter{}
	assert.Equal(t, "json", f.Ext())

	cd := types.CategoryData{
		Category: types.Cookie,
		Data:     testData().Cookies,
		Len:      1,
	}

	var buf bytes.Buffer
	err := f.Format(&buf, cd, "Chrome", "Default")
	require.NoError(t, err)

	// Verify CookieEditor JSON format
	var entries []cookieEditorEntry
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entries))
	require.Len(t, entries, 1)

	assert.Equal(t, ".example.com", entries[0].Domain)
	assert.Equal(t, "session", entries[0].Name)
	assert.Equal(t, "abc123", entries[0].Value)
	assert.Equal(t, "/", entries[0].Path)
	assert.True(t, entries[0].Secure)
	assert.True(t, entries[0].HTTPOnly)
	assert.Equal(t, float64(testTime.Unix()), entries[0].ExpirationDate)
}

func TestCookieEditorFormatter_SkipsNonCookie(t *testing.T) {
	f := &CookieEditorFormatter{}
	cd := types.CategoryData{
		Category: types.Password,
		Data:     testData().Passwords,
		Len:      1,
	}

	var buf bytes.Buffer
	err := f.Format(&buf, cd, "Chrome", "Default")
	require.NoError(t, err)
	assert.Empty(t, buf.String()) // non-cookie silently skipped
}

// --- Write (integration) ---

func TestWrite_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFormatter("csv")

	Write(testData(), "Chrome", "Default", dir, f)

	// Should create password.csv, cookie.csv, history.csv
	assert.FileExists(t, filepath.Join(dir, "password.csv"))
	assert.FileExists(t, filepath.Join(dir, "cookie.csv"))
	assert.FileExists(t, filepath.Join(dir, "history.csv"))

	// Empty categories should NOT create files
	_, err := os.Stat(filepath.Join(dir, "bookmark.csv"))
	assert.True(t, os.IsNotExist(err))
}

func TestWrite_UTF8BOM(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFormatter("csv")

	Write(testData(), "Chrome", "Default", dir, f)

	// Read the raw bytes of password.csv
	raw, err := os.ReadFile(filepath.Join(dir, "password.csv"))
	require.NoError(t, err)

	// First 3 bytes should be UTF-8 BOM
	require.True(t, len(raw) >= 3, "file too short for BOM")
	assert.Equal(t, utf8BOM, raw[:3], "CSV file should start with UTF-8 BOM")
}

func TestWrite_JSONNoBOM(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFormatter("json")

	Write(testData(), "Chrome", "Default", dir, f)

	raw, err := os.ReadFile(filepath.Join(dir, "password.json"))
	require.NoError(t, err)

	// JSON files should NOT have BOM
	if len(raw) >= 3 {
		assert.NotEqual(t, utf8BOM, raw[:3], "JSON file should NOT have UTF-8 BOM")
	}
}

func TestWrite_AppendMultipleBrowsers(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFormatter("csv")

	data1 := &types.BrowserData{
		Passwords: []types.LoginEntry{
			{URL: "https://a.com", Username: "alice"},
		},
	}
	data2 := &types.BrowserData{
		Passwords: []types.LoginEntry{
			{URL: "https://b.com", Username: "bob"},
		},
	}

	Write(data1, "Chrome", "Default", dir, f)
	Write(data2, "Firefox", "abc123", dir, f)

	raw, err := os.ReadFile(filepath.Join(dir, "password.csv"))
	require.NoError(t, err)

	content := string(raw)
	// Should have both browsers in one file
	assert.Contains(t, content, "Chrome,Default")
	assert.Contains(t, content, "Firefox,abc123")
	assert.Contains(t, content, "alice")
	assert.Contains(t, content, "bob")

	// Header should appear only once
	assert.Equal(t, 1, strings.Count(content, "browser,profile,url"))
}

// --- BrowserData.Each ---

func TestBrowserDataEach(t *testing.T) {
	data := testData() // has Passwords, Cookies, Histories

	var categories []types.Category
	data.Each(func(cd types.CategoryData) {
		categories = append(categories, cd.Category)
		assert.Greater(t, cd.Len, 0)
	})

	assert.Len(t, categories, 3)
	assert.Contains(t, categories, types.Password)
	assert.Contains(t, categories, types.Cookie)
	assert.Contains(t, categories, types.History)
}

func TestBrowserDataEach_Empty(t *testing.T) {
	data := &types.BrowserData{}

	called := false
	data.Each(func(cd types.CategoryData) {
		called = true
	})
	assert.False(t, called)
}
