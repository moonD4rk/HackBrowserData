//go:build linux

package browser

func (b Browser) Platform() Platform { return b.Linux }
