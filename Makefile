BUNDLE_PATH ?= ./policy/bundles/ops-v0.1

# Prevent Go toolchain auto-download in offline/dev environments.
# Go 1.24.6 is required (set by go.mod); set GOTOOLCHAIN=off to fail fast
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

tidy:
	go mod tidy
