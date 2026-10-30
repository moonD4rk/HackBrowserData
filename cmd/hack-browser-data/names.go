package main

import (
	"io"
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func browserNames() string {
	return strings.Join(browser.IDs(), "|")
}

func categoryNames() string {
	return strings.Join(category.Names(), ",")
}

// report prints the run's summary; it is not part of the output, so failing to print it never fails the run.
func report(w io.Writer, s app.Summary) {
	if err := app.Report(w, s); err != nil {
		log.Warnf("print summary: %v", err)
	}
}
