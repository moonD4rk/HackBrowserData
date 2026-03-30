package firefox

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/sqliteutil"
	"github.com/moond4rk/hackbrowserdata/utils/typeutil"
)

const firefoxLocalStorageQuery = `SELECT originKey, key, value FROM webappsstore2`

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

// parseOriginKey converts Firefox's reversed origin format to a URL.
// Example: "moc.buhtig.:https:443" → "https://github.com:443"
func parseOriginKey(originKey string) string {
	parts := strings.SplitN(originKey, ":", 3)
	if len(parts) < 2 {
		return originKey
	}
	host := string(typeutil.Reverse([]byte(parts[0])))
	host = strings.TrimPrefix(host, ".")
	scheme := parts[1]
	if len(parts) == 3 {
		return fmt.Sprintf("%s://%s:%s", scheme, host, parts[2])
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}
