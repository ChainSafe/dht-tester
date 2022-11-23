GOPATH ?= $(shell go env GOPATH)

build:
	go build -o bin/tester
	go build -o bin/ ./cmd/... 