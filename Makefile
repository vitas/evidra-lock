POLICY_PATH ?= ./policy/profiles/ops-v0.1/policy.rego
POLICY_DATA_PATH ?= ./policy/profiles/ops-v0.1/data.json

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
