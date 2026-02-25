BUNDLE_PATH ?= ./policy/bundles/ops-v0.1

# Prevent Go toolchain auto-download in offline/dev environments.
# go.mod pins `toolchain go1.24.6`; set GOTOOLCHAIN=off to fail fast
# instead of silently attempting a network download.
export GOTOOLCHAIN ?= off

.PHONY: test fmt lint build tidy

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

build:
	go build -o bin/evidra ./cmd/evidra
	go build -o bin/evidra-mcp ./cmd/evidra-mcp

tidy:
	go mod tidy
