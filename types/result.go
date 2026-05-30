package types

// Profile identifies one browser profile — a leaf under an installation.
type Profile struct {
	Name string
	Dir  string
}

// ExtractResult pairs a profile with the data extracted from it.
type ExtractResult struct {
	Profile
	Data *BrowserData
}

// CountResult pairs a profile with its per-category entry counts.
type CountResult struct {
	Profile
	Counts map[Category]int
}
