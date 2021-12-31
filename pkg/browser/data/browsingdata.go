package data

type BrowsingData interface {
	Parse(masterKey []byte) error

	Name() string
}
