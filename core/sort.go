package core

func (l LoginDataSlice) Len() int {
	return len(l)
}

func (l LoginDataSlice) Less(i, j int) bool {
	return l[i].CreateDate.After(l[j].CreateDate)
}

func (l LoginDataSlice) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}
