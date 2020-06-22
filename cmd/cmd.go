package cmd

import (
	"hack-browser-data/log"
	"hack-browser-data/utils"
	"os"
	"runtime"

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
		Usage:   "export password/cookie/history/bookmark from browser",
		Version: "0.0.1",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"vv"}, Destination: &verbose, Value: false, Usage: "verbose"},
			&cli.StringFlag{Name: "browser", Aliases: []string{"b"}, Destination: &browser, Value: "all", Usage: "browser name, all|chrome|safari"},
			&cli.StringFlag{Name: "dir", Aliases: []string{"d"}, Destination: &exportDir, Value: "data", Usage: "export dir"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Destination: &outputFormat, Value: "csv", Usage: "result format, csv|json"},
			&cli.StringFlag{Name: "export-data", Aliases: []string{"e"}, Destination: &exportData, Value: "all", Usage: "all|password|cookie|history|bookmark"},
		},
		Action: func(c *cli.Context) error {
			log.InitLog()
			switch runtime.GOOS {
			case "darwin":
				err := utils.InitChromeKey()
				if err != nil {

				}
			case "windows":
			}
			switch {
			case browser == "all":
			case exportDir == "data":
			case exportData == "all":
			case outputFormat == "json":
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
