//go:build darwin

package browser

// Platform returns the running OS's entry.
func (d Def) Platform() Platform { return d.Darwin }
