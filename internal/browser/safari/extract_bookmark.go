package safari

import (
	"fmt"
	"os"
	"time"

	"github.com/moond4rk/plist"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// safariBookmark mirrors the plist structure of Safari's Bookmarks.plist.
type safariBookmark struct {
	Type          string           `plist:"WebBookmarkType"`
	Title         string           `plist:"Title"`
	URLString     string           `plist:"URLString"`
	URIDictionary uriDictionary    `plist:"URIDictionary"`
	Children      []safariBookmark `plist:"Children"`
}

type uriDictionary struct {
	Title string `plist:"title"`
}

const (
	bookmarkTypeLeaf = "WebBookmarkTypeLeaf"
	bookmarkTypeList = "WebBookmarkTypeList"
)

func extractBookmarks(path string, out browser.Collector) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open bookmarks: %w", err)
	}
	defer f.Close()

	var root safariBookmark
	if err := plist.NewDecoder(f).Decode(&root); err != nil {
		return fmt.Errorf("decode bookmarks: %w", err)
	}

	var bookmarks []record.Bookmark
	walkBookmarks(root.Children, "", &bookmarks)
	emit(out, bookmarks)
	return nil
}

// walkBookmarks collects the leaves in tree order. A leaf's ID is its position among them, counting from 1, because
// the plist stores no number and an index within one folder repeats across folders.
func walkBookmarks(nodes []safariBookmark, folder string, out *[]record.Bookmark) {
	for _, node := range nodes {
		switch node.Type {
		case bookmarkTypeLeaf:
			title := node.URIDictionary.Title
			if title == "" {
				title = node.Title
			}
			if node.URLString == "" {
				continue
			}
			*out = append(*out, record.Bookmark{
				ID:        int64(len(*out) + 1),
				Name:      title,
				URL:       node.URLString,
				Folder:    folder,
				Type:      "bookmark",
				CreatedAt: time.Time{},
			})
		case bookmarkTypeList:
			name := node.Title
			if name == "com.apple.ReadingList" {
				name = "ReadingList"
			}
			walkBookmarks(node.Children, name, out)
		}
	}
}
