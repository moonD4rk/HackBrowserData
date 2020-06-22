package main

import (
	"fmt"
	"hack-browser-data/core/common"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"path/filepath"
	"runtime"
)

func main() {
	log.InitLog()
	parse()
}

func parse() {
	osName := runtime.GOOS
	switch osName {
	case "darwin":
		//err := utils.InitChromeKey()
		//if err != nil {
		//	log.Println(err)
		//	panic("init chrome key failed")
		//}
	case "windows":
		fmt.Println("Windows")
	}
	//chromePath, err := utils.GetDBPath(utils.LoginData, utils.History, utils.BookMarks, utils.Cookies, utils.WebData)
	chromePath, err := utils.GetDBPath(utils.Bookmarks)
	if err != nil {
		log.Error("can't find chrome.app in OS")
	}
	for _, v := range chromePath {
		dst := filepath.Base(v)
		err := utils.CopyDB(v, dst)
		if err != nil {
			log.Println(err)
			continue
		}
		common.ParseDB(dst)
	}

}
