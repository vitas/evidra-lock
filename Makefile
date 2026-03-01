BUNDLE_PATH ?= ./policy/bundles/ops-v0.1
POLICY_CATALOG_PATH ?= POLICY_CATALOG.md

# Prevent Go toolchain auto-download in offline/dev environments.
# go.mod pins `toolchain go1.24.6`; set GOTOOLCHAIN=off to fail fast
# instead of silently attempting a network download.
export GOTOOLCHAIN ?= off

.PHONY: test fmt lint build tidy skill skill-check test-skill-e2e test-skill-e2e-full test-skill-e2e-weekly

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

build:
	go build -o bin/evidra ./cmd/evidra
	go build -o bin/evidra-mcp ./cmd/evidra-mcp
	go build -o bin/evidra-api ./cmd/evidra-api

tidy:
	go mod tidy

skill:
	mkdir -p skills/evidra-infra-safety/references
	cp $(POLICY_CATALOG_PATH) skills/evidra-infra-safety/references/policy-rules.md

skill-check:
	diff $(POLICY_CATALOG_PATH) skills/evidra-infra-safety/references/policy-rules.md

test-skill-e2e:
	SCENARIOS=p0 bash tests/e2e/run_e2e.sh

test-skill-e2e-full:
	SCENARIOS=all bash tests/e2e/run_e2e.sh

test-skill-e2e-weekly:
	SCENARIOS=all RETRY_ALL=3 bash tests/e2e/run_e2e.sh
