package browserdata

import (
	"github.com/moond4rk/hackbrowserdata/extractor"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

type BrowserData struct {
	extractors map[types.DataType]extractor.Extractor
}

func New(items []types.DataType) *BrowserData {
	bd := &BrowserData{
		extractors: make(map[types.DataType]extractor.Extractor),
	}
	bd.addExtractors(items)
	return bd
}

func (d *BrowserData) Recovery(masterKey []byte) error {
	for _, source := range d.extractors {
		if err := source.Extract(masterKey); err != nil {
			log.Errorf("parse %s error: %v", source.Name(), err)
			continue
		}
	}
	return nil
}

func (d *BrowserData) Output(dir, browserName, flag string) {
	output := newOutPutter(flag)

	for _, source := range d.extractors {
		if source.Len() == 0 {
			// if the length of the export data is 0, then it is not necessary to output
			continue
		}
		filename := fileutil.Filename(browserName, source.Name(), output.Ext())

		f, err := output.CreateFile(dir, filename)
		if err != nil {
			log.Errorf("create file %s error: %v", filename, err)
			continue
		}
		if err := output.Write(source, f); err != nil {
			log.Errorf("write to file %s error: %v", filename, err)
			continue
		}
		if err := f.Close(); err != nil {
			log.Errorf("close file %s error: %v", filename, err)
			continue
		}
		log.Warnf("export success: %s", filename)
	}
}

func (d *BrowserData) addExtractors(items []types.DataType) {
	for _, itemType := range items {
		if source := extractor.CreateExtractor(itemType); source != nil {
			d.extractors[itemType] = source
		} else {
			log.Debugf("source not found: %s", itemType)
		}
	}
}
