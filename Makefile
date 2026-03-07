BUNDLE_PATH ?= ./policy/bundles/ops-v0.1
POLICY_CATALOG_PATH ?= POLICY_CATALOG.md

# Prevent Go toolchain auto-download in offline/dev environments.
# go.mod pins `toolchain go1.24.6`; set GOTOOLCHAIN=off to fail fast
# instead of silently attempting a network download.
export GOTOOLCHAIN ?= off

.PHONY: test fmt lint build tidy skill skill-check test-skill-e2e test-skill-e2e-full test-skill-e2e-weekly test-corpus validate-corpus corpus-coverage test-mcp-inspector test-mcp-inspector-hosted test-mcp-inspector-rest

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

build:
	go build -o bin/evidra-lock ./cmd/evidra
	go build -o bin/evidra-lock-mcp ./cmd/evidra-mcp
	go build -o bin/evidra-lock-api ./cmd/evidra-api

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

test-corpus:
	go test ./tests/corpus/...

validate-corpus:
	bash tests/corpus/scripts/validate_corpus.sh

corpus-coverage:
	bash tests/corpus/scripts/coverage_report.sh

test-mcp-inspector:
	bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-hosted:
	EVIDRA_TEST_MODE=hosted EVIDRA_MCP_URL=$${EVIDRA_MCP_URL:-https://evidra.samebits.com/mcp} EVIDRA_API_KEY=$${EVIDRA_API_KEY} bash tests/inspector/run_inspector_tests.sh

test-mcp-inspector-rest:
	EVIDRA_TEST_MODE=rest EVIDRA_API_URL=$${EVIDRA_API_URL:-https://api.evidra.rest} EVIDRA_API_KEY=$${EVIDRA_API_KEY} bash tests/inspector/run_inspector_tests.sh
