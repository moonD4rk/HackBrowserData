package browser

import (
	"fmt"
	"testing"

	"hack-browser-data/pkg/browser/outputter"
)

func TestPickBrowsers(t *testing.T) {
	browsers := PickBrowsers("all")
	filetype := "json"
	dir := "result"
	output := outputter.NewOutPutter(filetype)
	if err := output.MakeDir("result"); err != nil {
		panic(err)
	}
	for _, b := range browsers {
		if err := b.walkItemAbsPath(); err != nil {
			panic(err)
		}
		fmt.Printf("%+v\n", b)
		if err := b.copyItemFileToLocal(); err != nil {
			panic(err)
		}
		masterKey, err := b.GetMasterKey()
		if err != nil {
			fmt.Println(err)
		}
		browserName := b.GetBrowserName()
		multiData := b.GetBrowsingData()
		for _, data := range multiData {
			if data == nil {
				fmt.Println(data)
				continue
			}
			if err := data.Parse(masterKey); err != nil {
				fmt.Println(err)
			}
			filename := fmt.Sprintf("%s_%s.%s", browserName, data.Name(), filetype)
			file, err := output.CreateFile(dir, filename)
			if err != nil {
				panic(err)
			}
			if err := output.Write(data, file); err != nil {
				panic(err)
			}
		}
	}
}
