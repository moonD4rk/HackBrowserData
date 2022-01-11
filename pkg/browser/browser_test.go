package browser

import (
	"fmt"
	"testing"

	"hack-browser-data/pkg/browser/outputter"
)

func TestPickChromium(t *testing.T) {
	browsers := PickChromium("all")
	filetype := "json"
	dir := "result"
	output := outputter.NewOutPutter(filetype)
	if err := output.MakeDir("result"); err != nil {
		panic(err)
	}
	for _, b := range browsers {
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
		// TODO: 优化获取 Data 逻辑
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

func TestPickFirefox(t *testing.T) {
	browsers := PickFirefox("all")
	filetype := "json"
	dir := "result"
	output := outputter.NewOutPutter(filetype)
	if err := output.MakeDir("result"); err != nil {
		panic(err)
	}
	for _, b := range browsers {
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
		// TODO: 优化获取 Data 逻辑
		for _, data := range multiData {
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
