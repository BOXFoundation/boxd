# Copyright (c) 2018 ContentBox Authors.
# Use of this source code is governed by a MIT-style
# license that can be found in the LICENSE file.

NAME=box

VERSION?=0.1.0

COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

DIR_WORKSPACE=$(shell pwd)
DIR_OUTPUTS=${DIR_WORKSPACE}
BIN="${DIR_OUTPUTS}/${NAME}"

LDFLAGS = -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.branch=${BRANCH}"

export GO15VENDOREXPERIMENT=1

BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem
PKGS ?= $(shell glide novendor)
# Many Go tools take file globs or directories as arguments instead of packages.
PKG_FILES ?= $(shell ls -d  */ | grep -v "vendor") *.go 

# The linting tools evolve with each Go version, so run them only on the latest
# stable release.
GO_VERSION := $(shell go version | cut -d " " -f 3)
GO_MINOR_VERSION := $(word 2,$(subst ., ,$(GO_VERSION)))
LINTABLE_MINOR_VERSIONS := 10
ifneq ($(filter $(LINTABLE_MINOR_VERSIONS),$(GO_MINOR_VERSION)),)
SHOULD_LINT := true
endif


.PHONY: all
all: clean lint test build

.PHONY: dependencies
dependencies:
	@echo "Installing Glide and locked dependencies..."
	glide --version || go get -u -f github.com/Masterminds/glide
	glide install
	@echo "Installing test dependencies..."
	go install ./vendor/github.com/axw/gocov/gocov
	go install ./vendor/github.com/mattn/goveralls
ifdef SHOULD_LINT
	@echo "Installing golint..."
	go install ./vendor/github.com/golang/lint/golint
else
	@echo "Not installing golint, since we don't expect to lint on" $(GO_VERSION)
endif

# Disable printf-like invocation checking due to testify.assert.Error()
VET_RULES := -printf=false

.PHONY: lint
lint:
	echo $(PKG_FILES)
ifdef SHOULD_LINT
	@rm -rf lint.log
	@echo "Checking formatting..."
	@gofmt -d -s $(PKG_FILES) 2>&1 | tee lint.log
	@echo "Installing test dependencies for vet..."
	@go test -i $(PKGS)
	@echo "Checking vet..."
	@$(foreach dir,$(PKG_FILES),go tool vet $(VET_RULES) $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking lint..."
	@$(foreach dir,$(PKGS),golint $(dir) 2>&1 | tee -a lint.log;)
	@echo "Checking for unresolved FIXMEs..."
	@git grep -i fixme | grep -v -e vendor -e Makefile | tee -a lint.log
	@echo "Checking for license headers..."
	@./check_license.sh | tee -a lint.log
	@[ ! -s lint.log ]
else
	@echo "Skipping linters on" $(GO_VERSION)
endif

.PHONY: test
test:
	go test -race $(PKGS)

.PHONY: clean
clean:
	@rm -rf ${BIN}

.PHONY: build
build: fullnode 

.PHONY: fullnode
fullnode:		
	go build $(LDFLAGS) -o ${BIN} main.go
