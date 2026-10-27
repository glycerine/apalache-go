GO ?= go
VERSION ?= dev
BUILD_ROOT ?= build
DIST_ROOT ?= dist
GOOS_VALUE := $(shell $(GO) env GOOS)
GOARCH_VALUE := $(shell $(GO) env GOARCH)
RELEASE_NAME := apalache-go_$(VERSION)_$(GOOS_VALUE)-$(GOARCH_VALUE)
STAGING_DIR := $(BUILD_ROOT)/$(RELEASE_NAME)
ARCHIVE := $(DIST_ROOT)/$(RELEASE_NAME).tar.gz

.PHONY: build dist test test-race test-oracle test-corpus test-release

build:
	mkdir -p $(BUILD_ROOT)
	GOFLAGS='-p=8' $(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o $(BUILD_ROOT)/apalache ./cmd/apalache

dist: build
	mkdir -p $(STAGING_DIR) $(DIST_ROOT)
	cp $(BUILD_ROOT)/apalache $(STAGING_DIR)/apalache
	cp ../LICENSE ../NOTICE ../README.md $(STAGING_DIR)/
	tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner -czf $(ARCHIVE) -C $(BUILD_ROOT) $(RELEASE_NAME)

test:
	GOFLAGS='-p=8' $(GO) test ./...

test-race:
	GOFLAGS='-p=8' $(GO) test -race ./...

test-oracle: build
	APALACHE_GO=$(abspath $(BUILD_ROOT)/apalache) GOFLAGS='-p=8' $(GO) test -tags=oracle -run TestScalaDifferentialSafetyManifest .

test-corpus:
	GOFLAGS='-p=8' $(GO) test -tags=corpus -run TestAnnotatedTopLevelScalaFixtureCorpus .

test-release:
	GOFLAGS='-p=8' $(GO) test -tags=release -run TestReleaseArchiveRunsWithoutSourceOrExternalRuntime .
