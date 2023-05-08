package byteutil

var OnSplitUTF8Func = func(r rune) rune {
	if r == 0x00 || r == 0x01 {
		return -1
	}
	return r
}
