//go:build windows

package browser

func (d Def) Platform() Platform { return d.Windows }
