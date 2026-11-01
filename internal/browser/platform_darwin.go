//go:build darwin

package browser

func (d Def) Platform() Platform { return d.Darwin }
