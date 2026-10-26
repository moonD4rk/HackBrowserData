package export

import (
	"sort"
	"time"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// Stable, so ties and keyless categories keep source order; Safari's bookmarks and downloads have no timestamp to sort by.
func sortRecords(records []record.Record) {
	sort.SliceStable(records, func(i, j int) bool {
		return keyOf(records[i]).before(keyOf(records[j]))
	})
}

type sortKey struct {
	visits int
	at     time.Time
}

func keyOf(r record.Record) sortKey {
	switch r := r.(type) {
	case *record.Login:
		return sortKey{at: r.CreatedAt}
	case *record.Cookie:
		return sortKey{at: r.CreatedAt}
	case *record.Bookmark:
		return sortKey{at: r.CreatedAt}
	case *record.Download:
		return sortKey{at: r.StartTime}
	case *record.History:
		return sortKey{visits: r.VisitCount}
	default:
		return sortKey{}
	}
}

func (k sortKey) before(o sortKey) bool {
	if k.visits != o.visits {
		return k.visits > o.visits
	}
	return k.at.After(o.at)
}
