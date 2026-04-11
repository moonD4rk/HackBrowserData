package safari

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestBinaryCookies constructs a minimal valid Cookies.binarycookies file
// containing the given cookies. Each cookie is placed in its own page.
func buildTestBinaryCookies(t *testing.T, cookies []testCookie) string {
	t.Helper()

	var pages [][]byte
	for _, c := range cookies {
		pages = append(pages, buildPage(c))
	}

	path := filepath.Join(t.TempDir(), "Cookies.binarycookies")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	// File header: magic + numPages (big-endian)
	_, err = f.WriteString("cook")
	require.NoError(t, err)
	require.NoError(t, binary.Write(f, binary.BigEndian, uint32(len(pages))))

	// Page sizes (big-endian)
	for _, p := range pages {
		require.NoError(t, binary.Write(f, binary.BigEndian, uint32(len(p))))
	}

	// Page data
	for _, p := range pages {
		_, err = f.Write(p)
		require.NoError(t, err)
	}

	// Checksum (8 bytes, not validated by decoder)
	_, err = f.Write(make([]byte, 8))
	require.NoError(t, err)

	return path
}

type testCookie struct {
	domain, name, path, value string
	secure, httpOnly          bool
	expires, creation         float64 // Core Data epoch seconds
}

func buildPage(c testCookie) []byte {
	// Cookie string data: domain\0 name\0 path\0 value\0
	domain := c.domain + "\x00"
	name := c.name + "\x00"
	cpath := c.path + "\x00"
	value := c.value + "\x00"

	// Cookie binary layout (all offsets are from cookie start):
	//   size(4) + unknown1(4) + flags(4) + unknown2(4)
	//   + domainOff(4) + nameOff(4) + pathOff(4) + valueOff(4) + commentOff(4)
	//   + endHeader(4) + expires(8) + creation(8)
	//   = 56 bytes header, then string data
	const headerSize = 56
	domainOffset := uint32(headerSize)
	nameOffset := domainOffset + uint32(len(domain))
	pathOffset := nameOffset + uint32(len(name))
	valueOffset := pathOffset + uint32(len(cpath))
	cookieSize := valueOffset + uint32(len(value))

	var flags uint32
	switch {
	case c.secure && c.httpOnly:
		flags = 0x5
	case c.httpOnly:
		flags = 0x4
	case c.secure:
		flags = 0x1
	}

	// Build cookie bytes (little-endian)
	cookie := make([]byte, cookieSize)
	binary.LittleEndian.PutUint32(cookie[0:], cookieSize)
	// cookie[4:8] = unknown1 (zero)
	binary.LittleEndian.PutUint32(cookie[8:], flags)
	// cookie[12:16] = unknown2 (zero)
	binary.LittleEndian.PutUint32(cookie[16:], domainOffset)
	binary.LittleEndian.PutUint32(cookie[20:], nameOffset)
	binary.LittleEndian.PutUint32(cookie[24:], pathOffset)
	binary.LittleEndian.PutUint32(cookie[28:], valueOffset)
	// cookie[32:36] = commentOffset (zero = no comment)
	// cookie[36:40] = endHeader marker (zero)
	binary.LittleEndian.PutUint64(cookie[40:], math.Float64bits(c.expires))
	binary.LittleEndian.PutUint64(cookie[48:], math.Float64bits(c.creation))
	copy(cookie[domainOffset:], domain)
	copy(cookie[nameOffset:], name)
	copy(cookie[pathOffset:], cpath)
	copy(cookie[valueOffset:], value)

	// Page layout: marker(4) + cookieCount(4) + offsets(4*N) + endMarker(4) + cookies
	const pageHeaderSize = 16 // marker + count + 1 offset + end marker
	page := make([]byte, pageHeaderSize+len(cookie))
	copy(page[0:4], []byte{0x00, 0x00, 0x01, 0x00}) // page start marker
	binary.LittleEndian.PutUint32(page[4:], 1)      // 1 cookie
	binary.LittleEndian.PutUint32(page[8:], pageHeaderSize)
	// page[12:16] = page end marker (zero)
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

	cookies, err := extractCookies(path)
	require.NoError(t, err)
	require.Len(t, cookies, 2)

	// Sorted by CreatedAt descending (newest first)
	assert.Equal(t, ".go.dev", cookies[0].Host)
	assert.Equal(t, ".example.com", cookies[1].Host)

	// Verify field mapping
	c := cookies[1] // .example.com cookie
	assert.Equal(t, "session", c.Name)
	assert.Equal(t, "abc123", c.Value)
	assert.Equal(t, "/", c.Path)
	assert.True(t, c.IsSecure)
	assert.True(t, c.IsHTTPOnly)
	assert.False(t, c.CreatedAt.IsZero())
	assert.False(t, c.ExpireAt.IsZero())
}

func TestCountCookies(t *testing.T) {
	path := buildTestBinaryCookies(t, []testCookie{
		{domain: ".a.com", name: "a", path: "/", value: "1", expires: 2000000000.0, creation: 700000000.0},
		{domain: ".b.com", name: "b", path: "/", value: "2", expires: 2000000000.0, creation: 700000000.0},
		{domain: ".c.com", name: "c", path: "/", value: "3", expires: 2000000000.0, creation: 700000000.0},
	})

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestExtractCookies_InvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.binarycookies")
	require.NoError(t, os.WriteFile(path, []byte("not a cookies file"), 0o644))

	_, err := extractCookies(path)
	assert.Error(t, err)
}

func TestExtractCookies_FileNotFound(t *testing.T) {
	_, err := extractCookies("/nonexistent/Cookies.binarycookies")
	assert.Error(t, err)
}
