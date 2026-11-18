package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func TestCapStorageValue(t *testing.T) {
	short := &record.LocalStorage{Value: strings.Repeat("x", 10)}
	meta := &record.LocalStorage{Value: strings.Repeat("y", 100), IsMeta: true}
	session := &record.SessionStorage{Value: strings.Repeat("z", 100)}
	cookie := &record.Cookie{Value: strings.Repeat("c", 100)}
	for _, row := range []record.Record{short, meta, session, cookie} {
		capStorageValue(row, 50)
	}

	assert.Equal(t, strings.Repeat("x", 10), short.Value, "under limit is kept")
	assert.Equal(t, strings.Repeat("y", 100), meta.Value, "meta is never capped")
	assert.Contains(t, session.Value, "value too long", "over limit is marked")
	assert.Equal(t, strings.Repeat("c", 100), cookie.Value, "non-storage is untouched")
}

func TestCapStorageValue_BoundaryAndUnlimited(t *testing.T) {
	exact := &record.LocalStorage{Value: strings.Repeat("x", 50)}
	capStorageValue(exact, 50)
	assert.Equal(t, strings.Repeat("x", 50), exact.Value, "a value of exactly the limit is kept")

	unlimited := &record.LocalStorage{Value: strings.Repeat("x", 5000)}
	capStorageValue(unlimited, 0)
	assert.Len(t, unlimited.Value, 5000, "limit <= 0 keeps every value")
}

func TestCappedValue_CountsRunesNotBytes(t *testing.T) {
	// 40 CJK runes decode fine but span 120 UTF-8 bytes; the limit is runes, so this stays under a 50 limit.
	value := strings.Repeat("世", 40)
	assert.Equal(t, value, cappedValue(value, 50))
}
