package safari

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
	"github.com/moond4rk/hackbrowserdata/internal/sqlitex"
)

// Profiles are SafariTabs.db bookmarks rows with subtype=2; "DefaultProfile" is the default, which has no UUID dir.
const (
	tabsDatabasePath       = containerDir + "/Safari/SafariTabs.db"
	profileSubtype         = 2
	defaultProfileSentinel = "DefaultProfile"
)

// unsafeNameChars are path-unsafe bytes for filenames and CSV values; Unicode letters (CJK etc.) survive.
var unsafeNameChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]+`)

// uuidPattern only checks the canonical 8-4-4-4-12 hex form; it does not parse the UUID.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// namedProfiles reads SafariTabs.db (authoritative), with a ReadDir fallback only if the DB itself is unreadable.
func namedProfiles(library string) []browser.Profile {
	named, err := readNamedProfilesFromDB(library)
	if err != nil {
		named = readNamedProfilesFromDir(library)
	}
	return named
}

// readNamedProfilesFromDB reads a copy, never Safari's live file; (nil, err) means unreadable, and any slice is final.
func readNamedProfilesFromDB(library string) ([]browser.Profile, error) {
	root, err := snapshot.Temp()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := root.Close(); err != nil {
			log.Warnf("%v", err)
		}
	}()
	if err := root.Capture(context.Background(), []snapshot.File{{
		UserDataDirID: discoveryUserDataDirID, UserDataDirPath: library, RelativePath: tabsDatabasePath,
	}}); err != nil {
		return nil, fmt.Errorf("copy SafariTabs.db: %w", err)
	}
	path, err := root.Snapshot(discoveryUserDataDirID).Path(tabsDatabasePath)
	if err != nil {
		return nil, fmt.Errorf("copy SafariTabs.db: %w", err)
	}

	// Immutable, as on the live file before: the copied -wal is not replayed, so the profile list stays today's.
	database, err := sqlitex.Open(path, sqlitex.ReadOnly, sqlitex.Immutable)
	if err != nil {
		return nil, fmt.Errorf("open SafariTabs.db: %w", err)
	}
	defer database.Close()

	rows, err := database.Query(
		`SELECT external_uuid, title FROM bookmarks WHERE subtype = ? AND external_uuid != ?`,
		profileSubtype, defaultProfileSentinel,
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

// readNamedProfilesFromDir synthesizes names from UUIDs when SafariTabs.db is missing.
func readNamedProfilesFromDir(library string) []browser.Profile {
	entries, err := os.ReadDir(filepath.Join(library, filepath.FromSlash(namedProfilesDir)))
	if err != nil {
		return nil
	}

	var out []browser.Profile
	for _, entry := range entries {
		if !entry.IsDir() || !isCanonicalUUID(entry.Name()) {
			continue
		}
		out = append(out, newNamedProfile(entry.Name(), ""))
	}
	return out
}

// newNamedProfile sets RelativePath to the profile's own History.db directory, so its UUID travels with it to Extractors.
func newNamedProfile(upperUUID, title string) browser.Profile {
	return browser.Profile{Name: resolveProfileName(title, upperUUID), RelativePath: namedProfilesDir + "/" + upperUUID}
}

func isCanonicalUUID(value string) bool { return uuidPattern.MatchString(value) }

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
