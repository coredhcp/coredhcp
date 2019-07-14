#!/usr/bin/env bash

go get github.com/golangci/golangci-lint/cmd/golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint
golangci-lint run
