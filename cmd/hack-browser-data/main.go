package main

import (
	"fmt"
	"os"
	"strings"

	"hack-browser-data/internal/browser"
	"hack-browser-data/internal/log"
	"hack-browser-data/internal/outputter"
	"hack-browser-data/internal/utils/fileutil"

	"github.com/urfave/cli/v2"
)

var (
	browserName  string
	outputDir    string
	outputFormat string
	verbose      bool
	compress     bool
	profilePath  string
)

func main() {
	Execute()
}

func Execute() {
	app := &cli.App{
		Name:      "hack-browser-data",
		Usage:     "Export passwords/cookies/history/bookmarks from browser",
		UsageText: "[hack-browser-data -b chrome -f json -dir results -cc]\nGet all browingdata(password/cookie/history/bookmark) from browser",
		Version:   "0.4.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "verbose"},
			&cli.BoolFlag{Name: "compress", Aliases: []string{"cc"}, Destination: &compress, Value: false, Usage: "compress result to zip"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browserName, Value: "all", Usage: "available browsers: all|" + strings.Join(browser.ListBrowser(), "|")},
			&cli.StringFlag{Name: "results-dir", Aliases: []string{"dir"}, Destination: &outputDir, Value: "results", Usage: "export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "format, csv|json|console"},
			&cli.StringFlag{Name: "profile-path", Aliases: []string{"p"}, Destination: &profilePath, Value: "", Usage: "custom profile dir path, get with chrome://version"},
		},
		HideHelpCommand: true,
		Action: func(c *cli.Context) error {
			if verbose {
				log.Init("debug")
			} else {
				log.Init("error")
			}
			var (
				browsers []browser.Browser
				err      error
			)
			log.Debugf("browser: %s", browserName)
			browsers, err = browser.PickBrowser(browserName, profilePath)
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
					f, err = output.CreateFile(outputDir, filename)
					if err != nil {
						log.Error(err)
					}
					err = output.Write(source, f)
					if err != nil {
						log.Error(err)
					}
				}
			}
			if compress {
				err = fileutil.CompressDir(outputDir)
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
