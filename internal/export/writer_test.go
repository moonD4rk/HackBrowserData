package export

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

var testTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

type extraction struct {
	dataCategory category.Category
	records      []record.Record
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

func writeAll(t *testing.T, dir string, format Format, extractions ...extraction) {
	t.Helper()
	writer, err := NewWriter(dir, format, false)
	require.NoError(t, err)
	for _, batch := range extractions {
		require.NoError(t, writer.Write(batch.dataCategory, batch.records))
	}
	require.NoError(t, writer.Close())
}

func TestNewWriter_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "results")
	_, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	assert.DirExists(t, dir)
}

func TestWrite_NoData(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir, CSV)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "no files should be created when no data added")
}

func TestWrite_AppendsExtractionsInWriteOrder(t *testing.T) {
	dir := t.TempDir()
	older := &record.History{Browser: "Chrome", Profile: "Default", URL: "https://a.example", VisitCount: 1}
	busier := &record.History{Browser: "Chrome", Profile: "Profile 1", URL: "https://b.example", VisitCount: 99}
	quiet := &record.History{Browser: "Chrome", Profile: "Profile 1", URL: "https://c.example", VisitCount: 2}
	writeAll(t, dir, JSON,
		extraction{category.History, []record.Record{older}},
		extraction{category.History, []record.Record{quiet, busier}},
	)

	var rows []record.History
	readJSON(t, filepath.Join(dir, "history.json"), &rows)
	assert.Equal(t, []record.History{*older, *busier, *quiet}, rows, "each extraction is sorted on its own, then appended")
}

func TestClose_AfterFailedWriteKeepsEarlierFiles(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	require.NoError(t, writer.Write(category.Password, []record.Record{&record.Login{URL: "https://kept.example"}}))

	beyondJSON := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.Error(t, writer.Write(category.Cookie, []record.Record{&record.Cookie{Host: "bad.example", ExpireAt: beyondJSON}}))
	require.NoError(t, writer.Close())

	var logins []record.Login
	readJSON(t, filepath.Join(dir, "password.json"), &logins)
	require.Len(t, logins, 1)
	assert.Equal(t, "https://kept.example", logins[0].URL)

	var cookies []record.Cookie
	readJSON(t, filepath.Join(dir, "cookie.json"), &cookies)
	assert.Empty(t, cookies)
	assert.Zero(t, writer.Listing(), "a failed run lists nothing")
}

func TestClose_OutputListsTheFilesInCategoryOrder(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, JSON, false)
	require.NoError(t, err)
	require.NoError(t, writer.Write(category.History, []record.Record{&record.History{URL: "https://a.example"}}))
	require.NoError(t, writer.Write(category.Password, []record.Record{
		&record.Login{URL: "https://b.example"}, &record.Login{URL: "https://c.example"},
	}))
	require.NoError(t, writer.Close())

	assert.Equal(t, Listing{Dir: dir, Files: []File{{Name: "password.json", Count: 2}, {Name: "history.json", Count: 1}}}, writer.Listing())
}

// TestClose_ZipLeavesOtherFilesAlone packs only this run's files; the rest of the directory is not the writer's.
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
	writer, err := NewWriter(dir, CSV, true)
	require.NoError(t, err)
	require.NoError(t, writer.Write(category.Password, []record.Record{&record.Login{URL: "https://a.example"}}))
	require.NoError(t, writer.Close())

	assert.Equal(t, Listing{Dir: dir, Files: []File{{Name: "password.csv", Count: 1}}, ZipName: "results.zip"}, writer.Listing())
}

// TestClose_ZipIsOwnerOnly gives the zip the category files' owner-only mode, since it holds the same records.
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

func zipAll(dir string, extractions ...extraction) error {
	writer, err := NewWriter(dir, CSV, true)
	if err != nil {
		return err
	}
	for _, batch := range extractions {
		if err := writer.Write(batch.dataCategory, batch.records); err != nil {
			return errors.Join(err, writer.Close())
		}
	}
	return writer.Close()
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func zipFiles(t *testing.T, path string) map[string]string {
	t.Helper()
	zipReader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer zipReader.Close()
	files := make(map[string]string, len(zipReader.File))
	for _, zipFile := range zipReader.File {
		entryReader, err := zipFile.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(entryReader)
		require.NoError(t, errors.Join(err, entryReader.Close()))
		files[zipFile.Name] = string(data)
	}
	return files
}

func readJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, value))
}
