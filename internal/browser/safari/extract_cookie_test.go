package safari

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// buildTestBinaryCookies puts each cookie in its own page of a minimal Cookies.binarycookies file.
func buildTestBinaryCookies(t *testing.T, cookies []testCookie) string {
	t.Helper()

	var pages [][]byte
	for _, cookie := range cookies {
		pages = append(pages, buildPage(cookie))
	}

	path := filepath.Join(t.TempDir(), "Cookies.binarycookies")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()

	// File header: magic + numPages (big-endian)
	_, err = file.WriteString("cook")
	require.NoError(t, err)
	require.NoError(t, binary.Write(file, binary.BigEndian, uint32(len(pages))))

	// Page sizes (big-endian)
	for _, page := range pages {
		require.NoError(t, binary.Write(file, binary.BigEndian, uint32(len(page))))
	}

	for _, page := range pages {
		_, err = file.Write(page)
		require.NoError(t, err)
	}

	// Checksum (8 bytes, not validated by decoder)
	_, err = file.Write(make([]byte, 8))
	require.NoError(t, err)

	return path
}

type testCookie struct {
	domain, name, path, value string
	secure, httpOnly          bool
	expires, creation         float64 // Core Data epoch seconds
}

func buildPage(fixture testCookie) []byte {
	// Cookie string data: domain\0 name\0 path\0 value\0
	domain := fixture.domain + "\x00"
	name := fixture.name + "\x00"
	cookiePath := fixture.path + "\x00"
	value := fixture.value + "\x00"

	// A cookie is a 56-byte header (sizes, flags, string offsets, expiry, creation), then its strings.
	const headerSize = 56
	domainOffset := uint32(headerSize)
	nameOffset := domainOffset + uint32(len(domain))
	pathOffset := nameOffset + uint32(len(name))
	valueOffset := pathOffset + uint32(len(cookiePath))
	cookieSize := valueOffset + uint32(len(value))

	var flags uint32
	switch {
	case fixture.secure && fixture.httpOnly:
		flags = 0x5
	case fixture.httpOnly:
		flags = 0x4
	case fixture.secure:
		flags = 0x1
	}

	// Cookie fields are little-endian, unlike the file header.
	cookie := make([]byte, cookieSize)
	binary.LittleEndian.PutUint32(cookie[0:], cookieSize)
	binary.LittleEndian.PutUint32(cookie[8:], flags)
	binary.LittleEndian.PutUint32(cookie[16:], domainOffset)
	binary.LittleEndian.PutUint32(cookie[20:], nameOffset)
	binary.LittleEndian.PutUint32(cookie[24:], pathOffset)
	binary.LittleEndian.PutUint32(cookie[28:], valueOffset)
	binary.LittleEndian.PutUint64(cookie[40:], math.Float64bits(fixture.expires))
	binary.LittleEndian.PutUint64(cookie[48:], math.Float64bits(fixture.creation))
	copy(cookie[domainOffset:], domain)
	copy(cookie[nameOffset:], name)
	copy(cookie[pathOffset:], cookiePath)
	copy(cookie[valueOffset:], value)

	// Page layout: marker(4) + cookieCount(4) + offsets(4*N) + endMarker(4) + cookies
	const pageHeaderSize = 16
	page := make([]byte, pageHeaderSize+len(cookie))
	copy(page[0:4], []byte{0x00, 0x00, 0x01, 0x00}) // page start marker
	binary.LittleEndian.PutUint32(page[4:], 1)      // 1 cookie
	binary.LittleEndian.PutUint32(page[8:], pageHeaderSize)
	copy(page[pageHeaderSize:], cookie)

	return page
}

func TestExtractCookies(t *testing.T) {
	path := buildTestBinaryCookies(t, []testCookie{
		{
			domain: ".example.com", name: "session", path: "/", value: "abc123",
			secure: true, httpOnly: true,
			expires: 2000000000.0, creation: 700000000.0,
		},
		{
			domain: ".go.dev", name: "lang", path: "/", value: "en",
			secure: false, httpOnly: false,
			expires: 2000000000.0, creation: 750000000.0,
		},
	})

	var out testCollector
	err := extractCookies(path, &out)
	cookies := collected[record.Cookie](t, &out)
	require.NoError(t, err)
	require.Len(t, cookies, 2)

	// Cookies keep the file's order; the exporter sorts them.
	assert.Equal(t, ".example.com", cookies[0].Host)
	assert.Equal(t, ".go.dev", cookies[1].Host)

	cookie := cookies[0]
	assert.Equal(t, "session", cookie.Name)
	assert.Equal(t, "abc123", cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.IsSecure)
	assert.True(t, cookie.IsHTTPOnly)
	assert.False(t, cookie.CreatedAt.IsZero())
	assert.False(t, cookie.ExpireAt.IsZero())
}

func TestExtractCookies_InvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.binarycookies")
	require.NoError(t, os.WriteFile(path, []byte("not a cookies file"), 0o644))

	err := extractCookies(path, &testCollector{})
	assert.Error(t, err)
}

func TestExtractCookies_FileNotFound(t *testing.T) {
	err := extractCookies("/nonexistent/Cookies.binarycookies", &testCollector{})
	assert.Error(t, err)
}
