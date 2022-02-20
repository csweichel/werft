#!/bin/sh

go get github.com/golang/protobuf/protoc-gen-go@v1.3.5
protoc -I. --go_out=plugins=grpc:. *.proto
mv github.com/csweichel/werft/pkg/api/v1/*.go .
rm -r github.com

go install github.com/golang/mock/mockgen@v1.6.0
mkdir -p mock
mockgen -package=mock -source werft.pb.go > mock/mock.tmp
mv mock/mock.tmp mock/mock.go