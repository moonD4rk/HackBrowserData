//go:build windows

package browser

func (b Browser) Platform() Platform { return b.Windows }
