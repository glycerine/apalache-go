GO ?= go
VERSION ?= dev
BUILD_ROOT ?= build
DIST_ROOT ?= dist
TASKSET_OK := $(shell if command -v taskset >/dev/null 2>&1 && taskset -c 1-8 true >/dev/null 2>&1; then echo yes; fi)
CPU_RUN := $(if $(TASKSET_OK),taskset -c 1-8,)
GOOS_VALUE := $(shell $(GO) env GOOS)
GOARCH_VALUE := $(shell $(GO) env GOARCH)
RELEASE_NAME := apalache-go_$(VERSION)_$(GOOS_VALUE)-$(GOARCH_VALUE)
STAGING_DIR := $(BUILD_ROOT)/$(RELEASE_NAME)
ARCHIVE := $(DIST_ROOT)/$(RELEASE_NAME).tar.gz

.PHONY: build dist test vet verify test-race test-fuzz-smoke test-oracle test-corpus test-release test-deterministic

FUZZ_TIME ?= 10s

build:
	mkdir -p $(BUILD_ROOT)
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o $(BUILD_ROOT)/apalache ./cmd/apalache

dist: build
	mkdir -p $(STAGING_DIR) $(DIST_ROOT)
	cp $(BUILD_ROOT)/apalache $(STAGING_DIR)/apalache
	cp ../LICENSE ../NOTICE ../README.md $(STAGING_DIR)/
	tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner -czf $(ARCHIVE) -C $(BUILD_ROOT) $(RELEASE_NAME)

test:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) test ./...

vet:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) vet ./...

verify: test vet build

test-race:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) test -race ./...

test-fuzz-smoke:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) test -run '^$$' -fuzz '^FuzzTypeParserRoundTrip$$' -fuzztime=$(FUZZ_TIME) -parallel=8 .

test-oracle: build
	APALACHE_GO=$(abspath $(BUILD_ROOT)/apalache) GOFLAGS='-p=8' $(CPU_RUN) $(GO) test -tags=oracle -run TestScalaDifferentialSafetyManifest .

test-corpus:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) test -tags=corpus -run TestAnnotatedTopLevelScalaFixtureCorpus .

test-release:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) test -tags=release -run TestReleaseArchiveRunsWithoutSourceOrExternalRuntime .

test-deterministic:
	GOFLAGS='-p=8' $(CPU_RUN) $(GO) test -tags=deterministic -run TestStandaloneCounterexampleIsDeterministicAcrossWorkerCounts .
