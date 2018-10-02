# Copyright (c) 2018 ContentBox Authors.
# Use of this source code is governed by a MIT-style
# license that can be found in the LICENSE file.

PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

all: $(GO)

%.pb.go: %.proto
	 protoc --gogofaster_out=plugins=grpc:. $<

clean:
	rm *.pb.go