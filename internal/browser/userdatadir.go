package browser

// DiscoverOptions is everything discovery would otherwise read from the environment.
type DiscoverOptions struct {
	Home        string // the user's home directory; tests pass a temp dir
	ProfilePath string // the -p value; "" when not given
}

// DirID names one user data dir: Def.ID for the first one of a Def, then "-2", "-3"… in Path order. Its own type
// keeps it from being passed where a Def.ID belongs: the two are equal on a machine with one Arc.
type DirID string

// UserDataDir is one browser data directory found on this machine; ID also names its snapshot and archive directory.
type UserDataDir struct {
	ID       DirID
	Def      Def
	Path     string
	Profiles []Profile
}

// Profile is one profile within a user data dir. Name is the output's profile value and never a path.
type Profile struct {
	Name string
	Dir  string // slash-separated, relative to UserDataDir.Path; "" when the user data dir itself is the profile
}
