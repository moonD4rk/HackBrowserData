GO     ?= go
GOEXE  ?= hack-browser-data

include crypto/windows/abe_native/Makefile.frag

.PHONY: build build-windows clean

build:
	$(GO) build -o $(GOEXE) ./cmd/hack-browser-data

build-windows: $(ABE_BIN)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	  $(GO) build -tags abe_embed -trimpath -ldflags="-s -w" \
	  -o $(GOEXE).exe ./cmd/hack-browser-data

clean: payload-clean
	rm -f $(GOEXE) $(GOEXE).exe
