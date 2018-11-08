FROM golang:1.11.1-alpine as go-rocksdb

RUN apk add --update --no-cache build-base linux-headers coreutils git cmake bash perl vim expect
RUN apk add --update --no-cache zlib-dev bzip2-dev snappy-dev lz4-dev zstd-dev

# installing latest gflags
RUN cd /tmp && \
    go get github.com/tools/godep && \
    git clone https://github.com/gflags/gflags.git && \
    cd gflags && \
    mkdir build && \
    cd build && \
    cmake -DBUILD_SHARED_LIBS=1 -DGFLAGS_INSTALL_SHARED_LIBS=1 .. && \
    make install && \
    make install DESTDIR=/dist

# Install Rocksdb
RUN cd /tmp && \
    git clone https://github.com/facebook/rocksdb.git /tmp/rocksdb --depth 1 --single-branch --branch v5.15.10 && \
    cd rocksdb && \
    make shared_lib && make install-shared

#Cleanup
RUN rm -R /tmp/gflags/ && \
    rm -R /tmp/rocksdb/


#############################
# image for builder
#############################
FROM go-rocksdb as builder

ENV CGO_CFLAGS  "-I/usr/local/include"
ENV CGO_LDFLAGS "-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy"

#get dependancies
#RUN GO111MODULE=on go get -d -v ./...
#ENV http_proxy http://127.0.0.1:1087
#ENV https_proxy http://127.0.0.1:1087
#RUN GO111MODULE=on go mod vendor

COPY . $GOPATH/src/github.com/BOXFoundation/boxd/
WORKDIR $GOPATH/src/github.com/BOXFoundation/boxd/

#build the binary
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /app/boxd/box


#############################
# image for boxd
#############################
FROM go-rocksdb

RUN mkdir -p /app/boxd/.devconfig /app/boxd/keyfile
WORKDIR /app/boxd/
#RUN /sbin/ldconfig && mkdir .devconfig && mkdir keyfile

COPY --from=builder /app/boxd/box .
COPY startnode .
COPY keyfile/* keyfile/
COPY tests/newaccount.sh .
CMD ["/bin/bash", "startnode", "1"]
