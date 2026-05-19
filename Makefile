GO_BUILD_ENV :=
GO_BUILD_FLAGS :=
MODULE_BINARY := bin/example-visualizations-go
VERSION := $(shell cat VERSION 2>/dev/null)
PLATFORM ?= linux/amd64

$(MODULE_BINARY): Makefile go.mod go.sum *.go cmd/module/*.go
	$(GO_BUILD_ENV) go build $(GO_BUILD_FLAGS) -o $(MODULE_BINARY) cmd/module/main.go

.PHONY: lint
lint:
	gofmt -s -w .

.PHONY: update
update:
	go get go.viam.com/rdk@latest
	go mod tidy

.PHONY: test
test:
	go test ./...

module.tar.gz: meta.json $(MODULE_BINARY) VERSION assets/*
	tar czf $@ meta.json $(MODULE_BINARY) assets/

.PHONY: module
module: test module.tar.gz

.PHONY: all
all: test module.tar.gz

.PHONY: upload
upload: module.tar.gz
	viam module upload --version=$(VERSION) --platform=$(PLATFORM) module.tar.gz

.PHONY: setup
setup:
	go mod tidy
