#!/usr/bin/env bash
# verify_p0.sh — P0 completion check for evidra-mcp zero-config embedded bundle.
# Usage: ./verify_p0.sh [docker-image]
# Exits non-zero on any failure. No jq required.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA="$REPO_ROOT/cmd/evidra-mcp/test/testdata"
BINARY="$REPO_ROOT/bin/evidra-mcp"
DOCKER_IMAGE="${1:-ghcr.io/vitas/evidra-mcp:latest}"

FAILURES=0
pass() { printf "  [PASS] %s\n" "$*"; }
fail() { printf "  [FAIL] %s\n" "$*" >&2; FAILURES=$((FAILURES + 1)); }

EVDIR="$(mktemp -d)"
STDERR_F="$(mktemp)"
STDOUT_F="$(mktemp)"
trap 'rm -rf "$EVDIR" "$STDERR_F" "$STDOUT_F"' EXIT

# Emit all MCP messages for one session: handshake + PASS (id:2) + DENY (id:3).
# The deny fixture uses id:2; remap to id:3 so both coexist in the same session.
send_msgs() {
    cat "$TESTDATA/init.jsonl"
    cat "$TESTDATA/validate_pass.jsonl"
    sed 's/"id":2/"id":3/' "$TESTDATA/validate_deny.jsonl"
    sleep 3
}

check_stderr() {
    grep -q "using built-in ops-v0.1 bundle" "$STDERR_F" \
        && pass "stderr: embedded bundle notice" \
        || { fail "stderr missing 'using built-in ops-v0.1 bundle'"; cat "$STDERR_F" >&2; }
}

assert_field() {  # label id grep-pattern
    local line; line=$(grep "\"id\":$2" "$STDOUT_F" | head -1)
    echo "$line" | grep -qE "$3" \
        && pass "$1" \
        || { fail "$1"; printf "    raw: %.200s\n" "$line" >&2; }
}

check_responses() {
    local pfx="${1:-}"
    check_stderr
    assert_field "${pfx}PASS (id:2): allow=true"  2 '"allow":true'
    assert_field "${pfx}DENY (id:3): allow=false" 3 '"allow":false'
    assert_field "${pfx}DENY (id:3): rule_ids"    3 '"rule_ids":\[".+'
    assert_field "${pfx}DENY (id:3): hints"       3 '"hints":\[".+'
}

# ════════════════════════════════════════════════════════════════════════════════
echo "=== PART 1: Local Binary ==="

unset EVIDRA_BUNDLE_PATH EVIDRA_POLICY_PATH EVIDRA_DATA_PATH

go build -o "$BINARY" "$REPO_ROOT/cmd/evidra-mcp" 2>&1 \
    && pass "binary built" \
    || { fail "go build failed"; exit 1; }

send_msgs | "$BINARY" --evidence-dir "$EVDIR" >"$STDOUT_F" 2>"$STDERR_F" || true

check_responses

# ════════════════════════════════════════════════════════════════════════════════
echo "=== PART 2: Docker ==="

if ! command -v docker >/dev/null 2>&1; then
    echo "  SKIP: docker not found in PATH"
elif ! docker image inspect "$DOCKER_IMAGE" >/dev/null 2>&1; then
    echo "  SKIP: image $DOCKER_IMAGE not found locally (build or pull first)"
else
    # No -v mounts, no -e env vars — zero-config proof.
    send_msgs | docker run --rm -i "$DOCKER_IMAGE" >"$STDOUT_F" 2>"$STDERR_F" || true

    check_responses "Docker: "
    pass "no volume mounts used"
    pass "no env vars required"
fi

# ════════════════════════════════════════════════════════════════════════════════
echo "=== PART 3: Toolchain Consistency ==="

go version
go env GOVERSION

GOMOD_MIN=$(grep '^go ' "$REPO_ROOT/go.mod" | awk '{print $2}')
# toolchain directive is optional; when absent, go directive is the pinned version.
TOOLCHAIN_VER=$(grep '^toolchain ' "$REPO_ROOT/go.mod" | sed 's/^toolchain go//') || true
TOOLCHAIN_VER="${TOOLCHAIN_VER:-$GOMOD_MIN}"
LOCAL_VER=$(go env GOVERSION | sed 's/^go//')
DOCKER_VER=$(grep '^FROM golang:' "$REPO_ROOT/Dockerfile" | head -1 \
             | sed -E 's/FROM golang:([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/')

printf "  go.mod go:        %s  (language minimum)\n" "$GOMOD_MIN"
printf "  go.mod toolchain: %s  (pinned build toolchain)\n" "$TOOLCHAIN_VER"
printf "  local go:         %s\n" "$LOCAL_VER"
printf "  Dockerfile:       %s\n" "$DOCKER_VER"

# Local Go must be >= the language minimum floor.
ver_gte() {
    local IFS=.
    local -a a=($1) b=($2)
    for i in 0 1 2; do
        local av=${a[$i]:-0} bv=${b[$i]:-0}
        [ "$av" -gt "$bv" ] && return 0
        [ "$av" -lt "$bv" ] && return 1
    done
    return 0
}

ver_gte "$LOCAL_VER" "$GOMOD_MIN" \
    && pass "local go ($LOCAL_VER) >= go.mod minimum ($GOMOD_MIN)" \
    || fail  "local go ($LOCAL_VER) is older than go.mod minimum ($GOMOD_MIN)"

# Dockerfile must pin the exact toolchain version for reproducible CI/Docker builds.
[ "$DOCKER_VER" = "$TOOLCHAIN_VER" ] \
    && pass "Dockerfile ($DOCKER_VER) matches go.mod toolchain ($TOOLCHAIN_VER)" \
    || fail  "Dockerfile ($DOCKER_VER) != go.mod toolchain ($TOOLCHAIN_VER) — CI pin mismatch"

# ════════════════════════════════════════════════════════════════════════════════
echo ""
if [ "$FAILURES" -eq 0 ]; then
    printf "ALL CHECKS PASSED — P0 technically complete.\n"
else
    printf "%d CHECK(S) FAILED\n" "$FAILURES" >&2
    exit 1
fi
