package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat  Category
		want string
	}{
		{Password, "password"},
		{Cookie, "cookie"},
		{Bookmark, "bookmark"},
		{History, "history"},
		{Download, "download"},
		{CreditCard, "creditcard"},
		{Extension, "extension"},
		{LocalStorage, "localstorage"},
		{SessionStorage, "sessionstorage"},
		{Category(999), "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.cat.String())
	}
}

func TestCategory_IsSensitive(t *testing.T) {
	sensitive := []Category{Password, Cookie, CreditCard}
	for _, c := range sensitive {
		assert.True(t, c.IsSensitive(), "%s should be sensitive", c)
	}

	notSensitive := []Category{Bookmark, History, Download, Extension, LocalStorage, SessionStorage}
	for _, c := range notSensitive {
		assert.False(t, c.IsSensitive(), "%s should not be sensitive", c)
	}
}

func TestAllCategories(t *testing.T) {
	assert.Len(t, AllCategories, 9)
}

func TestNonSensitiveCategories(t *testing.T) {
	cats := NonSensitiveCategories()
	assert.Len(t, cats, 6)
	for _, c := range cats {
		assert.False(t, c.IsSensitive())
	}
}
