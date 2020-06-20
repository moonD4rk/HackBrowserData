package main

import (
	"fmt"
	"hack-browser-data/core/common"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"
	"runtime"
)

func main() {
	osName := runtime.GOOS
	switch osName {
	case "darwin":
		chromePath, err := utils.GetDBPath(utils.LoginData)
		if err != nil {
			log.Error("can't find chrome.app in OS")
		}
		err = utils.CopyDB(chromePath, utils.LoginData)
		if err != nil {
			log.Println(err)
		}
		utils.InitChromeKey()
		common.ParseDB()
	case "windows":
		fmt.Println("Windows")
	}
	os.Remove(utils.LoginData)
}
