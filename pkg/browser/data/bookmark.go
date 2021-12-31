package data

import (
	"sort"

	"github.com/tidwall/gjson"

	"hack-browser-data/pkg/browser/consts"
	"hack-browser-data/utils"
)

type ChromiumBookmark []bookmark

func (c *ChromiumBookmark) Parse(masterKey []byte) error {
	bookmarks, err := utils.ReadFile(consts.ChromiumBookmarkFilename)
	if err != nil {
		return err
	}
	r := gjson.Parse(bookmarks)
	if r.Exists() {
		roots := r.Get("roots")
		roots.ForEach(func(key, value gjson.Result) bool {
			getBookmarkChildren(value, c)
			return true
		})
	}
	sort.Slice(*c, func(i, j int) bool {
		return (*c)[i].DateAdded.After((*c)[j].DateAdded)
	})
	return nil
}

func getBookmarkChildren(value gjson.Result, w *ChromiumBookmark) (children gjson.Result) {
	const (
		bookmarkID       = "id"
		bookmarkAdded    = "date_added"
		bookmarkUrl      = "url"
		bookmarkName     = "name"
		bookmarkType     = "type"
		bookmarkChildren = "children"
	)
	nodeType := value.Get(bookmarkType)
	bm := bookmark{
		ID:        value.Get(bookmarkID).Int(),
		Name:      value.Get(bookmarkName).String(),
		URL:       value.Get(bookmarkUrl).String(),
		DateAdded: utils.TimeEpochFormat(value.Get(bookmarkAdded).Int()),
	}
	children = value.Get(bookmarkChildren)
	if nodeType.Exists() {
		bm.Type = nodeType.String()
		*w = append(*w, bm)
		if children.Exists() && children.IsArray() {
			for _, v := range children.Array() {
				children = getBookmarkChildren(v, w)
			}
		}
	}
	return children
}

func (c *ChromiumBookmark) Name() string {
	return "bookmark"
}
