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
