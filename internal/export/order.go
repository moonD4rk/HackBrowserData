package export

import (
	"sort"
	"time"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// sortRecords is stable, so ties and keyless records keep source order: Safari bookmarks and downloads lack a time.
func sortRecords(records []record.Record) {
	sort.SliceStable(records, func(i, j int) bool {
		return keyOf(records[i]).before(keyOf(records[j]))
	})
}

type sortKey struct {
	visits    int
	timestamp time.Time
}

func keyOf(row record.Record) sortKey {
	switch typed := row.(type) {
	case *record.Login:
		return sortKey{timestamp: typed.CreatedAt}
	case *record.Cookie:
		return sortKey{timestamp: typed.CreatedAt}
	case *record.Bookmark:
		return sortKey{timestamp: typed.CreatedAt}
	case *record.Download:
		return sortKey{timestamp: typed.StartTime}
	case *record.History:
		return sortKey{visits: typed.VisitCount}
	default:
		return sortKey{}
	}
}

func (k sortKey) before(other sortKey) bool {
	if k.visits != other.visits {
		return k.visits > other.visits
	}
	return k.timestamp.After(other.timestamp)
}
