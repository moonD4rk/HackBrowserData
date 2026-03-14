package types2

type DataTypeInfo struct {
	filename       string
	isDir          bool
	alternateNames []string
}

func (dt DataTypeInfo) Filename() string {
	return dt.filename
}

func (dt DataTypeInfo) IsDir() bool {
	return dt.isDir
}

func (dt DataTypeInfo) AlternateNames() []string {
	return dt.alternateNames
}
