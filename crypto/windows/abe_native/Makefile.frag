ZIG        ?= zig
ABE_ARCH   ?= amd64
ABE_TARGET ?= x86_64-windows-gnu

ABE_SRC_DIR = crypto/windows/abe_native
ABE_BIN_DIR = crypto
ABE_BIN     = $(ABE_BIN_DIR)/abe_extractor_$(ABE_ARCH).bin

ABE_CFLAGS = -shared -s -O2 \
             -fno-stack-protector -fno-builtin \
             -I$(ABE_SRC_DIR)
ABE_LDFLAGS = -Wl,--subsystem,windows
ABE_LDLIBS  = -lole32 -loleaut32 -lcrypt32

ABE_C_SRCS = $(ABE_SRC_DIR)/abe_extractor.c \
             $(ABE_SRC_DIR)/com_iid.c \
             $(ABE_SRC_DIR)/bootstrap.c

ABE_HDRS = $(ABE_SRC_DIR)/com_iid.h \
           $(ABE_SRC_DIR)/bootstrap.h

$(ABE_BIN): $(ABE_C_SRCS) $(ABE_HDRS)
	@mkdir -p $(ABE_BIN_DIR)
	$(ZIG) cc -target $(ABE_TARGET) $(ABE_CFLAGS) $(ABE_LDFLAGS) \
	    $(ABE_C_SRCS) -o $@ $(ABE_LDLIBS)
	@printf "built %s (%s bytes)\n" "$@" "$$(wc -c < $@ | tr -d ' ')"

.PHONY: payload payload-verify payload-clean

payload: $(ABE_BIN)

payload-verify: $(ABE_BIN)
	@if strings -a "$(ABE_BIN)" | grep -qx "Bootstrap"; then \
		echo "OK: Bootstrap export name present"; \
	else \
		echo "FAIL: Bootstrap not found in $(ABE_BIN)"; \
		exit 1; \
	fi

payload-clean:
	rm -f $(ABE_BIN_DIR)/abe_extractor_*.bin

# Scratch-layout codegen. The C header bootstrap_layout.h is the single
# source of truth; the Go constants in crypto/windows/abe_native/bootstrap
# are derived from it via cgo -godefs. We pin CC to zig for reproducible
# output across macOS / Linux / Windows hosts.
ABE_LAYOUT_PKG = $(ABE_SRC_DIR)/bootstrap
ABE_LAYOUT_GO  = $(ABE_LAYOUT_PKG)/layout.go

.PHONY: gen-layout gen-layout-verify

# Split into two stages so a cgo failure doesn't silently produce an empty
# layout.go via `gofmt` on empty stdin. Write cgo output to a temp file first;
# only if that step succeeds do we format and publish.
gen-layout:
	cd $(ABE_LAYOUT_PKG) && \
	  CC="$(ZIG) cc" $(GO) tool cgo -godefs layout_gen.go > layout.go.tmp && \
	  gofmt layout.go.tmp > layout.go && \
	  rm -f layout.go.tmp && \
	  rm -rf _obj

gen-layout-verify: gen-layout
	@git diff --exit-code $(ABE_LAYOUT_GO) >/dev/null || \
	  (echo "layout.go is stale — run 'make gen-layout' and commit"; exit 1)
