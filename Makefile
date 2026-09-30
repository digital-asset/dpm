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

# Publish built artifacts to registry:
publish-release-to-gar: VERSION = $(shell cat dist/metadata.json | jq -r '.["version"]')
publish-release-to-gar:
	dist/${OS_TYPE}/${CPU_ARCH}/./dpm repo publish-dpm $(VERSION) $(ASSISTANT_ARGS) -g \
		-p linux/arm64=dist/linux/arm64/dpm \
		-p linux/amd64=dist/linux/amd64/dpm \
		-p darwin/arm64=dist/darwin/arm64/dpm \
		-p darwin/amd64=dist/darwin/amd64/dpm \
		-p windows/amd64=dist/windows/amd64/dpm.exe

# Clean Up!
clean:
	rm -rfv dist/


