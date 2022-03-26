package browser

import (
	"fmt"
	"testing"

	"hack-browser-data/internal/browser/outputter"
	"hack-browser-data/internal/log"
)

func TestPickChromium(t *testing.T) {
	browsers := pickChromium("chrome")
	log.InitLog("debug")
	filetype := "json"
	dir := "result"
	output := outputter.NewOutPutter(filetype)
	if err := output.MakeDir("result"); err != nil {
		panic(err)
	}
	for _, b := range browsers {
		fmt.Printf("%+v\n", b)
		if err := b.CopyItemFileToLocal(); err != nil {
			panic(err)
		}
		masterKey, err := b.GetMasterKey()
		if err != nil {
			fmt.Println(err)
		}
		browserName := b.GetName()
		multiData := b.GetBrowsingData()
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

func TestPickBrowsers(t *testing.T) {
	browsers := PickBrowser("all")
	for _, v := range browsers {
		fmt.Println(v.GetName())
	}
	// filetype := "json"
	// dir := "result"
	// output := outputter.NewOutPutter(filetype)
}

func TestPickFirefox(t *testing.T) {
	browsers := pickFirefox("all")
	filetype := "json"
	dir := "result"
	output := outputter.NewOutPutter(filetype)
	if err := output.MakeDir("result"); err != nil {
		panic(err)
	}
	for _, b := range browsers {
		fmt.Printf("%+v\n", b)
		if err := b.CopyItemFileToLocal(); err != nil {
			panic(err)
		}
		masterKey, err := b.GetMasterKey()
		if err != nil {
			fmt.Println(err)
		}
		browserName := b.GetName()
		multiData := b.GetBrowsingData()
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
