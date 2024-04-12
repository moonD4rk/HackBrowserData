package extractor

import (
	"github.com/moond4rk/hackbrowserdata/types"
)

var extractorRegistry = make(map[types.DataType]func() Extractor)

// RegisterExtractor is used to register the data source
func RegisterExtractor(dataType types.DataType, factoryFunc func() Extractor) {
	extractorRegistry[dataType] = factoryFunc
}

// CreateExtractor is used to create the data source
func CreateExtractor(dataType types.DataType) Extractor {
	if factoryFunc, ok := extractorRegistry[dataType]; ok {
		return factoryFunc()
	}
	return nil
}
