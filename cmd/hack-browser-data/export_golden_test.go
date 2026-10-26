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

	"github.com/moond4rk/hackbrowserdata/output"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
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
			exportWrite(t, dir, tc.format, exportStamps())
			got := bundleDir(t, dir)
			if tc.jsonFallback {
				jsonDir := filepath.Join(t.TempDir(), "results")
				exportWrite(t, jsonDir, "json", exportStamps())
				assertJSONFallback(t, dir, jsonDir)
				got = bundleFiles(t, dir, "cookie.json")
			}
			assertGolden(t, "export-"+tc.format+".golden", got)
			assert.Equal(t, exportCategoryFiles(tc.ext, types.AllCategories...), exportDirFiles(t, dir))
			exportAssertFileContract(t, dir)
			for _, form := range tc.urlForms {
				exportAssertURLForm(t, dir, form)
			}

			sparse := filepath.Join(t.TempDir(), "sparse")
			exportWrite(t, sparse, tc.format, exportSparseStamps())
			assert.Equal(t, exportCategoryFiles(tc.ext, types.Password, types.LocalStorage), exportDirFiles(t, sparse))
		})
	}

	t.Run("zip", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "results")
		exportWrite(t, dir, "csv", exportStamps())
		loose := readDirFiles(t, dir)
		require.NoError(t, fileutil.CompressDir(dir))

		assert.Equal(t, []string{"results.zip"}, exportDirFiles(t, dir))
		archive := filepath.Join(dir, "results.zip")
		assertGolden(t, "export-zip.golden", zipListing(t, archive))
		assert.Equal(t, loose, exportReadZip(t, archive))
	})
}

func exportWrite(t *testing.T, dir, format string, stamps []exportStamp) {
	t.Helper()
	w, err := output.NewWriter(dir, format)
	require.NoError(t, err)
	for _, s := range stamps {
		w.Add(s.browser, s.profile, s.data)
	}
	require.NoError(t, w.Write())
}

func exportCategoryFiles(ext string, categories ...types.Category) []string {
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
			Passwords: []types.LoginEntry{
				{
					URL: "https://accounts.example.com/signin?continue=/mail&hl=ja", Username: "alice@example.com",
					Password:  "pa$$ \"quoted\", comma\\backslash\nnewline",
					CreatedAt: time.Date(2026, 3, 14, 15, 9, 26, 535897932, time.UTC), Store: types.PasswordStoreAccount,
				},
				{
					URL: "https://例え.jp/ログイン", Username: "日本語ユーザー", Password: "🔑 emoji",
					CreatedAt: time.Date(2025, 11, 5, 6, 7, 8, 0, time.UTC), Store: types.PasswordStoreLocal,
				},
			},
			Cookies: []types.CookieEntry{
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
			Histories: []types.HistoryEntry{
				{
					URL: exportURLWithHTMLChars, Title: `Cart "deals" <b>&</b>, 日本語 🛒`, VisitCount: 42,
					LastVisit: time.Date(2026, 3, 14, 15, 10, 0, 1000, time.UTC),
				},
				{URL: "file:///C:/Users/Alice/notes.html", Title: "multi\nline title", VisitCount: 3},
			},
			Downloads: []types.DownloadEntry{
				{
					URL:        "https://dl.example.com/files/big.iso?token=a&sig=<x>",
					TargetPath: `C:\Users\Alice\Downloads\Ubuntu "final", v2.iso`,
					MimeType:   "application/x-iso9660-image", TotalBytes: 5368709121,
					StartTime: time.Date(2026, 3, 14, 15, 0, 0, 250000000, time.UTC),
					EndTime:   time.Date(2026, 3, 14, 15, 30, 0, 750000000, time.UTC),
				},
			},
			Bookmarks: []types.BookmarkEntry{
				{
					ID: 7, Name: "Docs & <Specs>", Type: "url", URL: "https://docs.example.com/search?a=1&b=<2>", Folder: "Bookmarks bar",
					CreatedAt: time.Date(2026, 2, 2, 2, 2, 2, 20202020, time.UTC),
				},
				{ID: 4294967298, Name: "日本語 📚", Type: "url", URL: "https://例え.jp/", Folder: "Other bookmarks"},
			},
			CreditCards: []types.CreditCardEntry{
				{
					GUID: "00000000-0000-4000-8000-000000000001", Name: "山田 太郎", Number: "4111111111111111",
					ExpMonth: "12", ExpYear: "2030", NickName: `Work "primary", 💳`, Address: "1-2-3 Chiyoda,\nTokyo 100-0001",
				},
			},
			Extensions: []types.ExtensionEntry{
				{
					Name: "Ad Blocker", ID: "abcdefghijklmnopabcdefghijklmnop", Description: `Blocks ads & trackers, "fast" <b>`,
					Version: "1.58.0", HomepageURL: "https://adblock.example/?a=1&b=<2>", Enabled: true,
				},
			},
			LocalStorage: []types.StorageEntry{
				{URL: "https://app.example.com", Key: "prefs", Value: `{"theme":"dark","greeting":"こんにちは 👋","dir":"C:\Temp"}`},
				{IsMeta: true, URL: "https://app.example.com", Value: "meta data, value bytes is [8 128 1]"},
			},
			SessionStorage: []types.StorageEntry{
				{URL: "https://app.example.com", Key: "tab-id", Value: "42"},
			},
		}},
		{browser: "Chrome", profile: "Profile 1", data: &types.BrowserData{
			Passwords: []types.LoginEntry{
				{
					URL: "http://192.168.0.1:8080/admin", Username: "admin", Password: " leading space",
					CreatedAt: time.Date(2025, 6, 1, 0, 0, 0, 1, time.UTC), Store: types.PasswordStoreLocal,
				},
				{URL: "android://hash@com.example.app/", Store: types.PasswordStoreLocal},
			},
			Cookies: []types.CookieEntry{
				{
					Host: ".tracker.example", Path: "/", Name: "_ga", Value: "GA1.2.1234567890.1700000000",
					IsSecure: true, HasExpire: true, IsPersistent: true,
					ExpireAt:  time.Date(2028, 2, 29, 23, 59, 59, 0, time.UTC),
					CreatedAt: time.Date(2025, 5, 5, 5, 5, 5, 123456789, time.UTC), SameSite: "none",
				},
				{Host: ".dev.example", Path: "/dev", Name: "multi_line", Value: "line one\nline two, with comma", SameSite: "unspecified"},
			},
			Histories: []types.HistoryEntry{
				{
					URL: "https://search.example.com/?q=a%26b&lang=日本語", VisitCount: 1000000,
					LastVisit: time.Date(2025, 7, 7, 7, 7, 7, 700000000, time.UTC),
				},
				{URL: "about:blank", Title: " leading space", LastVisit: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			Downloads: []types.DownloadEntry{
				{
					URL: "https://例え.jp/資料.pdf", TargetPath: "/home/alice/Downloads/資料 📄.pdf", MimeType: "application/pdf",
					StartTime: time.Date(2025, 8, 1, 8, 0, 0, 0, time.UTC),
				},
			},
			Bookmarks: []types.BookmarkEntry{
				{
					ID: 1, Name: `Quote "this", please`, Type: "url", URL: "https://example.org/a,b", Folder: "multi\nline folder",
					CreatedAt: time.Date(2025, 3, 3, 3, 3, 3, 0, time.UTC),
				},
			},
			CreditCards: []types.CreditCardEntry{
				{
					GUID: "00000000-0000-4000-8000-000000000002", Name: "Alice O'Brien", Number: "5555555555554444",
					ExpMonth: "1", ExpYear: "2027", CVC: "123", Comment: `note <b>&</b> \ end`,
				},
			},
			Extensions: []types.ExtensionEntry{
				{Name: "日本語拡張 🧩", ID: "ponmlkjihgfedcbaponmlkjihgfedcba", Description: "line one\nline two", Version: "0.0.1"},
			},
			LocalStorage: []types.StorageEntry{
				{URL: "https://example.org", Key: "a&b<c>", Value: "multi\nline"},
			},
			SessionStorage: []types.StorageEntry{
				{URL: "https://example.org/?x=1&y=<2>", Key: "draft", Value: `He said "hi", then left \`},
			},
		}},
		{browser: "Microsoft Edge", profile: "Default", data: &types.BrowserData{
			Passwords: []types.LoginEntry{
				{
					URL: "https://login.example.org/<tenant>/authorize?client=a&scope=b", Username: `CORP\jdoe`, Password: "日本語パスワード",
					CreatedAt: time.Date(2024, 2, 29, 12, 0, 0, 500000000, time.UTC), Store: types.PasswordStoreAccount,
				},
			},
			Cookies: []types.CookieEntry{
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
			Histories: []types.HistoryEntry{
				{
					URL: "edge://settings/?a=1&b=2", Title: `C:\Windows\System32`, VisitCount: 7,
					LastVisit: time.Date(2024, 7, 4, 0, 0, 0, 500000000, time.UTC),
				},
			},
			Downloads: []types.DownloadEntry{
				{URL: "blob:https://app.example/1234", TargetPath: `\\fileserver\share\Q3 summary.csv`, TotalBytes: 4294967296},
			},
			Bookmarks: []types.BookmarkEntry{
				{
					ID: 12, Name: `C:\Shares\docs`, Type: "url", URL: "https://edge.example/", Folder: "Favorites bar",
					CreatedAt: time.Date(2024, 4, 4, 4, 4, 4, 400000000, time.UTC),
				},
			},
			CreditCards: []types.CreditCardEntry{
				{GUID: "00000000-0000-4000-8000-000000000003"},
			},
			Extensions: []types.ExtensionEntry{
				{
					Name: "Edge Helper", ID: "aaaabbbbccccddddeeeeffffgggghhhh", Description: `Installed under C:\Program Files\Helper`,
					Version: "2.0", HomepageURL: "https://helper.example/", Enabled: true,
				},
			},
			LocalStorage: []types.StorageEntry{
				{URL: "https://edge.example", Key: "日本語"},
			},
			SessionStorage: []types.StorageEntry{
				{IsMeta: true, URL: "https://edge.example", Value: "meta data, value bytes is [16 2]"},
			},
		}},
	}
}

func exportSparseStamps() []exportStamp {
	return []exportStamp{
		{browser: "Chrome", profile: "Default", data: &types.BrowserData{
			Passwords: []types.LoginEntry{{URL: "https://example.com/", Username: "alice", Store: types.PasswordStoreLocal}},
			Cookies:   []types.CookieEntry{},
		}},
		{browser: "Chrome", profile: "Profile 1"},
		{browser: "Microsoft Edge", profile: "Default", data: &types.BrowserData{
			Histories:    []types.HistoryEntry{},
			LocalStorage: []types.StorageEntry{{URL: "https://example.com", Key: "k", Value: "v"}},
		}},
	}
}
