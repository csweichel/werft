#!/bin/sh

go get github.com/golang/protobuf/protoc-gen-go@v1.3.5
protoc -I. -I../../ --go_out=plugins=grpc:. *.proto