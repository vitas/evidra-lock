# P0 Docker Integration — Zero-Config Verification

**Requires:** Docker, `jq`

## Automated

```sh
docker build -t evidra-mcp:zero-config .
cmd/evidra-mcp/test/docker/run.sh evidra-mcp:zero-config
```

## What it tests

| # | Test | Fixture | Assert |
|---|---|---|---|
| 1 | Startup | `testdata/init.jsonl` | stderr: `using built-in ops-v0.1 bundle` |
| 2 | PASS | `testdata/validate_pass.jsonl` | `ok=true`, `allow=true`, `event_id` present |
| 3 | DENY | `testdata/validate_deny.jsonl` | `ok=false`, `allow=false`, `k8s.protected_namespace`, hints > 0 |
| 4 | SIGTERM | — | clean exit code 0 |

## Test files

```
cmd/evidra-mcp/test/docker/
├── run.sh                          test runner
└── testdata/
    ├── init.jsonl                  MCP handshake (initialize + initialized)
    ├── validate_pass.jsonl         kubectl get — expected PASS
    └── validate_deny.jsonl         kubectl delete kube-system — expected DENY
```

## Failure triage

| Symptom | Action |
|---|---|
| Image build fails | `go mod tidy` and commit `go.sum` |
| Validate returns error | Check stderr — evidence path write failure |
| No stdout response | Ensure `docker run -i` (stdin open) |
| SIGTERM hangs | Check Go signal handling in `main.go` |
