package record

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/category"
)

func allRecords() map[category.Category]Record {
	return map[category.Category]Record{
		category.Password:       &Login{},
		category.Cookie:         &Cookie{},
		category.Bookmark:       &Bookmark{},
		category.History:        &History{},
		category.Download:       &Download{},
		category.CreditCard:     &CreditCard{},
		category.Extension:      &Extension{},
		category.LocalStorage:   &LocalStorage{},
		category.SessionStorage: &SessionStorage{},
	}
}

func TestCategory_OneRecordTypePerCategory(t *testing.T) {
	records := allRecords()
	assert.Len(t, records, len(category.All()))
	for _, c := range category.All() {
		r, ok := records[c]
		if assert.Truef(t, ok, "no record type for %s", c) {
			assert.Equalf(t, c, r.Category(), "%T", r)
		}
	}
}

func TestStamp(t *testing.T) {
	for _, r := range allRecords() {
		Stamp(r, "Microsoft Edge", "Profile 1")
		v := reflect.ValueOf(r).Elem()
		assert.Equalf(t, "Microsoft Edge", v.FieldByName("Browser").String(), "%T", r)
		assert.Equalf(t, "Profile 1", v.FieldByName("Profile").String(), "%T", r)
	}
}

// The tags are the output's field names and column headers, so they stay snake_case(field name) and lead with the stamp.
func TestTags_AreSnakeCaseFieldNames(t *testing.T) {
	for _, r := range allRecords() {
		typ := reflect.TypeOf(r).Elem()
		assert.Equalf(t, "Browser", typ.Field(0).Name, "%s", typ)
		assert.Equalf(t, "Profile", typ.Field(1).Name, "%s", typ)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			want := snakeCase(f.Name)
			assert.Equalf(t, want, f.Tag.Get("json"), "%s.%s json tag", typ, f.Name)
			assert.Equalf(t, want, f.Tag.Get("csv"), "%s.%s csv tag", typ, f.Name)
		}
	}
}

func snakeCase(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prevLower := unicode.IsLower(runes[i-1])
			acronymEnds := unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevLower || acronymEnds {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func TestSnakeCase(t *testing.T) {
	for name, want := range map[string]string{
		"URL":         "url",
		"IsHTTPOnly":  "is_http_only",
		"HomepageURL": "homepage_url",
		"NickName":    "nick_name",
		"GUID":        "guid",
	} {
		assert.Equal(t, want, snakeCase(name))
	}
}
