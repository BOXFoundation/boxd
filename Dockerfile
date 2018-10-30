#FROM golang:latest
#
#RUN git clone https://github.com/facebook/rocksdb.git /tmp/rocksdb
#WORKDIR /tmp/rocksdb
#
#RUN apt-get update
#RUN apt-get -y install build-essential libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev
#
#RUN make shared_lib && make install
#RUN rm -rf /tmp/rocksdb

FROM rocksdb_shared as builder

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


FROM rocksdb_shared

WORKDIR /app/boxd/
RUN /sbin/ldconfig && mkdir -p .devconfig/ws{1,2,3}

COPY --from=builder /app/boxd/box .
COPY startnode .
COPY .devconfig/.box-*.yaml .devconfig/
COPY .devconfig/ws* .devconfig/

ENTRYPOINT ["/bin/bash"]
CMD ["startnode", "1"]
