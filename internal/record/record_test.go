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
	for _, dataCategory := range category.All() {
		row, ok := records[dataCategory]
		if assert.Truef(t, ok, "no record type for %s", dataCategory) {
			assert.Equalf(t, dataCategory, row.Category(), "%T", row)
		}
	}
}

func TestSetBrowserAndProfile(t *testing.T) {
	for _, row := range allRecords() {
		row.SetBrowserAndProfile("Microsoft Edge", "Profile 1")
		value := reflect.ValueOf(row).Elem()
		assert.Equalf(t, "Microsoft Edge", value.FieldByName("Browser").String(), "%T", row)
		assert.Equalf(t, "Profile 1", value.FieldByName("Profile").String(), "%T", row)
	}
}

// TestTags_AreSnakeCaseFieldNames freezes the tags, the output's names: snake_case, led by Browser and Profile.
func TestTags_AreSnakeCaseFieldNames(t *testing.T) {
	for _, row := range allRecords() {
		recordType := reflect.TypeOf(row).Elem()
		assert.Equalf(t, "Browser", recordType.Field(0).Name, "%s", recordType)
		assert.Equalf(t, "Profile", recordType.Field(1).Name, "%s", recordType)
		for i := 0; i < recordType.NumField(); i++ {
			field := recordType.Field(i)
			want := snakeCase(field.Name)
			assert.Equalf(t, want, field.Tag.Get("json"), "%s.%s json tag", recordType, field.Name)
			assert.Equalf(t, want, field.Tag.Get("csv"), "%s.%s csv tag", recordType, field.Name)
		}
	}
}

func snakeCase(name string) string {
	runes := []rune(name)
	var builder strings.Builder
	for i, character := range runes {
		if i > 0 && unicode.IsUpper(character) {
			prevLower := unicode.IsLower(runes[i-1])
			acronymEnds := unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevLower || acronymEnds {
				builder.WriteByte('_')
			}
		}
		builder.WriteRune(unicode.ToLower(character))
	}
	return builder.String()
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
