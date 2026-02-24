BUNDLE_PATH ?= ./policy/bundles/ops-v0.1

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
