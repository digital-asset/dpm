BINARY  := rst-to-mdx
DIST    := dist
GO      ?= go
# Stamp the binary with VERSION via -ldflags so `./rst-to-mdx --version`
# reports something useful in release builds. Override on the command line
# (e.g. `make release VERSION=0.1.0-alpha`) or via env. Default keeps the
# dev sentinel so unstamped local builds remain visibly unreleased.
VERSION ?= 0.0.1-dev
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
OS_ARCHES := darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/amd64

.PHONY: all build test fmt vet tidy smoke release clean

all: build

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) ./cmd/rst-to-mdx

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

smoke: build
	./smoke-test.sh

# Cross-compile to dist/<os>-<arch>/ for publishing via
# `dpm publish component` (dpm 3.5.1+; see README "Publishing").
#
# Each platform directory gets a component.yaml whose `path:` matches the
# platform's binary name. Windows uses component.windows.yaml (`.exe`
# suffix); every other platform uses the default component.yaml.
release:
	@rm -rf $(DIST)
	@for pair in $(OS_ARCHES); do \
	  os=$${pair%/*}; arch=$${pair#*/}; \
	  out=$(DIST)/$$os-$$arch; \
	  mkdir -p $$out; \
	  ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
	  echo ">> $$os/$$arch"; \
	  GOOS=$$os GOARCH=$$arch $(GO) build $(LDFLAGS) -o $$out/$(BINARY)$$ext ./cmd/rst-to-mdx; \
	  if [ "$$os" = "windows" ]; then \
	    cp component.windows.yaml $$out/component.yaml; \
	  else \
	    cp component.yaml $$out/; \
	  fi; \
	  cp LICENSE $$out/; \
	done

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
