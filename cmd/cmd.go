package cmd

import (
	"hack-browser-data/core"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"
	"path/filepath"

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
		Name:    "hack-browser-data",
		Usage:   "export passwords/cookies/history/bookmarks from browser",
		Version: "0.1.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "verbose"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browser, Value: "all", Usage: "browser name, all|chrome|safari"},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"d"}, Destination: &exportDir, Value: "results", Usage: "export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "result format, csv|json"},
			&cli.StringFlag{Name: "export-data", Aliases: []string{"e"}, Destination: &exportData, Value: "all", Usage: "all|password|cookie|history|bookmark"},
		},
		Action: func(c *cli.Context) error {
			log.InitLog()
			utils.MakeDir(exportDir)
			var fileList []string
			switch exportData {
			case "all":
				fileList = utils.GetDBPath(utils.LoginData, utils.History, utils.Bookmarks, utils.Cookies)
			case "password", "cookie", "history", "bookmark":
				fileList = utils.GetDBPath(exportData)
			default:
				log.Fatal("choose one from all|password|cookie|history|bookmark")
			}
			err := utils.InitChromeKey()
			if err != nil {
				panic(err)
			}
			for _, v := range fileList {
				dst := filepath.Base(v)
				err := utils.CopyDB(v, dst)
				if err != nil {
					log.Println(err)
					continue
				}
				core.ChromeDB(dst)
			}
			if outputFormat == "json" {
				err := core.FullData.OutPutJson(exportDir, outputFormat)
				if err != nil {
					log.Error(err)
				}
			} else {
				err := core.FullData.OutPutCsv(exportDir, outputFormat)
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
