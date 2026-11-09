package main

import (
	"strings"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

func browserNames() string {
	return strings.Join(browser.Names(), "|")
}

func categoryNames() string {
	return strings.Join(category.Names(), ",")
}
