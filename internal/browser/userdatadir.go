package browser

// DiscoverOptions is everything discovery would otherwise read from the environment.
type DiscoverOptions struct {
	HomeDir     string // the user's home directory; tests pass a temp dir
	ProfilePath string // the -p value; "" when not given
}

// UserDataDirID is Browser.Name, then "-2", "-3"... in Path order; its own type keeps it apart from that name.
type UserDataDirID string

// UserDataDir is one browser data directory found on this machine; ID also names its snapshot and archive directory.
type UserDataDir struct {
	ID       UserDataDirID
	Browser  Browser
	Path     string
	Profiles []Profile
}

// Profile is one profile within a user data dir. Name is the output's profile value and never a path.
type Profile struct {
	Name         string
	RelativePath string // slash-separated, relative to UserDataDir.Path; "" when the user data dir itself is the profile
}
