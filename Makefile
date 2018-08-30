NAME=box

DIR_WORKSPACE=$(shell pwd)
DIR_OUTPUTS=${DIR_WORKSPACE}/outputs
BIN="${DIR_OUTPUTS}/${NAME}"

VER_COMMIT=$(shell git rev-parse HEAD)
VER_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)


default: clean deps fullnode

build: fullnode 

fullnode:
	mkdir ${DIR_OUTPUTS}
	go build -o ${BIN} main.go

clean:
	rm -rf ${DIR_OUTPUTS}

deps:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure
