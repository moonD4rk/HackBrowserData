package firefox

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
)

const (
	firefoxLocalStorageQuery      = `SELECT originKey, key, value FROM webappsstore2`
	firefoxCountLocalStorageQuery = `SELECT COUNT(*) FROM webappsstore2`
)

func extractLocalStorage(path string) ([]types.StorageEntry, error) {
	return sqliteutil.QueryRows(path, true, firefoxLocalStorageQuery,
		func(rows *sql.Rows) (types.StorageEntry, error) {
			var originKey, key, value string
			if err := rows.Scan(&originKey, &key, &value); err != nil {
				return types.StorageEntry{}, err
			}
			return types.StorageEntry{
				URL:   parseOriginKey(originKey),
				Key:   key,
				Value: value,
			}, nil
		})
}

func countLocalStorage(path string) (int, error) {
	return sqliteutil.CountRows(path, true, firefoxCountLocalStorageQuery)
}

func reverseString(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// parseOriginKey converts Firefox's reversed origin format to a URL.
// Example: "moc.buhtig.:https:443" → "https://github.com:443"
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
