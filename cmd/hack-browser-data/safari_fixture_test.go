package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/moond4rk/plist"
	"github.com/stretchr/testify/require"
)

const (
	safariFixtureWorkUUID     = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	safariFixtureUntitledUUID = "9A1B2C3D-4E5F-4A6B-8C7D-0E1F2A3B4C5D"
)

var safariFixtureStart = time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)

func safariFixtureLibrary(t *testing.T, library string) {
	t.Helper()
	at := safariFixtureStart.Add
	visit := func(offset time.Duration) float64 { return safariFixtureCoreData(at(offset)) }
	day := 24 * time.Hour
	legacyHome := filepath.Join(library, "Safari")
	container := filepath.Join(library, "Containers", "com.apple.Safari", "Data", "Library")
	workStore := filepath.Join(container, "WebKit", "WebsiteDataStore", strings.ToLower(safariFixtureWorkUUID))

	fixtureSQLite(t, filepath.Join(container, "Safari", "SafariTabs.db"), safariFixtureTabsSchema,
		`INSERT INTO bookmarks (id, parent, type, title, order_index, external_uuid, subtype) VALUES
		 (1, 0, 1, 'Favorites', 0, 'Favorites', 0),
		 (2, 0, 1, '', 1, 'DefaultProfile', 2),
		 (3, 0, 1, 'Work', 2, '`+safariFixtureWorkUUID+`', 2),
		 (4, 0, 1, NULL, 3, '`+safariFixtureUntitledUUID+`', 2)`)

	fixtureSQLite(t, filepath.Join(legacyHome, "History.db"), safariFixtureHistoryItemsSchema, safariFixtureHistoryVisitsSchema,
		safariFixtureInsertHistoryItem(1, "https://www.home.safari.test/", 42),
		safariFixtureInsertHistoryVisit(1, 1, visit(time.Hour), "Home (old)"),
		safariFixtureInsertHistoryVisit(2, 1, visit(3*time.Hour+500*time.Millisecond), "Home"),
		safariFixtureInsertHistoryItem(2, "https://docs.home.safari.test/guide?q=go&lang=en", 7),
		safariFixtureInsertHistoryVisit(3, 2, visit(4*time.Hour), `Guide, "quoted" & <b>bold</b>`),
		safariFixtureInsertHistoryItem(3, "https://news.home.safari.test/ja", 3),
		safariFixtureInsertHistoryVisit(4, 3, visit(5*time.Hour), "ニュース"),
		safariFixtureInsertHistoryItem(4, "https://files.home.safari.test/report.pdf", 0),
	)
	fixtureSQLite(t, filepath.Join(container, "Safari", "Profiles", safariFixtureWorkUUID, "History.db"),
		safariFixtureHistoryItemsSchema, safariFixtureHistoryVisitsSchema,
		safariFixtureInsertHistoryItem(1, "https://intranet.work.safari.test/", 12),
		safariFixtureInsertHistoryVisit(1, 1, visit(6*time.Hour), "Intranet"),
		safariFixtureInsertHistoryItem(2, "https://wiki.work.safari.test/onboarding", 2),
		safariFixtureInsertHistoryVisit(2, 2, visit(7*time.Hour), "Onboarding"),
	)
	fixtureSQLite(t, filepath.Join(container, "Safari", "Profiles", safariFixtureUntitledUUID, "History.db"),
		safariFixtureHistoryItemsSchema, safariFixtureHistoryVisitsSchema,
		safariFixtureInsertHistoryItem(1, "https://www.untitled.safari.test/", 1),
		safariFixtureInsertHistoryVisit(1, 1, visit(8*time.Hour), "Untitled"),
	)

	fixtureWrite(t, filepath.Join(container, "Cookies", "Cookies.binarycookies"), string(safariFixtureCookieFile(t,
		[]safariFixtureCookie{
			{
				domain: ".home.safari.test", name: "SID", path: "/", value: "sid-home.safari.test", flags: 0x5,
				expires: at(365 * day), created: at(10*time.Minute + 750*time.Millisecond),
			},
			{
				domain: "www.home.safari.test", name: "prefs", path: "/settings", value: "theme=dark",
				expires: at(30 * day), created: at(20 * time.Minute),
			},
		},
		[]safariFixtureCookie{
			{
				domain: ".home.safari.test", name: "tracker", path: "/", value: "opt-out", flags: 0x1,
				expires: at(730 * day), created: at(30 * time.Minute),
			},
			{
				domain: "app.home.safari.test", name: "csrf", path: "/", value: "token-123", flags: 0x4,
				expires: at(day), created: at(40 * time.Minute),
			},
		},
	)))
	// Since macOS 14 the jar lives in the container, which wins over a leftover ~/Library/Cookies jar.
	fixtureWrite(t, filepath.Join(library, "Cookies", "Cookies.binarycookies"), string(safariFixtureCookieFile(t,
		[]safariFixtureCookie{{
			domain: "old.home.safari.test", name: "legacy", path: "/", value: "stale",
			expires: at(day), created: at(time.Minute),
		}},
	)))
	fixtureWrite(t, filepath.Join(workStore, "Cookies", "Cookies.binarycookies"), string(safariFixtureCookieFile(t,
		[]safariFixtureCookie{
			{
				domain: ".work.safari.test", name: "SSO", path: "/", value: "sso-work.safari.test", flags: 0x5,
				expires: at(7 * day), created: at(6 * time.Hour),
			},
			{
				domain: "wiki.work.safari.test", name: "lang", path: "/", value: "en",
				expires: at(90 * day), created: at(7 * time.Hour),
			},
		},
	)))

	safariFixturePlist(t, filepath.Join(legacyHome, "Bookmarks.plist"), safariFixtureBookmarks)
	safariFixturePlist(t, filepath.Join(legacyHome, "Downloads.plist"), fmt.Sprintf(safariFixtureDownloads, safariFixtureWorkUUID))

	defaultStore := filepath.Join(container, "WebKit", "WebsiteData", "Default")
	fixtureWrite(t, filepath.Join(defaultStore, "salt"), "\x8f\x1e\x5a\x33\x0c\xd4\x71\x9b")
	app := safariFixtureEndpoint{scheme: "https", host: "app.home.safari.test"}
	safariFixtureOrigin(t, defaultStore, app, app,
		[2]string{"greeting", "café"},
		[2]string{"言語", "日本語"},
		[2]string{"blob", strings.Repeat("x", 1100)},
	)
	safariFixtureOrigin(t, defaultStore,
		safariFixtureEndpoint{scheme: "https", host: "shop.home.safari.test"},
		safariFixtureEndpoint{scheme: "https", host: "cdn.home.safari.test", port: 8443},
		[2]string{"consent", "yes"},
	)
	idbOnly := safariFixtureEndpoint{scheme: "https", host: "idb.home.safari.test"}
	safariFixtureOrigin(t, defaultStore, idbOnly, idbOnly)
	workApp := safariFixtureEndpoint{scheme: "https", host: "app.work.safari.test"}
	safariFixtureOrigin(t, filepath.Join(workStore, "Origins"), workApp, workApp, [2]string{"greeting", "hello"})

	safariFixturePlist(t, filepath.Join(container, "Safari", "AppExtensions", "Extensions.plist"), safariFixtureAppExtensions)
	safariFixturePlist(t, filepath.Join(container, "Safari", "WebExtensions", "Extensions.plist"), safariFixtureWebExtensions)
}

func safariFixtureCoreData(ts time.Time) float64 {
	const unixToCoreDataSeconds = 978_307_200
	return float64(ts.UnixMilli())/1000 - unixToCoreDataSeconds
}

// Safari writes binary plists; the fixtures keep XML so they read like plutil's XML output.
func safariFixturePlist(t *testing.T, path, body string) {
	t.Helper()
	var v any
	_, err := plist.Unmarshal([]byte(`<plist version="1.0">`+body+`</plist>`), &v)
	require.NoError(t, err)
	data, err := plist.Marshal(v, plist.BinaryFormat)
	require.NoError(t, err)
	fixtureWrite(t, path, string(data))
}

type safariFixtureCookie struct {
	domain, name, path, value string
	flags                     uint32
	expires, created          time.Time
}

type safariFixtureCookieHeader struct {
	Size          uint32
	Version       uint32
	Flags         uint32
	HasPort       uint32
	DomainOffset  uint32
	NameOffset    uint32
	PathOffset    uint32
	ValueOffset   uint32
	CommentOffset uint32
	EndOfHeader   uint32
	Expires       float64
	Created       float64
}

// Each argument is one page of the jar.
func safariFixtureCookieFile(t *testing.T, pages ...[]safariFixtureCookie) []byte {
	t.Helper()
	encoded := make([][]byte, len(pages))
	file := binary.BigEndian.AppendUint32([]byte("cook"), uint32(len(pages)))
	for i, cookies := range pages {
		encoded[i] = safariFixtureCookiePage(t, cookies)
		file = binary.BigEndian.AppendUint32(file, uint32(len(encoded[i])))
	}
	for _, page := range encoded {
		file = append(file, page...)
	}
	return append(file, make([]byte, 8)...) // the checksum, which the decoder reads but never verifies
}

func safariFixtureCookiePage(t *testing.T, cookies []safariFixtureCookie) []byte {
	t.Helper()
	records := make([][]byte, len(cookies))
	for i, c := range cookies {
		records[i] = safariFixtureCookieRecord(t, c)
	}
	page := binary.LittleEndian.AppendUint32([]byte{0x00, 0x00, 0x01, 0x00}, uint32(len(cookies)))
	offset := 4 + 4 + 4*len(cookies) + 4
	for _, r := range records {
		page = binary.LittleEndian.AppendUint32(page, uint32(offset))
		offset += len(r)
	}
	page = append(page, 0x00, 0x00, 0x00, 0x00)
	for _, r := range records {
		page = append(page, r...)
	}
	return page
}

func safariFixtureCookieRecord(t *testing.T, c safariFixtureCookie) []byte {
	t.Helper()
	header := safariFixtureCookieHeader{
		Flags:   c.flags,
		Expires: safariFixtureCoreData(c.expires),
		Created: safariFixtureCoreData(c.created),
	}
	headerSize := uint32(binary.Size(header))
	var strs []byte
	for _, f := range []struct {
		offset *uint32
		value  string
	}{
		{&header.DomainOffset, c.domain},
		{&header.NameOffset, c.name},
		{&header.PathOffset, c.path},
		{&header.ValueOffset, c.value},
	} {
		*f.offset = headerSize + uint32(len(strs))
		strs = append(append(strs, f.value...), 0x00)
	}
	header.Size = headerSize + uint32(len(strs))
	var b bytes.Buffer
	require.NoError(t, binary.Write(&b, binary.LittleEndian, header))
	return append(b.Bytes(), strs...)
}

type safariFixtureEndpoint struct {
	scheme, host string
	port         uint16
}

// One partition of a WebKit origins root: top and frame differ for third-party storage, and an origin without
// localStorage items keeps only its origin file.
func safariFixtureOrigin(t *testing.T, root string, top, frame safariFixtureEndpoint, items ...[2]string) {
	t.Helper()
	dir := filepath.Join(root, safariFixtureHash(top), safariFixtureHash(frame))
	fixtureWrite(t, filepath.Join(dir, "origin"), string(append(safariFixtureOriginBlock(top), safariFixtureOriginBlock(frame)...)))
	if len(items) == 0 {
		return
	}
	stmts := []string{safariFixtureItemTableSchema}
	for _, item := range items {
		stmts = append(stmts, fmt.Sprintf(`INSERT INTO ItemTable (key, value) VALUES ('%s', x'%x')`, item[0], safariFixtureUTF16(item[1])))
	}
	fixtureSQLite(t, filepath.Join(dir, "LocalStorage", "localstorage.sqlite3"), stmts...)
}

func safariFixtureHash(e safariFixtureEndpoint) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s://%s:%d", e.scheme, e.host, e.port)))
	return hex.EncodeToString(sum[:])
}

func safariFixtureOriginBlock(e safariFixtureEndpoint) []byte {
	const latin1, defaultPort, explicitPort = 0x01, 0x00, 0x01
	var b []byte
	for _, s := range []string{e.scheme, e.host} {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(s)))
		b = append(append(b, latin1), s...)
	}
	if e.port == 0 {
		return append(b, defaultPort)
	}
	return binary.LittleEndian.AppendUint16(append(b, explicitPort), e.port)
}

func safariFixtureUTF16(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

func safariFixtureInsertHistoryItem(id int, url string, visitCount int) string {
	return fmt.Sprintf(`INSERT INTO history_items (id, url, visit_count) VALUES (%d, '%s', %d)`, id, url, visitCount)
}

func safariFixtureInsertHistoryVisit(id, item int, visitTime float64, title string) string {
	return fmt.Sprintf(`INSERT INTO history_visits (id, history_item, visit_time, title) VALUES (%d, %d, %f, '%s')`,
		id, item, visitTime, title)
}

const safariFixtureTabsSchema = `CREATE TABLE bookmarks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	parent INTEGER,
	type INTEGER,
	title TEXT,
	url TEXT,
	order_index INTEGER NOT NULL,
	external_uuid TEXT UNIQUE,
	subtype INTEGER DEFAULT 0
)`

const safariFixtureHistoryItemsSchema = `CREATE TABLE history_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT NOT NULL UNIQUE,
	domain_expansion TEXT NULL,
	visit_count INTEGER NOT NULL DEFAULT 0,
	daily_visit_counts BLOB NOT NULL DEFAULT x'',
	weekly_visit_counts BLOB NULL,
	autocomplete_triggers BLOB NULL,
	should_recompute_derived_visit_counts INTEGER NOT NULL DEFAULT 1,
	visit_count_score INTEGER NOT NULL DEFAULT 0
)`

const safariFixtureHistoryVisitsSchema = `CREATE TABLE history_visits (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	history_item INTEGER NOT NULL REFERENCES history_items(id),
	visit_time REAL NOT NULL,
	title TEXT NULL,
	load_successful BOOLEAN NOT NULL DEFAULT 1,
	http_non_get INTEGER NOT NULL DEFAULT 0,
	synthesized INTEGER NOT NULL DEFAULT 0,
	redirect_source INTEGER NULL,
	redirect_destination INTEGER NULL,
	origin INTEGER NOT NULL DEFAULT 0,
	generation INTEGER NOT NULL DEFAULT 0,
	attributes INTEGER NOT NULL DEFAULT 0,
	score INTEGER NOT NULL DEFAULT 0
)`

const safariFixtureItemTableSchema = `CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`

const safariFixtureBookmarks = `<dict>
	<key>Children</key>
	<array>
		<dict>
			<key>Title</key><string>History</string>
			<key>WebBookmarkIdentifier</key><string>History</string>
			<key>WebBookmarkType</key><string>WebBookmarkTypeProxy</string>
		</dict>
		<dict>
			<key>Children</key>
			<array>
				<dict>
					<key>URIDictionary</key><dict><key>title</key><string>Docs</string></dict>
					<key>URLString</key><string>https://docs.home.safari.test/</string>
					<key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
				</dict>
				<dict>
					<key>Children</key>
					<array>
						<dict>
							<key>URIDictionary</key><dict><key>title</key><string>Long read &lt;with&gt; &amp; tags</string></dict>
							<key>URLString</key><string>https://news.home.safari.test/long-read</string>
							<key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
						</dict>
					</array>
					<key>Title</key><string>Reading</string>
					<key>WebBookmarkType</key><string>WebBookmarkTypeList</string>
				</dict>
				<dict>
					<key>URIDictionary</key><dict><key>title</key><string>Home</string></dict>
					<key>URLString</key><string>https://www.home.safari.test/</string>
					<key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
				</dict>
			</array>
			<key>Title</key><string>BookmarksBar</string>
			<key>WebBookmarkType</key><string>WebBookmarkTypeList</string>
		</dict>
		<dict>
			<key>Children</key>
			<array>
				<dict>
					<key>URIDictionary</key><dict><key>title</key><string>Café menu</string></dict>
					<key>URLString</key><string>https://shop.home.safari.test/menu</string>
					<key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
				</dict>
			</array>
			<key>Title</key><string>BookmarksMenu</string>
			<key>WebBookmarkType</key><string>WebBookmarkTypeList</string>
		</dict>
		<dict>
			<key>Children</key>
			<array>
				<dict>
					<key>ReadingList</key><dict><key>PreviewText</key><string>A post worth saving.</string></dict>
					<key>URIDictionary</key><dict><key>title</key><string>Saved post</string></dict>
					<key>URLString</key><string>https://blog.home.safari.test/post</string>
					<key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
				</dict>
				<dict>
					<key>Title</key><string>Untitled entry</string>
					<key>URLString</key><string>https://blog.home.safari.test/draft</string>
					<key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
				</dict>
			</array>
			<key>Title</key><string>com.apple.ReadingList</string>
			<key>WebBookmarkType</key><string>WebBookmarkTypeList</string>
		</dict>
	</array>
	<key>Title</key><string></string>
	<key>WebBookmarkFileVersion</key><integer>1</integer>
	<key>WebBookmarkType</key><string>WebBookmarkTypeList</string>
</dict>`

const safariFixtureDownloads = `<dict>
	<key>DownloadHistory</key>
	<array>
		<dict>
			<key>DownloadEntryIdentifier</key><string>6B3E1D2C-0A9F-4E8D-B7C6-A5F4E3D2C1B0</string>
			<key>DownloadEntryPath</key><string>/Users/alice/Downloads/report.pdf</string>
			<key>DownloadEntryProfileUUIDStringKey</key><string>DefaultProfile</string>
			<key>DownloadEntryProgressBytesSoFar</key><integer>52344</integer>
			<key>DownloadEntryProgressTotalToLoad</key><integer>52344</integer>
			<key>DownloadEntryRemoveWhenDoneKey</key><false/>
			<key>DownloadEntryURL</key><string>https://files.home.safari.test/report.pdf</string>
		</dict>
		<dict>
			<key>DownloadEntryIdentifier</key><string>1C2D3E4F-5A6B-4C7D-8E9F-0A1B2C3D4E5F</string>
			<key>DownloadEntryPath</key><string>/Users/alice/Downloads/slides.key</string>
			<key>DownloadEntryProfileUUIDStringKey</key><string>%[1]s</string>
			<key>DownloadEntryProgressBytesSoFar</key><integer>1048576</integer>
			<key>DownloadEntryProgressTotalToLoad</key><integer>1048576</integer>
			<key>DownloadEntryRemoveWhenDoneKey</key><false/>
			<key>DownloadEntryURL</key><string>https://files.work.safari.test/slides.key</string>
		</dict>
		<dict>
			<key>DownloadEntryIdentifier</key><string>F0E1D2C3-B4A5-4968-8776-5A4B3C2D1E0F</string>
			<key>DownloadEntryPath</key><string>/Users/alice/Downloads/archive.tar.gz</string>
			<key>DownloadEntryProgressBytesSoFar</key><integer>2048</integer>
			<key>DownloadEntryProgressTotalToLoad</key><integer>2048</integer>
			<key>DownloadEntryRemoveWhenDoneKey</key><true/>
			<key>DownloadEntryURL</key><string>https://files.home.safari.test/archive.tar.gz</string>
		</dict>
	</array>
</dict>`

const safariFixtureAppExtensions = `<dict>
	<key>com.example.Clipper.Extension (ABCDE12345)</key>
	<dict><key>Permissions</key><dict/></dict>
</dict>`

const safariFixtureWebExtensions = `<dict>
	<key>com.example.Blocker.Extension (FGHIJ67890)</key>
	<dict><key>Enabled</key><true/></dict>
	<key>com.example.Reader.Extension (KLMNO13579)</key>
	<dict><key>Enabled</key><false/></dict>
	<key>com.example.Legacy</key>
	<dict><key>Enabled</key><true/></dict>
</dict>`
