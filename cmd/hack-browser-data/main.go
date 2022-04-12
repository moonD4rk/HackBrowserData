package main

import (
	"fmt"
	"os"
	"strings"

	"hack-browser-data/internal/browser"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/outputter"
	"hack-browser-data/internal/utils"

	"github.com/urfave/cli/v2"
)

var (
	browserName       string
	exportDir         string
	outputFormat      string
	verbose           bool
	compress          bool
	customProfilePath string
)

func main() {
	Execute()
}

func Execute() {
	app := &cli.App{
		Name:  "hack-browser-data",
		Usage: "Export passwords/cookies/history/bookmarks from browser",
		UsageText: "[hack-browser-data -b chrome -f json -dir results -cc]\n 	Get all browingdata(password/cookie/history/bookmark) from chrome",
		Version: "0.4.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "verbose"},
			&cli.BoolFlag{Name: "compress", Aliases: []string{"cc"}, Destination: &compress, Value: false, Usage: "compress result to zip"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browserName, Value: "all", Usage: "available browsers: all|" + strings.Join(browser.ListBrowser(), "|")},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &exportDir, Value: "results", Usage: "export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "format, csv|json|console"},
			&cli.StringFlag{Name: "profile-dir-path", Aliases: []string{"p"}, Destination: &customProfilePath, Value: "", Usage: "custom profile dir path, get with chrome://version"},
		},
		HideHelpCommand: true,
		Action: func(c *cli.Context) error {
			var (
				browsers []browser.Browser
				err      error
			)
			if verbose {
				log.InitLog("debug")
			} else {
				log.InitLog("error")
			}
			browsers, err = browser.PickBrowser(browserName)
			if err != nil {
				log.Error(err)
			}
			output := outputter.New(outputFormat)

			for _, b := range browsers {
				data, err := b.GetBrowsingData()
				if err != nil {
					log.Error(err)
				}
				var f *os.File
				for _, source := range data.Sources {
					filename := fmt.Sprintf("%s_%s.%s", b.Name(), source.Name(), outputFormat)
					f, err = output.CreateFile(exportDir, filename)
					err = output.Write(source, f)
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
