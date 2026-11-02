//go:build darwin

package browser

func (b Browser) Platform() Platform { return b.Darwin }
