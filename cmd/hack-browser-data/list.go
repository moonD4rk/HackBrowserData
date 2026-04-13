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
			browsers, err := browser.DiscoverBrowsers(browser.PickOptions{Name: "all"})
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
		counts, _ := b.CountEntries(types.AllCategories)
		fmt.Fprintf(w, "%s\t%s", b.BrowserName(), b.ProfileName())
		for _, c := range types.AllCategories {
			fmt.Fprintf(w, "\t%d", counts[c])
		}
		fmt.Fprintln(w)
	}
	return w.Flush()
}
