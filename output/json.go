package output

import (
	"encoding/json"
	"io"

	"github.com/moond4rk/hackbrowserdata/types"
)

// JSONFormatter writes category data as pretty-printed JSON.
// Each call appends a JSON object with browser/profile context.
type JSONFormatter struct{}

func (f *JSONFormatter) Ext() string { return "json" }

func (f *JSONFormatter) Format(w io.Writer, cd types.CategoryData, browser, profile string) error {
	wrapper := struct {
		Browser  string      `json:"browser"`
		Profile  string      `json:"profile"`
		Category string      `json:"category"`
		Data     interface{} `json:"data"`
	}{
		Browser:  browser,
		Profile:  profile,
		Category: cd.Category.String(),
		Data:     cd.Data,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(wrapper)
}
