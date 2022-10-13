GOPATH ?= $(shell go env GOPATH)

build:
	go build -o bin/tester
	cd client/cmd && go build -o ../../bin/cli