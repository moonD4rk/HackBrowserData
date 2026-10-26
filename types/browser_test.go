package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserKind_String(t *testing.T) {
	tests := []struct {
		kind BrowserKind
		want string
	}{
		{Chromium, "chromium"},
		{ChromiumYandex, "chromium-yandex"},
		{ChromiumOpera, "chromium-opera"},
		{Firefox, "firefox"},
		{Safari, "safari"},
		{BrowserKind(999), "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.kind.String())
	}
}
