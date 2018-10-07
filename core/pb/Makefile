PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

.PHONY: all
all: dependencies clean build

.PHONY: dependencies
dependencies:
	@echo "Installing gRPC tools..." # TODO work around build error on GO111MODULE=on...
	@-GO111MODULE=off go get -u github.com/gogo/protobuf/protoc-gen-gogofaster &>/dev/null
	@-GO111MODULE=off go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway &>/dev/null
	@-GO111MODULE=off go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger &>/dev/null

.PHONY: build
build: $(GO)

.PHONY: %.pb.go
%.pb.go: %.proto
	protoc -I. -I$(GOPATH)/src \
		-I../../core/pb \
		-I$(GOPATH)/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--gogofaster_out=plugins=grpc:. \
		--grpc-gateway_out=logtostderr=true:. \
		$<

.PHONY: clean
clean:
	@rm -f *.pb.go *.pb.gw.go