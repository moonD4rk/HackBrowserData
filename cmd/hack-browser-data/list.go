package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/types"
)

func listCmd() *cobra.Command {
	var detail bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List detected browsers and profiles",
		Example: `  hack-browser-data list
  hack-browser-data list --detail`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browsers, err := browser.PickBrowsers(browser.PickOptions{Name: "all"})
			if err != nil {
				return err
			}
			if len(browsers) == 0 {
				cmd.Println("No browsers found.")
				return nil
			}
			if detail {
				return printDetail(cmd.OutOrStdout(), browsers)
			}
			return printBasic(cmd.OutOrStdout(), browsers)
		},
	}

	cmd.Flags().BoolVar(&detail, "detail", false, "show per-category entry counts")
	return cmd
}

func printBasic(out io.Writer, browsers []browser.Browser) error {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Browser\tProfile\tPath")
	for _, b := range browsers {
		fmt.Fprintf(w, "%s\t%s\t%s\n", b.BrowserName(), b.ProfileName(), b.ProfileDir())
	}
	return w.Flush()
}

func printDetail(out io.Writer, browsers []browser.Browser) error {
	// Build header: Browser  Profile  Password  Cookie  ...
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprint(w, "Browser\tProfile")
	for _, c := range types.AllCategories {
		fmt.Fprintf(w, "\t%s", c.String())
	}
	fmt.Fprintln(w)

	for _, b := range browsers {
		data, _ := b.Extract(types.AllCategories)
		fmt.Fprintf(w, "%s\t%s", b.BrowserName(), b.ProfileName())
		for _, c := range types.AllCategories {
			fmt.Fprintf(w, "\t%d", countEntries(data, c))
		}
		fmt.Fprintln(w)
	}
	return w.Flush()
}

func countEntries(data *types.BrowserData, c types.Category) int {
	if data == nil {
		return 0
	}
	switch c {
	case types.Password:
		return len(data.Passwords)
	case types.Cookie:
		return len(data.Cookies)
	case types.Bookmark:
		return len(data.Bookmarks)
	case types.History:
		return len(data.Histories)
	case types.Download:
		return len(data.Downloads)
	case types.CreditCard:
		return len(data.CreditCards)
	case types.Extension:
		return len(data.Extensions)
	case types.LocalStorage:
		return len(data.LocalStorage)
	case types.SessionStorage:
		return len(data.SessionStorage)
	default:
		return 0
	}
}
