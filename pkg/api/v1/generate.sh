#!/bin/sh

go get github.com/golang/protobuf/protoc-gen-go
protoc -I. --go_out=plugins=grpc:. *.proto