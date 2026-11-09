package export

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

const tiedRecords = 20

func day(dayOfMonth int) time.Time { return time.Date(2026, 1, dayOfMonth, 0, 0, 0, 0, time.UTC) }

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

	olderCookie := &record.Cookie{Name: "older", CreatedAt: day(1)}
	newerCookie := &record.Cookie{Name: "newer", CreatedAt: day(2)}
	cookies := []record.Record{olderCookie, newerCookie}
	sortRecords(cookies)
	assert.Equal(t, []record.Record{newerCookie, olderCookie}, cookies)

	olderBookmark := &record.Bookmark{Name: "older", CreatedAt: day(1)}
	newerBookmark := &record.Bookmark{Name: "newer", CreatedAt: day(2)}
	bookmarks := []record.Record{olderBookmark, newerBookmark}
	sortRecords(bookmarks)
	assert.Equal(t, []record.Record{newerBookmark, olderBookmark}, bookmarks)
}

// TestSortRecords_HistoryMostVisitedFirst orders history alike for every browser, whatever order its database returns.
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

// TestSortRecords_IsStable interleaves timestamps so the sort moves records, which an unstable sort would reorder.
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
	for _, row := range records {
		switch typed := row.(type) {
		case *record.Login:
			urls = append(urls, typed.URL)
		case *record.Download:
			urls = append(urls, typed.URL)
		case *record.History:
			urls = append(urls, typed.URL)
		default:
			t.Fatalf("unexpected record %T", typed)
		}
	}
	return urls
}
