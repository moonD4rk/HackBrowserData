package extractor

// Extractor is an interface for extracting data from browser data files
type Extractor interface {
	Extract(masterKey []byte) error

	Name() string

	Len() int
}
