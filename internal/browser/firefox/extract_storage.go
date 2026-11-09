package firefox

import (
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

const localStorageQuery = `SELECT originKey, key, value FROM webappsstore2`

func extractLocalStorage(path string, out browser.Collector) error {
	database, err := sqlitex.Open(path, sqlitex.JournalOff)
	if err != nil {
		return err
	}
	defer database.Close()

	rows, err := database.Query(localStorageQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var originKey, key, value string
		if err := rows.Scan(&originKey, &key, &value); err != nil {
			out.Fail(err)
			continue
		}
		out.Add(&record.LocalStorage{
			URL:   parseOriginKey(originKey),
			Key:   key,
			Value: value,
		})
	}
	return rows.Err()
}

func reverseString(text string) string {
	reversed := []byte(text)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return string(reversed)
}

// parseOriginKey undoes Firefox's reversed host: "moc.buhtig.:https:443" is https://github.com:443.
func parseOriginKey(originKey string) string {
	parts := strings.SplitN(originKey, ":", 3)
	if len(parts) < 2 {
		return originKey
	}
	host := reverseString(parts[0])
	host = strings.TrimPrefix(host, ".")
	scheme := parts[1]
	if len(parts) == 3 {
		return fmt.Sprintf("%s://%s:%s", scheme, host, parts[2])
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
