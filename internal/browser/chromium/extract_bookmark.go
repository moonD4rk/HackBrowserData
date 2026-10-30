package chromium

import (
	"os"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/timeconv"
)

func extractBookmarks(path string, out browser.Collector) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var bookmarks []record.Bookmark
	roots := gjson.GetBytes(data, "roots")
	roots.ForEach(func(_, value gjson.Result) bool {
		walkBookmarks(value, "", &bookmarks)
		return true
	})
	emit(out, bookmarks)
	return nil
}

// walkBookmarks recursively traverses the bookmark tree, collecting URL entries.
func walkBookmarks(node gjson.Result, folder string, out *[]record.Bookmark) {
	nodeType := node.Get("type").String()
	if nodeType == "url" {
		*out = append(*out, record.Bookmark{
			ID:        node.Get("id").Int(),
			Name:      node.Get("name").String(),
			Type:      nodeType,
			URL:       node.Get("url").String(),
			Folder:    folder,
			CreatedAt: timeconv.Chromium(node.Get("date_added").Int()),
		})
	}

	children := node.Get("children")
	if !children.Exists() || !children.IsArray() {
		return
	}
	currentFolder := node.Get("name").String()
	for _, child := range children.Array() {
		walkBookmarks(child, currentFolder, out)
	}
}
