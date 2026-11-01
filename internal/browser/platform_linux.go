//go:build linux

package browser

func (d Def) Platform() Platform { return d.Linux }
