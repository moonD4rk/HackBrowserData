package safari

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/moond4rk/hackbrowserdata/log"
)

// profileContext tracks the uppercase (Safari/Profiles/<UUID>) and lowercase
// (WebKit/WebsiteDataStore/<uuid>) UUID forms a named profile needs. Both empty ⇒ default profile.
type profileContext struct {
	name       string
	uuidUpper  string
	uuidLower  string
	legacyHome string // ~/Library/Safari
	container  string // ~/Library/Containers/com.apple.Safari/Data/Library
}

func (p profileContext) isDefault() bool { return p.uuidUpper == "" }

// downloadOwnerUUID is the value Safari writes into DownloadEntryProfileUUIDStringKey
// for downloads that belong to this profile. The default profile uses the sentinel
// "DefaultProfile"; named profiles use their uppercase UUID.
func (p profileContext) downloadOwnerUUID() string {
	if p.isDefault() {
		return defaultProfileSentinel
	}
	return p.uuidUpper
}

// SafariTabs.db lists profiles in bookmarks rows with subtype=2. external_uuid "DefaultProfile"
// is the sentinel for the implicit default, which has no per-UUID directory.
const (
	safariTabsDBRelPath    = "Safari/SafariTabs.db"
	safariProfileSubtype   = 2
	defaultProfileSentinel = "DefaultProfile"
)

// Path-unsafe bytes for filenames/CSV values; Unicode letters (CJK etc.) survive.
var unsafeNameChars = regexp.MustCompile(`[/\\:*?"<>|\x00-\x1f]+`)

// Canonical 8-4-4-4-12 hex UUID — format check only, no semantic parse.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// discoverSafariProfiles always lists the default first, then named profiles from SafariTabs.db
// (authoritative) with a ReadDir fallback only if the DB itself is unreadable.
func discoverSafariProfiles(legacyHome string) []profileContext {
	container := deriveContainerRoot(legacyHome)

	profiles := []profileContext{{
		name:       "default",
		legacyHome: legacyHome,
		container:  container,
	}}

	named, err := readNamedProfilesFromDB(container)
	if err != nil {
		// Empty DB (nil, nil) is authoritative; fall back only when DB itself is unreadable.
		named = readNamedProfilesFromDir(container)
	}
	for _, p := range named {
		p.legacyHome = legacyHome
		p.container = container
		profiles = append(profiles, p)
	}

	disambiguateNames(profiles)
	return profiles
}

func deriveContainerRoot(legacyHome string) string {
	return filepath.Join(filepath.Dir(legacyHome), "Containers", "com.apple.Safari", "Data", "Library")
}

// readNamedProfilesFromDB returns (nil, err) when the DB is missing/unreadable so the caller can
// try the ReadDir fallback; (slice, nil) — possibly empty — is authoritative.
func readNamedProfilesFromDB(container string) ([]profileContext, error) {
	// Read-only + immutable so we don't disturb Safari's live WAL.
	dsn := "file:" + filepath.Join(container, safariTabsDBRelPath) + "?mode=ro&immutable=1"
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

	var out []profileContext
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
func readNamedProfilesFromDir(container string) []profileContext {
	entries, err := os.ReadDir(filepath.Join(container, "Safari", "Profiles"))
	if err != nil {
		return nil
	}

	var out []profileContext
	for _, e := range entries {
		if !e.IsDir() || !isCanonicalUUID(e.Name()) {
			continue
		}
		out = append(out, newNamedProfile(e.Name(), ""))
	}
	return out
}

func newNamedProfile(upperUUID, title string) profileContext {
	return profileContext{
		name:      resolveProfileName(title, upperUUID),
		uuidUpper: upperUUID,
		uuidLower: strings.ToLower(upperUUID),
	}
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
func disambiguateNames(profiles []profileContext) {
	occurrences := make(map[string]int, len(profiles))
	for i := range profiles {
		original := profiles[i].name
		if prior := occurrences[original]; prior > 0 {
			profiles[i].name = fmt.Sprintf("%s-%d", original, prior+1)
		}
		occurrences[original]++
	}
}
