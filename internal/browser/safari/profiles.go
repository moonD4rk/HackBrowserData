package safari

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// SafariTabs.db lists profiles in bookmarks rows with subtype=2. external_uuid "DefaultProfile"
// is the sentinel for the implicit default, which has no per-UUID directory.
const (
	safariTabsDBRelPath    = containerDir + "/Safari/SafariTabs.db"
	safariProfileSubtype   = 2
	defaultProfileSentinel = "DefaultProfile"
)

// Path-unsafe bytes for filenames/CSV values; Unicode letters (CJK etc.) survive.
var unsafeNameChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]+`)

// Canonical 8-4-4-4-12 hex UUID — format check only, no semantic parse.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// namedProfiles reads SafariTabs.db (authoritative), with a ReadDir fallback only if the DB itself is unreadable.
func namedProfiles(library string) []browser.Profile {
	named, err := readNamedProfilesFromDB(library)
	if err != nil {
		// Empty DB (nil, nil) is authoritative; fall back only when DB itself is unreadable.
		named = readNamedProfilesFromDir(library)
	}
	return named
}

// readNamedProfilesFromDB returns (nil, err) when the DB is missing/unreadable so the caller can
// try the ReadDir fallback; (slice, nil) — possibly empty — is authoritative.
// Discovery reads a snapshot copy, never the live file Safari has open.
func readNamedProfilesFromDB(library string) ([]browser.Profile, error) {
	d, err := snapshot.Temp()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := d.Close(); err != nil {
			log.Warnf("%v", err)
		}
	}()
	if err := d.Capture(context.Background(), []snapshot.File{{DirID: discoverDirID, From: library, Rel: safariTabsDBRelPath}}); err != nil {
		return nil, fmt.Errorf("copy SafariTabs.db: %w", err)
	}
	path, err := d.Snapshot(discoverDirID).Path(safariTabsDBRelPath)
	if err != nil {
		return nil, fmt.Errorf("copy SafariTabs.db: %w", err)
	}

	// Immutable, as on the live file before: the copied -wal is not replayed, so the profile list stays today's.
	dsn := "file:" + path + "?mode=ro&immutable=1"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SafariTabs.db: %w", err)
	}
	defer db.Close()

	// Ping forces connection; sql.Open is lazy and won't detect a missing file.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping SafariTabs.db: %w", err)
	}

	rows, err := db.Query(
		`SELECT external_uuid, title FROM bookmarks WHERE subtype = ? AND external_uuid != ?`,
		safariProfileSubtype, defaultProfileSentinel,
	)
	if err != nil {
		return nil, fmt.Errorf("query SafariTabs.db: %w", err)
	}
	defer rows.Close()

	var out []browser.Profile
	for rows.Next() {
		var externalUUID, title sql.NullString
		if err := rows.Scan(&externalUUID, &title); err != nil {
			log.Debugf("safari profiles: scan row: %v", err)
			continue
		}
		if !isCanonicalUUID(externalUUID.String) {
			continue
		}
		out = append(out, newNamedProfile(externalUUID.String, title.String))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SafariTabs.db rows: %w", err)
	}
	return out, nil
}

// readNamedProfilesFromDir is the fallback for missing SafariTabs.db. Names are synthesized from UUIDs.
func readNamedProfilesFromDir(library string) []browser.Profile {
	entries, err := os.ReadDir(filepath.Join(library, filepath.FromSlash(namedProfilesDir)))
	if err != nil {
		return nil
	}

	var out []browser.Profile
	for _, e := range entries {
		if !e.IsDir() || !isCanonicalUUID(e.Name()) {
			continue
		}
		out = append(out, newNamedProfile(e.Name(), ""))
	}
	return out
}

// A named profile's Dir is its own History.db directory, so its UUID travels with it to Extractors.
func newNamedProfile(upperUUID, title string) browser.Profile {
	return browser.Profile{Name: resolveProfileName(title, upperUUID), Dir: namedProfilesDir + "/" + upperUUID}
}

func isCanonicalUUID(s string) bool { return uuidPattern.MatchString(s) }

// resolveProfileName prefers the SafariTabs.db title, falling back to "profile-<uuid[:8]>".
func resolveProfileName(title, upperUUID string) string {
	if name := sanitizeProfileName(title); name != "" {
		return name
	}
	return "profile-" + strings.ToLower(upperUUID[:8])
}

func sanitizeProfileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return unsafeNameChars.ReplaceAllString(name, "_")
}

// disambiguateNames appends "-2", "-3", … to duplicate names, in place.
func disambiguateNames(profiles []browser.Profile) {
	occurrences := make(map[string]int, len(profiles))
	for i := range profiles {
		original := profiles[i].Name
		if prior := occurrences[original]; prior > 0 {
			profiles[i].Name = fmt.Sprintf("%s-%d", original, prior+1)
		}
		occurrences[original]++
	}
}
