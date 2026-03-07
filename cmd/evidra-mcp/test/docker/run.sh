#!/bin/sh
# Docker integration test for evidra-mcp zero-config embedded bundle.
# Usage: ./run.sh [image]
# Requires: docker, jq
set -e

IMAGE="${1:-evidra-lock-mcp:zero-config}"
DIR="$(cd "$(dirname "$0")" && pwd)"
PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); printf "  \033[32mPASS\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); printf "  \033[31mFAIL\033[0m %s\n" "$1"; }

send() {
    # Concatenate init handshake + test payload, then hold stdin open
    # long enough for the server to process all messages before EOF.
    { cat "$DIR/../testdata/init.jsonl" "$1"; sleep 1; } \
      | docker run --rm -i "$IMAGE" 2>"$STDERR"
}

STDERR=$(mktemp)
STDOUT=$(mktemp)
trap 'rm -f "$STDERR" "$STDOUT"' EXIT

# --- 1. Startup + stderr ---
printf "1. Startup\n"
send "$DIR/../testdata/validate_pass.jsonl" >"$STDOUT"

if grep -q "using built-in ops-v0.1 bundle" "$STDERR"; then
    pass "stderr: embedded bundle notice"
else
    fail "stderr missing 'using built-in ops-v0.1 bundle'"
    cat "$STDERR" >&2
fi

# --- 2. PASS case ---
printf "2. PASS case (kubectl get)\n"
RESULT=$(grep '"id":2' "$STDOUT" | jq -r '.result.structuredContent')
ALLOW=$(echo "$RESULT" | jq -r '.policy.allow')
OK=$(echo "$RESULT" | jq -r '.ok')
EVENT=$(echo "$RESULT" | jq -r '.event_id')

[ "$OK" = "true" ]    && pass "ok=true"          || fail "ok=$OK, want true"
[ "$ALLOW" = "true" ] && pass "allow=true"        || fail "allow=$ALLOW, want true"
[ -n "$EVENT" ] && [ "$EVENT" != "null" ] \
                       && pass "event_id present"  || fail "event_id missing"

# --- 3. DENY case ---
printf "3. DENY case (kubectl delete kube-system)\n"
send "$DIR/../testdata/validate_deny.jsonl" >"$STDOUT" 2>"$STDERR"
RESULT=$(grep '"id":2' "$STDOUT" | jq -r '.result.structuredContent')
ALLOW=$(echo "$RESULT" | jq -r '.policy.allow')
OK=$(echo "$RESULT" | jq -r '.ok')
RULES=$(echo "$RESULT" | jq -r '.rule_ids[]?' 2>/dev/null)
HINTS=$(echo "$RESULT" | jq -r '.hints | length')

[ "$OK" = "false" ]    && pass "ok=false"         || fail "ok=$OK, want false"
[ "$ALLOW" = "false" ] && pass "allow=false"       || fail "allow=$ALLOW, want false"
echo "$RULES" | grep -q "k8s.protected_namespace" \
                        && pass "rule_id present"  || fail "rule_ids=$RULES, want k8s.protected_namespace"
[ "$HINTS" -gt 0 ] 2>/dev/null \
                        && pass "hints present ($HINTS)" || fail "no hints returned"

# --- 4. SIGTERM clean exit ---
printf "4. SIGTERM\n"
docker rm -f evidra-sig-test >/dev/null 2>&1 || true
# Hold stdin open so the server blocks on read instead of exiting on EOF.
sleep 30 | docker run --rm -i --name evidra-sig-test "$IMAGE" >/dev/null 2>&1 &
PID=$!
sleep 2
if docker kill --signal SIGTERM evidra-sig-test >/dev/null 2>&1; then
    pass "SIGTERM accepted"
else
    fail "SIGTERM failed (container already exited?)"
fi
sleep 1
# Verify container is gone.
if docker inspect evidra-sig-test >/dev/null 2>&1; then
    fail "container still running after SIGTERM"
    docker rm -f evidra-sig-test >/dev/null 2>&1
else
    pass "container stopped"
fi
kill "$PID" 2>/dev/null || true
wait "$PID" 2>/dev/null || true

# --- Summary ---
printf "\n%d passed, %d failed\n" "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
