package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/types"
)

const (
	exportUTF8BOM                 = "\xEF\xBB\xBF"
	exportURLWithHTMLChars        = "https://shop.example.com/cart?item=42&qty=<3>"
	exportURLWithHTMLCharsEscaped = `https://shop.example.com/cart?item=42\u0026qty=\u003c3\u003e`
)

type exportStamp struct {
	browser string
	profile string
	data    *types.BrowserData
}

type exportURLForm struct {
	file    string
	escaped bool
}

func TestExportGolden(t *testing.T) {
	formats := []struct {
		format       string
		ext          string
		urlForms     []exportURLForm
		jsonFallback bool
	}{
		{
			format:   "csv",
			ext:      "csv",
			urlForms: []exportURLForm{{file: "cookie.csv"}, {file: "history.csv"}},
		},
		{
			format:   "json",
			ext:      "json",
			urlForms: []exportURLForm{{file: "cookie.json", escaped: true}, {file: "history.json", escaped: true}},
		},
		{
			format:       "cookie-editor",
			ext:          "json",
			urlForms:     []exportURLForm{{file: "cookie.json"}, {file: "history.json", escaped: true}},
			jsonFallback: true,
		},
	}
	for _, tc := range formats {
		tc := tc
		t.Run(tc.format, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "results")
			exportWrite(t, dir, tc.format, false, exportStamps())
			got := bundleDir(t, dir)
			if tc.jsonFallback {
				jsonDir := filepath.Join(t.TempDir(), "results")
				exportWrite(t, jsonDir, "json", false, exportStamps())
				assertJSONFallback(t, dir, jsonDir)
				got = bundleFiles(t, dir, "cookie.json")
			}
			assertGolden(t, "export-"+tc.format+".golden", got)
			assert.Equal(t, exportCategoryFiles(tc.ext, category.All()...), exportDirFiles(t, dir))
			exportAssertFileContract(t, dir)
			for _, form := range tc.urlForms {
				exportAssertURLForm(t, dir, form)
			}

			sparse := filepath.Join(t.TempDir(), "sparse")
			exportWrite(t, sparse, tc.format, false, exportSparseStamps())
			assert.Equal(t, exportCategoryFiles(tc.ext, category.Password, category.LocalStorage), exportDirFiles(t, sparse))
		})
	}

	t.Run("zip", func(t *testing.T) {
		loose := filepath.Join(t.TempDir(), "results")
		exportWrite(t, loose, "csv", false, exportStamps())
		dir := filepath.Join(t.TempDir(), "results")
		exportWrite(t, dir, "csv", true, exportStamps())

		assert.Equal(t, []string{"results.zip"}, exportDirFiles(t, dir))
		archive := filepath.Join(dir, "results.zip")
		assertGolden(t, "export-zip.golden", zipListing(t, archive))
		assert.Equal(t, readDirFiles(t, loose), exportReadZip(t, archive))
	})
}

func exportWrite(t *testing.T, dir, format string, compress bool, stamps []exportStamp) {
	t.Helper()
	f, err := export.ParseFormat(format)
	require.NoError(t, err)
	w, err := export.NewWriter(dir, f, compress)
	require.NoError(t, err)
	for _, s := range stamps {
		require.NoError(t, writeResult(w, s.browser, types.ExtractResult{Profile: types.Profile{Name: s.profile}, Data: s.data}))
	}
	require.NoError(t, w.Close())
}

func exportCategoryFiles(ext string, categories ...category.Category) []string {
	names := make([]string, 0, len(categories))
	for _, c := range categories {
		names = append(names, c.String()+"."+ext)
	}
	sort.Strings(names)
	return names
}

func exportDirFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func exportReadFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(data)
}

func exportReadZip(t *testing.T, path string) map[string]string {
	t.Helper()
	r, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer r.Close()
	entries := make(map[string]string, len(r.File))
	for _, f := range r.File {
		entries[f.Name] = exportReadZipEntry(t, f)
	}
	return entries
}

func exportReadZipEntry(t *testing.T, f *zip.File) string {
	t.Helper()
	rc, err := f.Open()
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(data)
}

func exportAssertFileContract(t *testing.T, dir string) {
	t.Helper()
	for _, name := range exportDirFiles(t, dir) {
		content := exportReadFile(t, dir, name)
		switch filepath.Ext(name) {
		case ".csv":
			assert.Truef(t, strings.HasPrefix(content, exportUTF8BOM), "%s must start with the UTF-8 BOM", name)
		case ".json":
			assert.Falsef(t, strings.HasPrefix(content, exportUTF8BOM), "%s must not start with a BOM", name)
		default:
			t.Errorf("unexpected output file %s", name)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(filepath.Join(dir, name))
			require.NoError(t, err)
			assert.Equalf(t, os.FileMode(0o600), info.Mode().Perm(), "%s permissions", name)
		}
	}
}

func exportAssertURLForm(t *testing.T, dir string, form exportURLForm) {
	t.Helper()
	present, absent := exportURLWithHTMLChars, exportURLWithHTMLCharsEscaped
	if form.escaped {
		present, absent = absent, present
	}
	content := exportReadFile(t, dir, form.file)
	assert.Containsf(t, content, present, "%s", form.file)
	assert.NotContainsf(t, content, absent, "%s", form.file)
}

// Entries are listed in extractor sort order with unique sort keys, so re-sorting a profile cannot change a golden.
func exportStamps() []exportStamp {
	return []exportStamp{
		{browser: "Chrome", profile: "Default", data: &types.BrowserData{
			Passwords: []record.Login{
				{
					URL: "https://accounts.example.com/signin?continue=/mail&hl=ja", Username: "alice@example.com",
					Password:  "pa$$ \"quoted\", comma\\backslash\nnewline",
					CreatedAt: time.Date(2026, 3, 14, 15, 9, 26, 535897932, time.UTC), Store: record.StoreAccount,
				},
				{
					URL: "https://例え.jp/ログイン", Username: "日本語ユーザー", Password: "🔑 emoji",
					CreatedAt: time.Date(2025, 11, 5, 6, 7, 8, 0, time.UTC), Store: record.StoreLocal,
				},
			},
			Cookies: []record.Cookie{
				{
					Host: ".example.com", Path: "/", Name: "return_to", Value: exportURLWithHTMLChars,
					IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: true,
					ExpireAt:  time.Date(2027, 6, 30, 12, 34, 56, 789000000, time.UTC),
					CreatedAt: time.Date(2026, 3, 14, 15, 9, 27, 100000000, time.UTC), SameSite: "lax",
				},
				{
					Host: "www.example.com", Path: "/account", Name: "session_id", Value: `{"user":"日本語 🍪","dir":"C:\Temp"}`,
					IsHTTPOnly: true, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), SameSite: "strict",
				},
			},
			Histories: []record.History{
				{
					URL: exportURLWithHTMLChars, Title: `Cart "deals" <b>&</b>, 日本語 🛒`, VisitCount: 42,
					LastVisit: time.Date(2026, 3, 14, 15, 10, 0, 1000, time.UTC),
				},
				{URL: "file:///C:/Users/Alice/notes.html", Title: "multi\nline title", VisitCount: 3},
			},
			Downloads: []record.Download{
				{
					URL:        "https://dl.example.com/files/big.iso?token=a&sig=<x>",
					TargetPath: `C:\Users\Alice\Downloads\Ubuntu "final", v2.iso`,
					MimeType:   "application/x-iso9660-image", TotalBytes: 5368709121,
					StartTime: time.Date(2026, 3, 14, 15, 0, 0, 250000000, time.UTC),
					EndTime:   time.Date(2026, 3, 14, 15, 30, 0, 750000000, time.UTC),
				},
			},
			Bookmarks: []record.Bookmark{
				{
					ID: 7, Name: "Docs & <Specs>", Type: "url", URL: "https://docs.example.com/search?a=1&b=<2>", Folder: "Bookmarks bar",
					CreatedAt: time.Date(2026, 2, 2, 2, 2, 2, 20202020, time.UTC),
				},
				{ID: 4294967298, Name: "日本語 📚", Type: "url", URL: "https://例え.jp/", Folder: "Other bookmarks"},
			},
			CreditCards: []record.CreditCard{
				{
					GUID: "00000000-0000-4000-8000-000000000001", Name: "山田 太郎", Number: "4111111111111111",
					ExpMonth: "12", ExpYear: "2030", NickName: `Work "primary", 💳`, Address: "1-2-3 Chiyoda,\nTokyo 100-0001",
				},
			},
			Extensions: []record.Extension{
				{
					Name: "Ad Blocker", ID: "abcdefghijklmnopabcdefghijklmnop", Description: `Blocks ads & trackers, "fast" <b>`,
					Version: "1.58.0", HomepageURL: "https://adblock.example/?a=1&b=<2>", Enabled: true,
				},
			},
			LocalStorage: []record.LocalStorage{
				{URL: "https://app.example.com", Key: "prefs", Value: `{"theme":"dark","greeting":"こんにちは 👋","dir":"C:\Temp"}`},
				{IsMeta: true, URL: "https://app.example.com", Value: "meta data, value bytes is [8 128 1]"},
			},
			SessionStorage: []record.SessionStorage{
				{URL: "https://app.example.com", Key: "tab-id", Value: "42"},
			},
		}},
		{browser: "Chrome", profile: "Profile 1", data: &types.BrowserData{
			Passwords: []record.Login{
				{
					URL: "http://192.168.0.1:8080/admin", Username: "admin", Password: " leading space",
					CreatedAt: time.Date(2025, 6, 1, 0, 0, 0, 1, time.UTC), Store: record.StoreLocal,
				},
				{URL: "android://hash@com.example.app/", Store: record.StoreLocal},
			},
			Cookies: []record.Cookie{
				{
					Host: ".tracker.example", Path: "/", Name: "_ga", Value: "GA1.2.1234567890.1700000000",
					IsSecure: true, HasExpire: true, IsPersistent: true,
					ExpireAt:  time.Date(2028, 2, 29, 23, 59, 59, 0, time.UTC),
					CreatedAt: time.Date(2025, 5, 5, 5, 5, 5, 123456789, time.UTC), SameSite: "none",
				},
				{Host: ".dev.example", Path: "/dev", Name: "multi_line", Value: "line one\nline two, with comma", SameSite: "unspecified"},
			},
			Histories: []record.History{
				{
					URL: "https://search.example.com/?q=a%26b&lang=日本語", VisitCount: 1000000,
					LastVisit: time.Date(2025, 7, 7, 7, 7, 7, 700000000, time.UTC),
				},
				{URL: "about:blank", Title: " leading space", LastVisit: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			Downloads: []record.Download{
				{
					URL: "https://例え.jp/資料.pdf", TargetPath: "/home/alice/Downloads/資料 📄.pdf", MimeType: "application/pdf",
					StartTime: time.Date(2025, 8, 1, 8, 0, 0, 0, time.UTC),
				},
			},
			Bookmarks: []record.Bookmark{
				{
					ID: 1, Name: `Quote "this", please`, Type: "url", URL: "https://example.org/a,b", Folder: "multi\nline folder",
					CreatedAt: time.Date(2025, 3, 3, 3, 3, 3, 0, time.UTC),
				},
			},
			CreditCards: []record.CreditCard{
				{
					GUID: "00000000-0000-4000-8000-000000000002", Name: "Alice O'Brien", Number: "5555555555554444",
					ExpMonth: "1", ExpYear: "2027", CVC: "123", Comment: `note <b>&</b> \ end`,
				},
			},
			Extensions: []record.Extension{
				{Name: "日本語拡張 🧩", ID: "ponmlkjihgfedcbaponmlkjihgfedcba", Description: "line one\nline two", Version: "0.0.1"},
			},
			LocalStorage: []record.LocalStorage{
				{URL: "https://example.org", Key: "a&b<c>", Value: "multi\nline"},
			},
			SessionStorage: []record.SessionStorage{
				{URL: "https://example.org/?x=1&y=<2>", Key: "draft", Value: `He said "hi", then left \`},
			},
		}},
		{browser: "Microsoft Edge", profile: "Default", data: &types.BrowserData{
			Passwords: []record.Login{
				{
					URL: "https://login.example.org/<tenant>/authorize?client=a&scope=b", Username: `CORP\jdoe`, Password: "日本語パスワード",
					CreatedAt: time.Date(2024, 2, 29, 12, 0, 0, 500000000, time.UTC), Store: record.StoreAccount,
				},
			},
			Cookies: []record.Cookie{
				{
					Host: ".edge.example", Path: "/", Name: "epoch_expiry", Value: `back\slash "quoted" <b>&amp;</b>`,
					IsSecure: true, IsHTTPOnly: true, HasExpire: true, IsPersistent: true,
					ExpireAt: time.Unix(0, 0).UTC(), CreatedAt: time.Date(2024, 12, 24, 18, 0, 0, 0, time.UTC),
				},
				{
					Host: "edge.example", Path: "/", Name: "host_only", Value: "v", HasExpire: true, IsPersistent: true,
					ExpireAt:  time.Date(2026, 12, 31, 23, 59, 59, 999999999, time.UTC),
					CreatedAt: time.Date(2024, 6, 15, 9, 30, 0, 0, time.UTC), SameSite: "strict",
				},
			},
			Histories: []record.History{
				{
					URL: "edge://settings/?a=1&b=2", Title: `C:\Windows\System32`, VisitCount: 7,
					LastVisit: time.Date(2024, 7, 4, 0, 0, 0, 500000000, time.UTC),
				},
			},
			Downloads: []record.Download{
				{URL: "blob:https://app.example/1234", TargetPath: `\\fileserver\share\Q3 summary.csv`, TotalBytes: 4294967296},
			},
			Bookmarks: []record.Bookmark{
				{
					ID: 12, Name: `C:\Shares\docs`, Type: "url", URL: "https://edge.example/", Folder: "Favorites bar",
					CreatedAt: time.Date(2024, 4, 4, 4, 4, 4, 400000000, time.UTC),
				},
			},
			CreditCards: []record.CreditCard{
				{GUID: "00000000-0000-4000-8000-000000000003"},
			},
			Extensions: []record.Extension{
				{
					Name: "Edge Helper", ID: "aaaabbbbccccddddeeeeffffgggghhhh", Description: `Installed under C:\Program Files\Helper`,
					Version: "2.0", HomepageURL: "https://helper.example/", Enabled: true,
				},
			},
			LocalStorage: []record.LocalStorage{
				{URL: "https://edge.example", Key: "日本語"},
			},
			SessionStorage: []record.SessionStorage{
				{IsMeta: true, URL: "https://edge.example", Value: "meta data, value bytes is [16 2]"},
			},
		}},
	}
}

func exportSparseStamps() []exportStamp {
	return []exportStamp{
		{browser: "Chrome", profile: "Default", data: &types.BrowserData{
			Passwords: []record.Login{{URL: "https://example.com/", Username: "alice", Store: record.StoreLocal}},
			Cookies:   []record.Cookie{},
		}},
		{browser: "Chrome", profile: "Profile 1"},
		{browser: "Microsoft Edge", profile: "Default", data: &types.BrowserData{
			Histories:    []record.History{},
			LocalStorage: []record.LocalStorage{{URL: "https://example.com", Key: "k", Value: "v"}},
		}},
	}
}

// writeResult writes the export fixtures, which are kept in today's BrowserData until the goldens move into export.
func writeResult(w *export.Writer, browserName string, r types.ExtractResult) error {
	if r.Data == nil {
		return nil
	}
	for _, c := range category.All() {
		records := profileRecords(r.Data, c)
		for _, rec := range records {
			record.Stamp(rec, browserName, r.Name)
		}
		if err := w.Write(c, records); err != nil {
			return err
		}
	}
	return nil
}

func profileRecords(d *types.BrowserData, c category.Category) []record.Record {
	switch c {
	case category.Password:
		return recordsOf(d.Passwords)
	case category.Cookie:
		return recordsOf(d.Cookies)
	case category.Bookmark:
		return recordsOf(d.Bookmarks)
	case category.History:
		return recordsOf(d.Histories)
	case category.Download:
		return recordsOf(d.Downloads)
	case category.CreditCard:
		return recordsOf(d.CreditCards)
	case category.Extension:
		return recordsOf(d.Extensions)
	case category.LocalStorage:
		return recordsOf(d.LocalStorage)
	case category.SessionStorage:
		return recordsOf(d.SessionStorage)
	default:
		return nil
	}
}

func recordsOf[T any, P interface {
	*T
	record.Record
}](items []T) []record.Record {
	records := make([]record.Record, len(items))
	for i := range items {
		records[i] = P(&items[i])
	}
	return records
}
