package export

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var testTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

type extraction struct {
	c       category.Category
	records []record.Record
}

func chromeExtractions() []extraction {
	return []extraction{
		{category.Password, []record.Record{&record.Login{
			Browser: "Chrome", Profile: "Default",
			URL: "https://example.com", Username: "alice", Password: "secret", CreatedAt: testTime, Store: record.StoreAccount,
		}}},
		{category.Cookie, []record.Record{&record.Cookie{
			Browser: "Chrome", Profile: "Default",
			Host: ".example.com", Path: "/", Name: "session", Value: "abc123",
			IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: true,
			ExpireAt: testTime, CreatedAt: testTime,
		}}},
		{category.History, []record.Record{&record.History{
			Browser: "Chrome", Profile: "Default",
			URL: "https://example.com", Title: "Example", VisitCount: 5, LastVisit: testTime,
		}}},
	}
}

func firefoxExtractions() []extraction {
	return []extraction{
		{category.Password, []record.Record{&record.Login{
			Browser: "Firefox", Profile: "abc123",
			URL: "https://reddit.com", Username: "bob", Password: "hunter2", CreatedAt: testTime, Store: record.StoreLocal,
		}}},
		{category.Cookie, []record.Record{&record.Cookie{
			Browser: "Firefox", Profile: "abc123",
			Host: ".reddit.com", Path: "/", Name: "token", Value: "xyz789",
			IsSecure: true, IsHTTPOnly: false, ExpireAt: testTime, CreatedAt: testTime,
		}}},
	}
}

func writeAll(t *testing.T, dir string, f Format, extractions ...extraction) {
	t.Helper()
	w, err := NewWriter(dir, f, false)
	require.NoError(t, err)
	for _, e := range extractions {
		require.NoError(t, w.Write(e.c, e.records))
	}
	require.NoError(t, w.Close())
}

// --- Format ---

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in   string
		want Format
	}{
		{"json", JSON},
		{"csv", CSV},
		{"cookie-editor", CookieEditor},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.in)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
	_, err := ParseFormat("unknown")
	require.EqualError(t, err, "unsupported format: unknown")
}

func TestFormat_ZeroIsJSON(t *testing.T) {
	var f Format
	assert.Equal(t, JSON, f)
}

func TestNewWriter_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "results")
	_, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	assert.DirExists(t, dir)
}

// --- CSV output ---

func TestWrite_CSV_Password(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV, append(chromeExtractions(), firefoxExtractions()...)...)

	records := readCSV(t, filepath.Join(dir, "password.csv"))
	require.Len(t, records, 3) // header + 2 rows

	assert.Equal(t, []string{"browser", "profile", "url", "username", "password", "created_at", "store"}, records[0])
	assert.Equal(t, []string{"Chrome", "Default", "https://example.com", "alice", "secret", "2026-01-15T10:30:00Z", "account"}, records[1])
	assert.Equal(t, []string{"Firefox", "abc123", "https://reddit.com", "bob", "hunter2", "2026-01-15T10:30:00Z", "local"}, records[2])
}

func TestWrite_CSV_Cookie(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV, chromeExtractions()...)

	records := readCSV(t, filepath.Join(dir, "cookie.csv"))
	require.Len(t, records, 2)

	assert.Equal(t,
		[]string{
			"browser", "profile", "host", "path", "name", "value",
			"is_secure", "is_http_only", "has_expire", "is_persistent", "expire_at", "created_at", "same_site",
		},
		records[0],
	)
	assert.Equal(t,
		[]string{
			"Chrome", "Default", ".example.com", "/", "session", "abc123",
			"true", "true", "true", "true", "2026-01-15T10:30:00Z", "2026-01-15T10:30:00Z", "",
		},
		records[1],
	)
}

func TestWrite_CSV_History(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV, chromeExtractions()...)

	records := readCSV(t, filepath.Join(dir, "history.csv"))
	require.Len(t, records, 2)

	assert.Equal(t, []string{"browser", "profile", "url", "title", "visit_count", "last_visit"}, records[0])
	assert.Equal(t, []string{"Chrome", "Default", "https://example.com", "Example", "5", "2026-01-15T10:30:00Z"}, records[1])
}

func TestWrite_CSV_UTF8BOM(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV, append(chromeExtractions(), firefoxExtractions()...)...)

	raw, err := os.ReadFile(filepath.Join(dir, "password.csv"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(raw), 3)
	assert.Equal(t, utf8BOM, raw[:3], "CSV should start with UTF-8 BOM")
	assert.Equal(t, 1, bytes.Count(raw, utf8BOM), "one BOM however many writes")
}

// --- JSON output ---

func TestWrite_JSON_Password(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, JSON, append(chromeExtractions(), firefoxExtractions()...)...)

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
	writeAll(t, dir, JSON, chromeExtractions()...)

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
	writeAll(t, dir, JSON, chromeExtractions()...)

	raw, err := os.ReadFile(filepath.Join(dir, "password.json"))
	require.NoError(t, err)
	if len(raw) >= 3 {
		assert.NotEqual(t, utf8BOM, raw[:3], "JSON should NOT have BOM")
	}
}

// --- CookieEditor output ---

func TestWrite_CookieEditor(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CookieEditor, chromeExtractions()...)

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

func TestWrite_CookieEditor_FallbackJSON(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CookieEditor, extraction{category.Password, []record.Record{&record.Login{URL: "https://a.com"}}})

	// non-cookie categories fall back to standard JSON format
	_, err := os.Stat(filepath.Join(dir, "password.json"))
	assert.False(t, os.IsNotExist(err), "password.json should be created via JSON fallback")
}

// --- File creation ---

func TestWrite_EmptyCategoryNoFile(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV,
		extraction{category.Password, []record.Record{&record.Login{URL: "https://a.com"}}},
		extraction{category.Cookie, nil},
	)

	assert.FileExists(t, filepath.Join(dir, "password.csv"))
	_, err := os.Stat(filepath.Join(dir, "cookie.csv"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(dir, "history.csv"))
	assert.True(t, os.IsNotExist(err))
}

func TestWrite_NoData(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no files should be created when no data added")
}

// --- Appending extractions ---

func TestWrite_AppendsExtractionsInWriteOrder(t *testing.T) {
	dir := t.TempDir()
	older := &record.History{Browser: "Chrome", Profile: "Default", URL: "https://a.example", VisitCount: 1}
	busier := &record.History{Browser: "Chrome", Profile: "Profile 1", URL: "https://b.example", VisitCount: 99}
	writeAll(t, dir, JSON,
		extraction{category.History, []record.Record{older}},
		extraction{category.History, []record.Record{busier}},
	)

	var rows []record.History
	readJSON(t, filepath.Join(dir, "history.json"), &rows)
	assert.Equal(t, []record.History{*older, *busier}, rows, "each extraction is sorted on its own, then appended")
}

// --- Error paths ---

func TestWrite_RejectsRecordOfAnotherCategory(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	require.EqualError(t, w.Write(category.Cookie, []record.Record{&record.Login{}}), "*record.Login is not a cookie record")
	require.NoError(t, w.Close())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// A failed Write must not cost the files already written: Close still completes them.
func TestClose_AfterFailedWriteKeepsEarlierFiles(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	require.NoError(t, w.Write(category.Password, []record.Record{&record.Login{URL: "https://kept.example"}}))

	beyondJSON := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.Error(t, w.Write(category.Cookie, []record.Record{&record.Cookie{Host: "bad.example", ExpireAt: beyondJSON}}))
	require.NoError(t, w.Close())

	var logins []record.Login
	readJSON(t, filepath.Join(dir, "password.json"), &logins)
	require.Len(t, logins, 1)
	assert.Equal(t, "https://kept.example", logins[0].URL)

	var cookies []record.Cookie
	readJSON(t, filepath.Join(dir, "cookie.json"), &cookies)
	assert.Empty(t, cookies)
	assert.Zero(t, w.Output(), "a failed run lists nothing")
}

func TestClose_OutputListsTheFilesInCategoryOrder(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	require.NoError(t, w.Write(category.History, []record.Record{&record.History{URL: "https://a.example"}}))
	require.NoError(t, w.Write(category.Password, []record.Record{
		&record.Login{URL: "https://b.example"}, &record.Login{URL: "https://c.example"},
	}))
	require.NoError(t, w.Close())

	assert.Equal(t, Output{Dir: dir, Files: []File{{Name: "password.json", Count: 2}, {Name: "history.json", Count: 1}}}, w.Output())
}

// --- --zip ---

func TestClose_ZipReplacesTheFilesWithOneArchive(t *testing.T) {
	loose := filepath.Join(t.TempDir(), "results")
	writeAll(t, loose, CSV, chromeExtractions()...)

	dir := filepath.Join(t.TempDir(), "results")
	require.NoError(t, zipAll(dir, chromeExtractions()...))

	assert.Equal(t, []string{"results.zip"}, dirNames(t, dir))
	assert.Equal(t, dirFiles(t, loose), zipFiles(t, filepath.Join(dir, "results.zip")))
}

// The output is the files this run wrote; anything else in the directory is not the writer's to pack or delete.
func TestClose_ZipLeavesOtherFilesAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "results")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o600))

	require.NoError(t, zipAll(dir, chromeExtractions()...))
	assert.Equal(t, []string{"notes.txt", "results.zip"}, dirNames(t, dir))
	assert.NotContains(t, zipFiles(t, filepath.Join(dir, "results.zip")), "notes.txt")
}

func TestClose_ZipFailureKeepsTheFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "results")
	// A non-empty directory in the zip's place makes the final rename fail on every OS.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "results.zip", "taken"), 0o755))

	require.Error(t, zipAll(dir, chromeExtractions()...))
	assert.Equal(t, []string{"cookie.csv", "history.csv", "password.csv", "results.zip"}, dirNames(t, dir))
}

func TestClose_ZipOutputNamesTheArchive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "results")
	w, err := NewWriter(dir, CSV, true)
	require.NoError(t, err)
	require.NoError(t, w.Write(category.Password, []record.Record{&record.Login{URL: "https://a.example"}}))
	require.NoError(t, w.Close())

	assert.Equal(t, Output{Dir: dir, Files: []File{{Name: "password.csv", Count: 1}}, Zip: "results.zip"}, w.Output())
}

// The zip holds the same records as the category files, so it gets their owner-only mode.
func TestClose_ZipIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no mode bits to check")
	}
	dir := filepath.Join(t.TempDir(), "results")
	require.NoError(t, zipAll(dir, chromeExtractions()...))

	info, err := os.Stat(filepath.Join(dir, "results.zip"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestClose_ZipWithNothingWrittenFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "results")
	require.Error(t, zipAll(dir))
	assert.Empty(t, dirNames(t, dir))
}

// --- helpers ---

func zipAll(dir string, extractions ...extraction) error {
	w, err := NewWriter(dir, CSV, true)
	if err != nil {
		return err
	}
	for _, e := range extractions {
		if err := w.Write(e.c, e.records); err != nil {
			return errors.Join(err, w.Close())
		}
	}
	return w.Close()
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func dirFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	for _, name := range dirNames(t, dir) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		files[name] = string(data)
	}
	return files
}

func zipFiles(t *testing.T, path string) map[string]string {
	t.Helper()
	r, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer r.Close()
	files := make(map[string]string, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, errors.Join(err, rc.Close()))
		files[f.Name] = string(data)
	}
	return files
}

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
