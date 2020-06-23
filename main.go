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
		err := utils.InitChromeKey()
		if err != nil {
			log.Println(err)
			panic("init chrome key failed")
		}
	case "windows":
		fmt.Println("Windows")
	}
	chromePath, err := utils.GetDBPath(utils.LoginData, utils.History, utils.Bookmarks, utils.Cookies)
	//chromePath, err := utils.GetDBPath(utils.Cookies)
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
	fmt.Println("bookmarks", len(common.FullData.Bookmarks))
	fmt.Println("cookies", len(common.FullData.Cookies))
	fmt.Println("login data", len(common.FullData.LoginData))
	fmt.Println("history", len(common.FullData.History))
}
