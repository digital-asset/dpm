# CPU, OS and for publishing executing built assistant:
ifeq ($(CI), true)
 OS_TYPE = linux
 CPU_ARCH = amd64
else
CPU_ARCH ?= $(shell uname -m)
OS_TYPE  ?= $(shell uname -s | tr A-Z a-z)
endif

# Controls for assistant execution:
export DPM_LOG_LEVEL ?= debug
ASSISTANT_ARGS ?=

# Build locally in one go:
local-build:
	go mod download
	go run cmd/dpm/main.go --version
	go test -v ./...
	GIT_COMMIT_COUNT=$(shell git rev-list --count HEAD) goreleaser --snapshot --clean

# Publish built artifacts to GAR registry:
publish-release-to-gar: VERSION = $(shell cat dist/metadata.json | jq -r '.["version"]')
publish-release-to-gar:
	dist/${OS_TYPE}/${CPU_ARCH}/./dpm publish component oci://ghcr.io/dasormeter/components/dpm:$(VERSION) $(ASSISTANT_ARGS) -g \
		-p linux/arm64=dist/linux/arm64/dpm \
		-p linux/amd64=dist/linux/amd64/dpm \
		-p darwin/arm64=dist/darwin/arm64/dpm \
		-p darwin/amd64=dist/darwin/amd64/dpm \
		-p windows/amd64=dist/windows/amd64/dpm.exe

# Clean Up!
clean:
	rm -rfv dist/

.PHONY: generate-cli-ref
generate-cli-ref:
	rm -rf docs-internal/src/cli
	go run cmd/docs/docs.go docs-internal/src/cli --format=rst

.PHONY: check-stale-docs
check-stale-docs:
	@echo "Checking for stale CLI docs..."
	@$(MAKE) generate-cli-ref
	@git diff --quiet -- docs-internal/src/cli || ( \
		echo "❌ CLI docs are stale. Run 'make generate-cli-ref' and commit the changes."; \
		git --no-pager diff -- docs-internal/src/cli; \
		exit 1; \
	)
	@echo "✅ CLI docs are up to date."

.PHONY: generate-sphinx
generate-sphinx:
	rm -rf docs-internal/generated
	sphinx-build -vvv -b html docs-internal/src/ docs-internal/generated/html

.PHONY: run-internal-docs
run-internal-docs: generate-cli-ref generate-sphinx
	open docs-internal/generated/html/index.html

.PHONY: run-docs
run-docs: run-internal-docs
	rm -rf docs/generated
	sphinx-build -vvv -b html docs/src/ docs/generated/html
	open docs/generated/html/index.html
