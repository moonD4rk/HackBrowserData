package cmd

import (
	"hack-browser-data/core"
	"hack-browser-data/log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

var (
	browser      string
	exportDir    string
	outputFormat string
	fileType     string
	filepath     string
	verbose      bool
	compress     bool
)

func Execute() {
	app := &cli.App{
		Name:  "hack-browser-data",
		Usage: "Export passwords/cookies/history/bookmarks from browser",
		UsageText: "[hack-browser-data -b chrome -f json -dir results -cc]\n 	Get all data(password/cookie/history/bookmark) from chrome",
		Version: "0.3.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "Verbose"},
			&cli.BoolFlag{Name: "compress", Aliases: []string{"cc"}, Destination: &compress, Value: false, Usage: "Compress result to zip"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browser, Value: "all", Usage: "Available browsers: all|" + strings.Join(core.ListBrowser(), "|")},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &exportDir, Value: "results", Usage: "Export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "Format, csv|json|console"},
			&cli.StringFlag{Name: "file-type", Aliases: []string{"ft"}, Destination: &fileType, Value: "csv", Usage: "File Type, bookmark|cookie|history|password|creditcard"},
			&cli.StringFlag{Name: "filepath", Destination: &filepath, Value: "csv", Usage: "Filepath of the file type"},
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

			browser := browsers[0]
			err = browser.InitSecretKey()
			if err != nil {
				log.Error(err)
			}
			item, err := browser.GetItem(fileType)
			if err != nil {
				log.Error(err)
			}
			name := browser.GetName()
			key := browser.GetSecretKey()
			switch browser.(type) {
			case *core.Chromium:
				err := item.ChromeParse(key, filepath)
				if err != nil {
					log.Error(err)
				}
			case *core.Firefox:
				err := item.FirefoxParse(filepath)
				if err != nil {
					log.Error(err)
				}
			}
			err = item.OutPut(outputFormat, name, exportDir)
			if err != nil {
				log.Error(err)
			}
		
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error(err)
	}
}
