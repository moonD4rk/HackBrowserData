package main

import (
	"encoding/json"
	"fmt"
	"hack-browser-data/cmd"
	"hack-browser-data/core/common"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"path/filepath"
	"runtime"
)

func main() {
	cmd.Execute()
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
	chromePath := utils.GetDBPath(utils.LoginData, utils.History, utils.Bookmarks, utils.Cookies)
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
	d, err := json.MarshalIndent(common.FullData.Bookmarks, "", "\t")
	err = utils.WriteFile("data.json", d)
	if err != nil {
		log.Println(err)
	}
}
