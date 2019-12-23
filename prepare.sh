#!/bin/bash
set -euf -o pipefail

# build web ui
cd pkg/webui
yarn
yarn build
cd -

# embed webui
go get github.com/GeertJohan/go.rice/rice
rice embed-go -i ./cmd/server
rice embed-go -i ./pkg/store/postgres