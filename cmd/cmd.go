package cmd

import (
	"hack-browser-data/core"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

var (
	browser      string
	exportData   string
	exportDir    string
	outputFormat string
	verbose      bool
)

func Execute() {
	app := &cli.App{
		Name:  "hack-browser-data",
		Usage: "Export passwords/cookies/history/bookmarks from browser",
		UsageText: "[hack-browser-data -b chrome -f json -dir results -e all]\n 	Get all data(password/cookie/history/bookmark) from chrome",
		Version: "0.1.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "Verbose"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browser, Value: "chrome", Usage: "Available browsers: " + strings.Join(utils.ListBrowser(), "|")},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &exportDir, Value: "results", Usage: "Export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "Format, csv|json"},
			&cli.StringFlag{Name: "export-data", Aliases: []string{"e"}, Destination: &exportData, Value: "all", Usage: "all|password|cookie|history|bookmark"},
		},
		HideHelpCommand: true,
		HideVersion:     true,
		Action: func(c *cli.Context) error {
			if verbose {
				log.InitLog("debug")
			} else {
				log.InitLog("error")
			}
			browserDir, key, err := utils.PickBrowser(browser)
			if err != nil {
				log.Fatal(err, " Available browsers: "+strings.Join(utils.ListBrowser(), "|"))
			}
			if browser != "firefox" {
				err = utils.InitKey(key)
				if err != nil {
					log.Fatal(err, "Please Open an issue on GitHub")
				}
				var fileList []string
				switch exportData {
				case "all":
					fileList = utils.GetDBPath(browserDir, utils.LoginData, utils.History, utils.Bookmarks, utils.Cookies)
				case "password", "cookie", "history", "bookmark":
					fileList = utils.GetDBPath(browserDir, exportData)
				default:
					log.Fatal("Choose one from all|password|cookie|history|bookmark")
				}
				for _, v := range fileList {
					dst := filepath.Base(v)
					err := utils.CopyDB(v, dst)
					if err != nil {
						log.Debug(err)
						continue
					}
					core.ParseResult(dst)
				}
			} else {
				fileList := utils.GetDBPath(browserDir, utils.FirefoxLoginData, utils.FirefoxKey4DB, utils.FirefoxCookie, utils.FirefoxData)
				for _, v := range fileList {
					dst := filepath.Base(v)
					err := utils.CopyDB(v, dst)
					if err != nil {
						log.Debug(err)
						continue
					}
					core.ParseResult(dst)
				}
			}
			core.FullData.Sorted()
			utils.MakeDir(exportDir)
			if outputFormat == "json" {
				err := core.FullData.OutPutJson(exportDir, browser, outputFormat)
				if err != nil {
					log.Error(err)
				}
			} else {
				err := core.FullData.OutPutCsv(exportDir, browser, outputFormat)
				if err != nil {
					log.Error(err)
				}
			}
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
		return
	}
}
