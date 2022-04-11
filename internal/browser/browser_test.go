package browser

import (
	"fmt"
	"testing"

	"hack-browser-data/internal/browser/chromium"
	"hack-browser-data/internal/item"
	"hack-browser-data/internal/outputter"
)

func TestPickChromium(t *testing.T) {
	// browsers := pickChromium("chrome")
	// log.InitLog("debug")
	// filetype := "json"
	// // dir := "result"
	// output := outputter.NewOutPutter(filetype)
	// _ = output
	// for _, b := range browsers {
	// 	fmt.Printf("%+v\n", b)
	// 	if err := b.CopyItemFileToLocal(); err != nil {
	// 		panic(err)
	// 	}
	// 	_, err := b.GetMasterKey()
	// 	if err != nil {
	// 		fmt.Println(err)
	// 	}
	// 	// browserName := b.GetName()
	// 	data, err := b.GetBrowsingData()
	// 	fmt.Println(data)
	// 	// for _, data := range multiData {
	// 	// 	if err := data.Parse(masterKey); err != nil {
	// 	// 		fmt.Println(err)
	// 	// 	}
	// 	// 	filename := fmt.Sprintf("%s_%s.%s", browserName, data.Name(), filetype)
	// 	// 	file, err := output.CreateFile(dir, filename)
	// 	// 	if err != nil {
	// 	// 		panic(err)
	// 	// 	}
	// 	// 	if err := output.Write(data, file); err != nil {
	// 	// 		panic(err)
	// 	// 	}
	// 	// }
	// }
}

func TestGetChromiumItemAbsPath(t *testing.T) {
	p := `/Library/Application Support/Google/Chrome/`
	p = homeDir + p
	c, err := chromium.New("chrome", "Chrome", p, item.DefaultChromium)
	if err != nil {
		t.Error(err)
	}
	data, err := c.GetBrowsingData()
	if err != nil {
		t.Error(err)
	}
	output := outputter.NewOutPutter("json")

	if err != nil {
		t.Error(err)
	}
	for _, v := range data.Sources {
		f, err := output.CreateFile("result", v.Name()+".json")
		if err != nil {
			panic(err)
		}
		if err := output.Write(v, f); err != nil {
			panic(err)
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

// func TestPickFirefox(t *testing.T) {
// 	browsers := pickFirefox("all")
// 	filetype := "json"
// 	dir := "result"
// 	output := outputter.NewOutPutter(filetype)
// 	if err := output.MakeDir("result"); err != nil {
// 		panic(err)
// 	}
// 	for _, b := range browsers {
// 		fmt.Printf("%+v\n", b)
// 		if err := b.CopyItemFileToLocal(); err != nil {
// 			panic(err)
// 		}
// 		masterKey, err := b.GetMasterKey()
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		browserName := b.GetName()
// 		multiData := b.GetBrowsingData()
// 		for _, data := range multiData {
// 			if err := data.Parse(masterKey); err != nil {
// 				fmt.Println(err)
// 			}
// 			filename := fmt.Sprintf("%s_%s.%s", browserName, data.Name(), filetype)
// 			file, err := output.CreateFile(dir, filename)
// 			if err != nil {
// 				panic(err)
// 			}
// 			if err := output.Write(data, file); err != nil {
// 				panic(err)
// 			}
// 		}
// 	}
// }
