package main

import (
	"fmt"
	"hack-browser-data/core/common"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"runtime"
)

func main() {
	err := utils.CopyDB(utils.GetDBPath(utils.LoginData), utils.LoginData)
	if err != nil {
		log.Println(err)
	}
	osName := runtime.GOOS
	switch osName {
	case "darwin":
		utils.InitChromeKey()
		common.ParseDB()
	case "windows":
		fmt.Println("Windows")
	}
}
