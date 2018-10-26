# Copyright (c) 2018 ContentBox Authors.
# Use of this source code is governed by a MIT-style
# license that can be found in the LICENSE file.

NAME=box

VERSION?=0.1.0

COMMIT=$(shell git rev-parse --short HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

DIR_WORKSPACE=$(shell pwd)
DIR_OUTPUTS=${DIR_WORKSPACE}
BIN="${DIR_OUTPUTS}/${NAME}"

LDFLAGS = -ldflags "-X github.com/BOXFoundation/boxd/config.Version=${VERSION} -X github.com/BOXFoundation/boxd/config.GitCommit=${COMMIT} -X github.com/BOXFoundation/boxd/config.GitBranch=${BRANCH}"

export GO15VENDOREXPERIMENT=1
export GO111MODULE=on

BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem
PKGS ?= $(shell go list ./... | grep -v /vendor/)
# Many Go tools take file globs or directories as arguments instead of packages.
PKG_FILES ?= $(shell ls -d  */ | grep -v "vendor") *.go 

# The linting tools evolve with each Go version, so run them only on the latest
# stable release.
GO_VERSION := $(shell go version | cut -d " " -f 3)
GO_MINOR_VERSION := $(word 2,$(subst ., ,$(GO_VERSION)))
LINTABLE_MINOR_VERSIONS := 11
ifneq ($(filter $(LINTABLE_MINOR_VERSIONS),$(GO_MINOR_VERSION)),)
SHOULD_LINT := true
endif


.PHONY: all
all: clean lint test build

.PHONY: dependencies
dependencies:
	@echo "Installing dev tools required by vs code..."
	go get -u github.com/mdempsky/gocode
	go get -u github.com/uudashr/gopkgs/cmd/gopkgs
	go get -u github.com/ramya-rao-a/go-outline
	go get -u github.com/acroca/go-symbols
	go get -u golang.org/x/tools/cmd/guru
	go get -u golang.org/x/tools/cmd/gorename
	go get -u github.com/derekparker/delve/cmd/dlv
	go get -u github.com/rogpeppe/godef
	go get -u golang.org/x/tools/cmd/godoc
	go get -u github.com/sqs/goreturns
	@echo "Installing test dependencies..."
	go get -u github.com/axw/gocov/gocov
	go get -u github.com/mattn/goveralls
	go get -v github.com/cweill/gotests/...
ifdef SHOULD_LINT
	@echo "Installing golint..."
	go get -u github.com/golang/lint/golint
else
	@echo "Not installing golint, since we don't expect to lint on" $(GO_VERSION)
endif

.PHONY: vendor
vendor:
	@echo "Installing libraries to vendor."
	go mod vendor

# Disable printf-like invocation checking due to testify.assert.Error()
VET_RULES := -printf=false

.PHONY: lint
lint:
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
	go clean -testcache
	go test -race $(PKGS)

.PHONY: clean
clean:
	@rm -rf ${BIN}

.PHONY: build
build: lint fullnode 

.PHONY: fullnode
fullnode:		
	go build $(LDFLAGS) -o ${BIN}
