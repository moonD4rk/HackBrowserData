package main

import (
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func listCmd() *cobra.Command {
	var detail bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List detected browsers and profiles",
		Example: `  hack-browser-data list
  hack-browser-data list --detail`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := app.ListRequest()
			if err != nil {
				return err
			}
			d := app.Discover(req)
			for _, r := range d.Failed {
				log.Warnf("%s: %v", r.DirID, r.Err)
			}
			if len(d.Dirs) == 0 {
				cmd.Println("No browsers found.")
				return nil
			}
			if !detail {
				return printBasic(cmd.OutOrStdout(), d)
			}
			summary, err := app.Count(cmd.Context(), app.NewPlan(d, req), req)
			if err != nil {
				return err
			}
			return printDetail(cmd.OutOrStdout(), d, summary)
		},
	}

	cmd.Flags().BoolVar(&detail, "detail", false, "show per-category entry counts")
	return cmd
}

func printBasic(out io.Writer, d app.Discovery) error {
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Browser\tProfile\tPath")
	for _, dir := range d.Dirs {
		for _, p := range dir.Profiles {
			fmt.Fprintf(w, "%s\t%s\t%s\n", dir.Def.Name, p.Name, filepath.Join(dir.Path, filepath.FromSlash(p.Dir)))
		}
	}
	return w.Flush()
}

func printDetail(out io.Writer, d app.Discovery, s app.Summary) error {
	counts := make(map[app.Extraction]int, len(s.Results))
	for _, r := range s.Results {
		counts[r.Extraction] = r.Count
	}

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprint(w, "Browser\tProfile")
	for _, c := range category.All() {
		fmt.Fprintf(w, "\t%s", c.String())
	}
	fmt.Fprintln(w)
	for _, dir := range d.Dirs {
		for _, p := range dir.Profiles {
			fmt.Fprintf(w, "%s\t%s", dir.Def.Name, p.Name)
			for _, c := range category.All() {
				fmt.Fprintf(w, "\t%d", counts[app.Extraction{DirID: dir.ID, Profile: p.Name, Category: c}])
			}
			fmt.Fprintln(w)
		}
	}
	return w.Flush()
}
