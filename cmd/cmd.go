package cmd

import (
	"hack-browser-data/core"
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"
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
		Version: "0.1.7",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "Verbose"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browser, Value: "all", Usage: "Available browsers: all|" + strings.Join(core.ListBrowser(), "|")},
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
			// default select all browsers
			browsers, err := core.PickBrowsers(browser)
			if err != nil {
				log.Error(err)
			}
			utils.MakeDir(exportDir)
			for _, v := range browsers {
				err := v.InitSecretKey()
				if err != nil {
					log.Error(err)
				}
				err = v.GetProfilePath(exportData)
				if err != nil {
					log.Error(err)
				}
				v.ParseDB()
				v.OutPut(exportDir, outputFormat)
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
