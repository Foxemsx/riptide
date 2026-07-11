# riptide — Makefile (POSIX, no GNU extensions)
#
# Common targets:
#   make            build a local binary for the current platform
#   make build      same as above
#   make install    go install into GOBIN
#   make test       run go tests
#   make clean      remove local build artifacts
#   make dist       cross-compile all 6 release archives (+ checksums.txt)
#
# Release matrix (GOOS/GOARCH):
#   linux/amd64    linux/arm64
#   windows/amd64  windows/arm64
#   darwin/amd64   darwin/arm64
#
# Archives: .tar.gz for unix (linux + darwin), .zip for windows. Each contains the binary
# (riptide / riptide.exe), LICENSE, and README.md.

BINARY   := riptide
PKG      := ./cmd/riptide
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
DIST     := dist

# Build environments: <os>/<arch> pairs.
TARGETS := linux/amd64  linux/arm64 \
           windows/amd64 windows/arm64 \
           darwin/amd64  darwin/arm64

.PHONY: all build install test clean dist

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf $(DIST)

# Generate one archive per target.
# Use a shell loop so this stays POSIX-portable.
dist: clean
	@mkdir -p $(DIST)
	@for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; \
		ext=""; [ "$$os" = "windows" ] && bin=$(BINARY).exe || bin=$(BINARY); \
		out=$(DIST)/riptide-$$os-$$arch; \
		echo ">> building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "$$out/$$bin" $(PKG) || exit 1; \
		cp LICENSE README.md "$$out/" 2>/dev/null || true; \
		if [ "$$os" = "windows" ]; then \
			(cd $(DIST) && zip -rq riptide-$$os-$$arch.zip riptide-$$os-$$arch); \
		else \
			tar -C $(DIST) -czf $(DIST)/riptide-$$os-$$arch.tar.gz riptide-$$os-$$arch; \
		fi; \
		rm -rf "$$out"; \
	done
	@cd $(DIST) && sha256sum * > checksums.txt
	@echo ">> dist ready in $(DIST)/ (with checksums.txt)"
