POLICY_PATH ?= ./policy/profiles/ops-v0.1/policy.rego
POLICY_DATA_PATH ?= ./policy/profiles/ops-v0.1/data.json
EVIDENCE_PATH ?= ./data/evidence
DATA_ARG := $(if $(wildcard $(POLICY_DATA_PATH)),--data $(POLICY_DATA_PATH),)

.PHONY: test fmt lint build tidy run-mcp policy-sim-echo policy-sim-kubectl-deny evidence-verify evidence-violations evidence-export

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run

build:
	go build ./...

tidy:
	go mod tidy

run-mcp:
	@echo "Evidence log path: $(EVIDENCE_PATH)"
	EVIDRA_POLICY_PATH=$(POLICY_PATH) EVIDRA_EVIDENCE_PATH=$(EVIDENCE_PATH) $(if $(wildcard $(POLICY_DATA_PATH)),EVIDRA_POLICY_DATA_PATH=$(POLICY_DATA_PATH),) go run ./cmd/evidra-mcp

policy-sim-echo:
	go run ./cmd/evidra-policy-sim --policy $(POLICY_PATH) --input ./examples/invocations/allowed_echo_dev.json $(DATA_ARG)

policy-sim-kubectl-deny:
	go run ./cmd/evidra-policy-sim --policy $(POLICY_PATH) --input ./examples/invocations/denied_kubectl_delete_prod.json $(DATA_ARG)

evidence-verify:
	go run ./cmd/evidra-evidence verify --evidence $(EVIDENCE_PATH)

evidence-violations:
	go run ./cmd/evidra-evidence violations --evidence $(EVIDENCE_PATH) --min-risk high

evidence-export:
	go run ./cmd/evidra-evidence export --evidence $(EVIDENCE_PATH) --out ./audit-pack.tar.gz --policy $(POLICY_PATH) $(DATA_ARG)
