package main

import (
	"os"
	"sort"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// browserNames lists this OS's browser IDs for the usage strings, sorted as they have always been listed.
func browserNames() string {
	defs := browser.Supported()
	ids := make([]string, 0, len(defs))
	for _, d := range defs {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, "|")
}

func categoryNames() string {
	all := category.All()
	names := make([]string, len(all))
	for i, c := range all {
		names[i] = c.String()
	}
	return strings.Join(names, ",")
}

// report prints the run's summary; it is not part of the output, so failing to print it never fails the run.
func report(s app.Summary) {
	if err := app.Report(os.Stderr, s); err != nil {
		log.Warnf("print summary: %v", err)
	}
}
