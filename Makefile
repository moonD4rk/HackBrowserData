GO     ?= go
GOEXE  ?= hack-browser-data

include crypto/windows/abe_native/Makefile.frag

.PHONY: build build-windows clean

build:
	$(GO) build -o $(GOEXE) ./cmd/hack-browser-data

# Make sets these in the recipe environment on both POSIX and native Windows
# GNU Make, so we do not need a cmd.exe vs sh branch for build-windows.
build-windows: export GOOS := windows
build-windows: export GOARCH := amd64
build-windows: export CGO_ENABLED := 0
build-windows: $(ABE_BIN)
	$(GO) build -tags abe_embed -trimpath -ldflags="-s -w" -o $(GOEXE).exe ./cmd/hack-browser-data

clean: payload-clean
	rm -f $(GOEXE) $(GOEXE).exe
