package browsertest

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
	safariWorkUUID     = "5604E6F5-02ED-4E40-8249-63DE7BC986C8"
	safariUntitledUUID = "9A1B2C3D-4E5F-4A6B-8C7D-0E1F2A3B4C5D"
)

var safariStart = time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)

func WriteSafariLibrary(t *testing.T, library string) {
	t.Helper()
	timeAt := safariStart.Add
	visit := func(offset time.Duration) float64 { return safariCoreData(timeAt(offset)) }
	day := 24 * time.Hour
	legacyHome := filepath.Join(library, "Safari")
	container := filepath.Join(library, "Containers", "com.apple.Safari", "Data", "Library")
	workStore := filepath.Join(container, "WebKit", "WebsiteDataStore", strings.ToLower(safariWorkUUID))

	writeSQLite(t, filepath.Join(container, "Safari", "SafariTabs.db"), safariTabsSchema,
		`INSERT INTO bookmarks (id, parent, type, title, order_index, external_uuid, subtype) VALUES
		 (1, 0, 1, 'Favorites', 0, 'Favorites', 0),
		 (2, 0, 1, '', 1, 'DefaultProfile', 2),
		 (3, 0, 1, 'Work', 2, '`+safariWorkUUID+`', 2),
		 (4, 0, 1, NULL, 3, '`+safariUntitledUUID+`', 2)`)

	writeSQLite(t, filepath.Join(legacyHome, "History.db"), safariHistoryItemsSchema, safariHistoryVisitsSchema,
		insertSafariHistoryItem(1, "https://www.home.safari.test/", 42),
		insertSafariHistoryVisit(1, 1, visit(time.Hour), "Home (old)"),
		insertSafariHistoryVisit(2, 1, visit(3*time.Hour+500*time.Millisecond), "Home"),
		insertSafariHistoryItem(2, "https://docs.home.safari.test/guide?q=go&lang=en", 7),
		insertSafariHistoryVisit(3, 2, visit(4*time.Hour), `Guide, "quoted" & <b>bold</b>`),
		insertSafariHistoryItem(3, "https://news.home.safari.test/ja", 3),
		insertSafariHistoryVisit(4, 3, visit(5*time.Hour), "ニュース"),
		insertSafariHistoryItem(4, "https://files.home.safari.test/report.pdf", 0),
	)
	writeSQLite(t, filepath.Join(container, "Safari", "Profiles", safariWorkUUID, "History.db"),
		safariHistoryItemsSchema, safariHistoryVisitsSchema,
		insertSafariHistoryItem(1, "https://intranet.work.safari.test/", 12),
		insertSafariHistoryVisit(1, 1, visit(6*time.Hour), "Intranet"),
		insertSafariHistoryItem(2, "https://wiki.work.safari.test/onboarding", 2),
		insertSafariHistoryVisit(2, 2, visit(7*time.Hour), "Onboarding"),
	)
	writeSQLite(t, filepath.Join(container, "Safari", "Profiles", safariUntitledUUID, "History.db"),
		safariHistoryItemsSchema, safariHistoryVisitsSchema,
		insertSafariHistoryItem(1, "https://www.untitled.safari.test/", 1),
		insertSafariHistoryVisit(1, 1, visit(8*time.Hour), "Untitled"),
	)

	writeFile(t, filepath.Join(container, "Cookies", "Cookies.binarycookies"), string(encodeSafariCookieFile(t,
		[]safariCookie{
			{
				domain: ".home.safari.test", name: "SID", path: "/", value: "sid-home.safari.test", flags: 0x5,
				expires: timeAt(365 * day), created: timeAt(10*time.Minute + 750*time.Millisecond),
			},
			{
				domain: "www.home.safari.test", name: "prefs", path: "/settings", value: "theme=dark",
				expires: timeAt(30 * day), created: timeAt(20 * time.Minute),
			},
		},
		[]safariCookie{
			{
				domain: ".home.safari.test", name: "tracker", path: "/", value: "opt-out", flags: 0x1,
				expires: timeAt(730 * day), created: timeAt(30 * time.Minute),
			},
			{
				domain: "app.home.safari.test", name: "csrf", path: "/", value: "token-123", flags: 0x4,
				expires: timeAt(day), created: timeAt(40 * time.Minute),
			},
		},
	)))
	// Since macOS 14 the jar lives in the container, which wins over a leftover ~/Library/Cookies jar.
	writeFile(t, filepath.Join(library, "Cookies", "Cookies.binarycookies"), string(encodeSafariCookieFile(t,
		[]safariCookie{{
			domain: "old.home.safari.test", name: "legacy", path: "/", value: "stale",
			expires: timeAt(day), created: timeAt(time.Minute),
		}},
	)))
	writeFile(t, filepath.Join(workStore, "Cookies", "Cookies.binarycookies"), string(encodeSafariCookieFile(t,
		[]safariCookie{
			{
				domain: ".work.safari.test", name: "SSO", path: "/", value: "sso-work.safari.test", flags: 0x5,
				expires: timeAt(7 * day), created: timeAt(6 * time.Hour),
			},
			{
				domain: "wiki.work.safari.test", name: "lang", path: "/", value: "en",
				expires: timeAt(90 * day), created: timeAt(7 * time.Hour),
			},
		},
	)))

	writeSafariPlist(t, filepath.Join(legacyHome, "Bookmarks.plist"), safariBookmarksPlist)
	writeSafariPlist(t, filepath.Join(legacyHome, "Downloads.plist"), fmt.Sprintf(safariDownloadsPlist, safariWorkUUID))

	defaultStore := filepath.Join(container, "WebKit", "WebsiteData", "Default")
	writeFile(t, filepath.Join(defaultStore, "salt"), "\x8f\x1e\x5a\x33\x0c\xd4\x71\x9b")
	app := safariEndpoint{scheme: "https", host: "app.home.safari.test"}
	writeSafariOrigin(t, defaultStore, app, app,
		[2]string{"greeting", "café"},
		[2]string{"言語", "日本語"},
		[2]string{"blob", strings.Repeat("x", 1100)},
	)
	writeSafariOrigin(t, defaultStore,
		safariEndpoint{scheme: "https", host: "shop.home.safari.test"},
		safariEndpoint{scheme: "https", host: "cdn.home.safari.test", port: 8443},
		[2]string{"consent", "yes"},
	)
	idbOnly := safariEndpoint{scheme: "https", host: "idb.home.safari.test"}
	writeSafariOrigin(t, defaultStore, idbOnly, idbOnly)
	workApp := safariEndpoint{scheme: "https", host: "app.work.safari.test"}
	writeSafariOrigin(t, filepath.Join(workStore, "Origins"), workApp, workApp, [2]string{"greeting", "hello"})

	writeSafariPlist(t, filepath.Join(container, "Safari", "AppExtensions", "Extensions.plist"), safariAppExtensionsPlist)
	writeSafariPlist(t, filepath.Join(container, "Safari", "WebExtensions", "Extensions.plist"), safariWebExtensionsPlist)
}

func safariCoreData(timestamp time.Time) float64 {
	const unixToCoreDataSeconds = 978_307_200
	return float64(timestamp.UnixMilli())/1000 - unixToCoreDataSeconds
}

// writeSafariPlist writes XML, though Safari writes binary plists, so the fixtures read like plutil's XML output.
func writeSafariPlist(t *testing.T, path, body string) {
	t.Helper()
	var value any
	_, err := plist.Unmarshal([]byte(`<plist version="1.0">`+body+`</plist>`), &value)
	require.NoError(t, err)
	data, err := plist.Marshal(value, plist.BinaryFormat)
	require.NoError(t, err)
	writeFile(t, path, string(data))
}

type safariCookie struct {
	domain, name, path, value string
	flags                     uint32
	expires, created          time.Time
}

type safariCookieHeader struct {
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

// encodeSafariCookieFile takes one page of the jar per argument.
func encodeSafariCookieFile(t *testing.T, pages ...[]safariCookie) []byte {
	t.Helper()
	encoded := make([][]byte, len(pages))
	file := binary.BigEndian.AppendUint32([]byte("cook"), uint32(len(pages)))
	for i, cookies := range pages {
		encoded[i] = encodeSafariCookiePage(t, cookies)
		file = binary.BigEndian.AppendUint32(file, uint32(len(encoded[i])))
	}
	for _, page := range encoded {
		file = append(file, page...)
	}
	return append(file, make([]byte, 8)...) // the checksum, which the decoder reads but never verifies
}

func encodeSafariCookiePage(t *testing.T, cookies []safariCookie) []byte {
	t.Helper()
	records := make([][]byte, len(cookies))
	for i, cookie := range cookies {
		records[i] = encodeSafariCookieRecord(t, cookie)
	}
	page := binary.LittleEndian.AppendUint32([]byte{0x00, 0x00, 0x01, 0x00}, uint32(len(cookies)))
	offset := 4 + 4 + 4*len(cookies) + 4
	for _, encodedRecord := range records {
		page = binary.LittleEndian.AppendUint32(page, uint32(offset))
		offset += len(encodedRecord)
	}
	page = append(page, 0x00, 0x00, 0x00, 0x00)
	for _, encodedRecord := range records {
		page = append(page, encodedRecord...)
	}
	return page
}

func encodeSafariCookieRecord(t *testing.T, cookie safariCookie) []byte {
	t.Helper()
	header := safariCookieHeader{
		Flags:   cookie.flags,
		Expires: safariCoreData(cookie.expires),
		Created: safariCoreData(cookie.created),
	}
	headerSize := uint32(binary.Size(header))
	var strs []byte
	for _, field := range []struct {
		offset *uint32
		value  string
	}{
		{&header.DomainOffset, cookie.domain},
		{&header.NameOffset, cookie.name},
		{&header.PathOffset, cookie.path},
		{&header.ValueOffset, cookie.value},
	} {
		*field.offset = headerSize + uint32(len(strs))
		strs = append(append(strs, field.value...), 0x00)
	}
	header.Size = headerSize + uint32(len(strs))
	var buffer bytes.Buffer
	require.NoError(t, binary.Write(&buffer, binary.LittleEndian, header))
	return append(buffer.Bytes(), strs...)
}

type safariEndpoint struct {
	scheme, host string
	port         uint16
}

// writeSafariOrigin takes top and frame, which differ for third-party storage; without items, one origin file stays.
func writeSafariOrigin(t *testing.T, root string, top, frame safariEndpoint, items ...[2]string) {
	t.Helper()
	dir := filepath.Join(root, safariOriginHash(top), safariOriginHash(frame))
	writeFile(t, filepath.Join(dir, "origin"), string(append(encodeSafariOriginBlock(top), encodeSafariOriginBlock(frame)...)))
	if len(items) == 0 {
		return
	}
	statements := []string{safariItemTableSchema}
	for _, item := range items {
		statements = append(statements, fmt.Sprintf(`INSERT INTO ItemTable (key, value) VALUES ('%s', x'%x')`, item[0], safariUTF16(item[1])))
	}
	writeSQLite(t, filepath.Join(dir, "LocalStorage", "localstorage.sqlite3"), statements...)
}

func safariOriginHash(endpoint safariEndpoint) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s://%s:%d", endpoint.scheme, endpoint.host, endpoint.port)))
	return hex.EncodeToString(sum[:])
}

func encodeSafariOriginBlock(endpoint safariEndpoint) []byte {
	const latin1, defaultPort, explicitPort = 0x01, 0x00, 0x01
	var block []byte
	for _, part := range []string{endpoint.scheme, endpoint.host} {
		block = binary.LittleEndian.AppendUint32(block, uint32(len(part)))
		block = append(append(block, latin1), part...)
	}
	if endpoint.port == 0 {
		return append(block, defaultPort)
	}
	return binary.LittleEndian.AppendUint16(append(block, explicitPort), endpoint.port)
}

func safariUTF16(text string) []byte {
	var encoded []byte
	for _, unit := range utf16.Encode([]rune(text)) {
		encoded = binary.LittleEndian.AppendUint16(encoded, unit)
	}
	return encoded
}

func insertSafariHistoryItem(id int, url string, visitCount int) string {
	return fmt.Sprintf(`INSERT INTO history_items (id, url, visit_count) VALUES (%d, '%s', %d)`, id, url, visitCount)
}

func insertSafariHistoryVisit(id, item int, visitTime float64, title string) string {
	return fmt.Sprintf(`INSERT INTO history_visits (id, history_item, visit_time, title) VALUES (%d, %d, %f, '%s')`,
		id, item, visitTime, title)
}

const safariTabsSchema = `CREATE TABLE bookmarks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	parent INTEGER,
	type INTEGER,
	title TEXT,
	url TEXT,
	order_index INTEGER NOT NULL,
	external_uuid TEXT UNIQUE,
	subtype INTEGER DEFAULT 0
)`

const safariHistoryItemsSchema = `CREATE TABLE history_items (
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

const safariHistoryVisitsSchema = `CREATE TABLE history_visits (
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

const safariItemTableSchema = `CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB NOT NULL ON CONFLICT FAIL)`

const safariBookmarksPlist = `<dict>
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

const safariDownloadsPlist = `<dict>
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

const safariAppExtensionsPlist = `<dict>
	<key>com.example.Clipper.Extension (ABCDE12345)</key>
	<dict><key>Permissions</key><dict/></dict>
</dict>`

const safariWebExtensionsPlist = `<dict>
	<key>com.example.Blocker.Extension (FGHIJ67890)</key>
	<dict><key>Enabled</key><true/></dict>
	<key>com.example.Reader.Extension (KLMNO13579)</key>
	<dict><key>Enabled</key><false/></dict>
	<key>com.example.Legacy</key>
	<dict><key>Enabled</key><true/></dict>
</dict>`
