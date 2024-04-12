// Package browserdata is responsible for initializing all the necessary
// components that handle different types of browser data extraction.
// This file, imports.go, is specifically used to import various data
// handler packages to ensure their initialization logic is executed.
// These imports are crucial as they trigger the `init()` functions
// within each package, which typically handle registration of their
// specific data handlers to a central registry.
package browserdata

import (
	_ "github.com/moond4rk/hackbrowserdata/browserdata/bookmark"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/cookie"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/creditcard"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/download"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/extension"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/history"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/localstorage"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/password"
	_ "github.com/moond4rk/hackbrowserdata/browserdata/sessionstorage"
)
