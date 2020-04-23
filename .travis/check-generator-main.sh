#!/usr/bin/env bash

set -ex

# Build the generator version
pushd cmds/coredhcp-generator
go build
generated=$(./coredhcp-generator -from core-plugins.txt)/coredhcp.go
popd

gofmt -w $generated
diff -u $generated cmds/coredhcp/main.go
