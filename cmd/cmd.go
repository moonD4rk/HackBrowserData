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
	exportDir    string
	outputFormat string
	verbose      bool
	compress     bool
)

func Execute() {
	app := &cli.App{
		Name:  "hack-browser-data",
		Usage: "Export passwords/cookies/history/bookmarks from browser",
		UsageText: "[hack-browser-data -b chrome -f json -dir results -cc]\n 	Get all data(password/cookie/history/bookmark) from chrome",
		Version: "0.2.4",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "Verbose"},
			&cli.BoolFlag{Name: "compress", Aliases: []string{"cc"}, Destination: &compress, Value: false, Usage: "Compress result to zip"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browser, Value: "all", Usage: "Available browsers: all|" + strings.Join(core.ListBrowser(), "|")},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &exportDir, Value: "results", Usage: "Export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "json", Usage: "Format, csv|json|console"},
		},
		HideHelpCommand: true,
		Action: func(c *cli.Context) error {
			if verbose {
				log.InitLog("debug")
			} else {
				log.InitLog("error")
			}
			// default select all browsers
			browsers, err := core.PickBrowser(browser)
			if err != nil {
				log.Error(err)
			}
			err = utils.MakeDir(exportDir)
			if err != nil {
				log.Error(err)
			}
			for _, browser := range browsers {
				err := browser.InitSecretKey()
				if err != nil {
					log.Error(err)
				}
				// default select all items
				// you can get single item with browser.GetItem(itemName)
				items, err := browser.GetAllItems()
				if err != nil {
					log.Error(err)
				}
				name := browser.GetName()
				key := browser.GetSecretKey()
				for _, item := range items {
					err := item.CopyDB()
					if err != nil {
						log.Error(err)
					}
					switch browser.(type) {
					case *core.Chromium:
						err := item.ChromeParse(key)
						if err != nil {
							log.Error(err)
						}
					case *core.Firefox:
						err := item.FirefoxParse()
						if err != nil {
							log.Error(err)
						}
					}
					err = item.Release()
					if err != nil {
						log.Error(err)
					}
					err = item.OutPut(outputFormat, name, exportDir)
					if err != nil {
						log.Error(err)
					}
				}
			}
			if compress {
				err = utils.Compress(exportDir)
				if err != nil {
					log.Error(err)
				}
			}
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error(err)
	}
}
