# Copyright (c) 2018 ContentBox Authors.
# Use of this source code is governed by a MIT-style
# license that can be found in the LICENSE file.

PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

.PHONY: all
all: clean build

.PHONY: build
build: $(GO)

.PHONY: %.pb.go
%.pb.go: %.proto
	protoc --gogo_out=plugins=grpc:. $<

.PHONY: clean
clean:
	@rm -f *.pb.go