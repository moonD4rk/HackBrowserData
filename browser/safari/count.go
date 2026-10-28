package safari

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

// list --detail keeps today's counting queries until the switch-over, when it counts what the extractors emit.
const safariCountHistoryQuery = `SELECT COUNT(*) FROM history_items`

const (
	webkitOriginFile         = "origin"
	webkitLocalStorageSubdir = "LocalStorage"
	webkitLocalStorageDB     = "localstorage.sqlite3"
	webkitOriginSaltName     = "salt"
)

func countHistories(path string) (int, error) {
	return sqlitex.CountRows(path, true, safariCountHistoryQuery)
}

// countLocalStorage sums ItemTable row counts across every origin DB under root without
// parsing origin files or decoding values — CountEntries callers only need the total, not the
// URLs or plaintext. COUNT(key) naturally excludes NULL keys, matching the same skip rule
// applied by readLocalStorageFile, so count and extract stay in sync.
func countLocalStorage(root string) (int, error) {
	dirs, err := findOriginDataDirs(root)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, od := range dirs {
		dbPath := filepath.Join(od, webkitLocalStorageSubdir, webkitLocalStorageDB)
		n, err := countLocalStorageFile(dbPath)
		if err != nil {
			log.Debugf("safari localstorage: count %s: %v", dbPath, err)
			continue
		}
		total += n
	}
	return total, nil
}

func countLocalStorageFile(path string) (int, error) {
	// mode=ro (no immutable) so SQLite replays the copied -wal sidecar — this surfaces entries
	// Safari has committed to WAL but not yet checkpointed to the main DB. Writes SQLite might
	// make to the temp-copy's -shm during replay are harmless; the Session cleanup removes
	// everything. Live-file reads (profiles.go) still use immutable=1 to stay off the real WAL.
	dsn := "file:" + path + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", path, err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return 0, fmt.Errorf("ping %s: %w", path, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(key) FROM ItemTable`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count ItemTable: %w", err)
	}
	return count, nil
}

// findOriginDataDirs returns <root>/<h1>/<h2>/ paths that contain both an "origin" file and
// a "LocalStorage/localstorage.sqlite3" database. Non-directory entries, the "salt" sibling,
// and partition dirs without localStorage data are silently skipped.
func findOriginDataDirs(root string) ([]string, error) {
	topEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read origins root %s: %w", root, err)
	}
	var out []string
	for _, top := range topEntries {
		if !top.IsDir() || top.Name() == webkitOriginSaltName {
			continue
		}
		topPath := filepath.Join(root, top.Name())
		frameEntries, err := os.ReadDir(topPath)
		if err != nil {
			continue
		}
		for _, frame := range frameEntries {
			if !frame.IsDir() {
				continue
			}
			framePath := filepath.Join(topPath, frame.Name())
			if _, err := os.Stat(filepath.Join(framePath, webkitOriginFile)); err != nil {
				continue
			}
			dbPath := filepath.Join(framePath, webkitLocalStorageSubdir, webkitLocalStorageDB)
			if _, err := os.Stat(dbPath); err != nil {
				continue
			}
			out = append(out, framePath)
		}
	}
	return out, nil
}
