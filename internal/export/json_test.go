package export

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func encodeOnce(t *testing.T, value any, escapeHTML bool) string {
	t.Helper()
	var buffer bytes.Buffer
	referenceEncoder := json.NewEncoder(&buffer)
	referenceEncoder.SetIndent("", jsonIndent)
	referenceEncoder.SetEscapeHTML(escapeHTML)
	require.NoError(t, referenceEncoder.Encode(value))
	return buffer.String()
}

func encodeFramed(t *testing.T, framedEncoder *jsonEncoder, records []record.Record) string {
	t.Helper()
	var buffer bytes.Buffer
	framedEncoder.writer = &buffer
	for _, row := range records {
		require.NoError(t, framedEncoder.encode(row))
	}
	require.NoError(t, framedEncoder.end())
	return buffer.String()
}

func TestJSONEncoder_MatchesOneEncodeOfTheSlice(t *testing.T) {
	cookies := []record.Record{
		&record.Cookie{
			Browser: "Chrome", Profile: "Default", Host: ".example.com", Path: "/", Name: "q",
			Value: `https://shop.example.com/cart?item=42&qty=<3> "日本語" ` + "\u2028", CreatedAt: testTime, SameSite: "none",
		},
		&record.Cookie{Browser: "Chrome", Profile: "Default", Host: "example.com", Name: "empty", ExpireAt: testTime},
	}

	t.Run("json escapes HTML", func(t *testing.T) {
		got := encodeFramed(t, &jsonEncoder{escapeHTML: true}, cookies)
		assert.Equal(t, encodeOnce(t, cookies, true), got)
		assert.Contains(t, got, `\u0026qty=\u003c3\u003e`)
	})

	t.Run("cookie-editor does not", func(t *testing.T) {
		entries := make([]any, 0, len(cookies))
		for _, cookie := range cookies {
			entries = append(entries, toCookieEditor(cookie))
		}
		got := encodeFramed(t, &jsonEncoder{convert: toCookieEditor}, cookies)
		assert.Equal(t, encodeOnce(t, entries, false), got)
		assert.Contains(t, got, `&qty=<3>`)
	})

	t.Run("one record", func(t *testing.T) {
		assert.Equal(t, encodeOnce(t, cookies[:1], true), encodeFramed(t, &jsonEncoder{escapeHTML: true}, cookies[:1]))
	})
}
