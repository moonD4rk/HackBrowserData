package chromium

import (
	"os"
	"sort"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

func extractBookmarks(path string) ([]types.BookmarkEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var bookmarks []types.BookmarkEntry
	roots := gjson.GetBytes(data, "roots")
	roots.ForEach(func(_, value gjson.Result) bool {
		walkBookmarks(value, "", &bookmarks)
		return true
	})

	sort.Slice(bookmarks, func(i, j int) bool {
		return bookmarks[i].CreatedAt.After(bookmarks[j].CreatedAt)
	})
	return bookmarks, nil
}

// walkBookmarks recursively traverses the bookmark tree, collecting URL entries.
func walkBookmarks(node gjson.Result, folder string, out *[]types.BookmarkEntry) {
	if node.Get("type").String() == "url" {
		*out = append(*out, types.BookmarkEntry{
			Name:      node.Get("name").String(),
			URL:       node.Get("url").String(),
			Folder:    folder,
			CreatedAt: typeutil.TimeEpoch(node.Get("date_added").Int()),
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
