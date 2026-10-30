package export

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

const tiedRecords = 20

func day(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }

func TestSortRecords_NewestFirst(t *testing.T) {
	logins := []record.Record{
		&record.Login{URL: "old", CreatedAt: day(1)},
		&record.Login{URL: "unset"},
		&record.Login{URL: "new", CreatedAt: day(3)},
	}
	sortRecords(logins)
	assert.Equal(t, []string{"new", "old", "unset"}, urlsOf(t, logins))

	downloads := []record.Record{
		&record.Download{URL: "older", StartTime: day(1)},
		&record.Download{URL: "newer", StartTime: day(2)},
	}
	sortRecords(downloads)
	assert.Equal(t, []string{"newer", "older"}, urlsOf(t, downloads))
}

// Firefox used to write its history least visited first; the exporter now orders history the same for every browser.
func TestSortRecords_HistoryMostVisitedFirst(t *testing.T) {
	histories := []record.Record{
		&record.History{URL: "once", VisitCount: 1},
		&record.History{URL: "often", VisitCount: 42},
		&record.History{URL: "twice", VisitCount: 2},
	}
	sortRecords(histories)
	assert.Equal(t, []string{"often", "twice", "once"}, urlsOf(t, histories))
}

func TestSortRecords_TiesKeepSourceOrder(t *testing.T) {
	logins := make([]record.Record, 0, tiedRecords)
	for i := 0; i < tiedRecords; i++ {
		store := record.StoreLocal
		if i >= tiedRecords/2 {
			store = record.StoreAccount
		}
		logins = append(logins, &record.Login{URL: strconv.Itoa(i), CreatedAt: day(2), Store: store})
	}
	want := append([]record.Record(nil), logins...)
	sortRecords(logins)
	assert.Equal(t, want, logins, "equal timestamps keep emission order, so local rows stay before account rows")

	bookmarks := make([]record.Record, 0, tiedRecords)
	for i := 0; i < tiedRecords; i++ {
		bookmarks = append(bookmarks, &record.Bookmark{ID: int64(tiedRecords - i)})
	}
	want = append([]record.Record(nil), bookmarks...)
	sortRecords(bookmarks)
	assert.Equal(t, want, bookmarks, "bookmarks without a creation time keep their tree order")
}

// Interleaved timestamps make the sort move records; an unstable sort would then reorder the ties it moves.
func TestSortRecords_IsStable(t *testing.T) {
	logins := make([]record.Record, 0, tiedRecords)
	var newer, older []string
	for i := 0; i < tiedRecords; i++ {
		url := strconv.Itoa(i)
		created := day(1)
		if i%2 == 1 {
			created = day(2)
			newer = append(newer, url)
		} else {
			older = append(older, url)
		}
		logins = append(logins, &record.Login{URL: url, CreatedAt: created})
	}
	sortRecords(logins)
	assert.Equal(t, append(newer, older...), urlsOf(t, logins))
}

func TestSortRecords_KeylessCategoriesKeepSourceOrder(t *testing.T) {
	records := make([]record.Record, 0, 4*tiedRecords)
	for i := 0; i < tiedRecords; i++ {
		id := strconv.Itoa(i)
		records = append(records,
			&record.CreditCard{GUID: id}, &record.Extension{ID: id},
			&record.LocalStorage{Key: id}, &record.SessionStorage{Key: id},
		)
	}
	want := append([]record.Record(nil), records...)
	sortRecords(records)
	assert.Equal(t, want, records)
}

func urlsOf(t *testing.T, records []record.Record) []string {
	t.Helper()
	urls := make([]string, 0, len(records))
	for _, r := range records {
		switch r := r.(type) {
		case *record.Login:
			urls = append(urls, r.URL)
		case *record.Download:
			urls = append(urls, r.URL)
		case *record.History:
			urls = append(urls, r.URL)
		default:
			t.Fatalf("unexpected record %T", r)
		}
	}
	return urls
}
