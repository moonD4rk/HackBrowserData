package chromium

import (
	"os"
	"sort"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/types"
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
	nodeType := node.Get("type").String()
	if nodeType == "url" {
		*out = append(*out, types.BookmarkEntry{
			ID:        node.Get("id").Int(),
			Name:      node.Get("name").String(),
			Type:      nodeType,
			URL:       node.Get("url").String(),
			Folder:    folder,
			CreatedAt: timeEpoch(node.Get("date_added").Int()),
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

func countBookmarks(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var count int
	roots := gjson.GetBytes(data, "roots")
	roots.ForEach(func(_, value gjson.Result) bool {
		count += walkCountBookmarks(value)
		return true
	})
	return count, nil
}

// walkCountBookmarks recursively counts URL nodes in the bookmark tree.
func walkCountBookmarks(node gjson.Result) int {
	count := 0
	if node.Get("type").String() == "url" {
		count++
	}
	children := node.Get("children")
	if children.Exists() && children.IsArray() {
		for _, child := range children.Array() {
			count += walkCountBookmarks(child)
		}
	}
	return count
}
