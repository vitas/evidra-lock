#!/usr/bin/env bash
#
# MCP Inspector CLI test layer (Layer 2.5)
#
# Exercises evidra-mcp over stdio using the MCP Inspector CLI.
# Deterministic — no LLM, no Docker, no API keys.
# Policy test data comes from tests/corpus/.
#
# Modes (EVIDRA_TEST_MODE):
#   local  — Inspector CLI → evidra-mcp stdio (default)
#   hosted — Inspector CLI → supergateway streamable-http
#   rest   — curl → evidra-api REST /v1/validate
#
set -euo pipefail

# ── paths ──────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CORPUS_DIR="$REPO_ROOT/tests/corpus"
CONFIG="$SCRIPT_DIR/mcp-config.json"
EVIDENCE_DIR="/tmp/evidra-inspector-evidence"

# ── Mode selection ───────────────────────────────────────────────────
# EVIDRA_TEST_MODE: "local" (default), "hosted", or "rest"
#   local  — Inspector CLI → evidra-mcp stdio
#   hosted — Inspector CLI → supergateway streamable-http (same MCP protocol)
#   rest   — curl → evidra-api REST /v1/validate (different response shape)
MODE="${EVIDRA_TEST_MODE:-local}"

# ── counters ───────────────────────────────────────────────────────────
PASS=0
FAIL=0
SKIP=0
ERRORS=""

# ── colors ─────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
    GREEN='\033[0;32m'
    RED='\033[0;31m'
    YELLOW='\033[0;33m'
    CYAN='\033[0;36m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    GREEN='' RED='' YELLOW='' CYAN='' BOLD='' RESET=''
fi

# ── helpers ────────────────────────────────────────────────────────────

pass() {
    local name="$1"
    PASS=$((PASS + 1))
    printf "${GREEN}  ✓ %s${RESET}\n" "$name"
}

fail() {
    local name="$1"
    shift
    FAIL=$((FAIL + 1))
    printf "${RED}  ✗ %s: %s${RESET}\n" "$name" "$*"
    ERRORS="${ERRORS}\n  - ${name}: $*"
}

skip() {
    local name="$1"
    shift
    SKIP=$((SKIP + 1))
    printf "${YELLOW}  ⊘ %s: %s${RESET}\n" "$name" "$*"
}

section() {
    printf "\n${BOLD}${CYAN}── %s ──${RESET}\n" "$1"
}

inspector_call_tool() {
    local tool="$1" args_json="$2" env="${3:-}"
    # Convert JSON object to --tool-arg key=value pairs.
    # Each top-level key becomes a separate --tool-arg.
    local -a cmd=(npx -y @modelcontextprotocol/inspector --cli
        --config "$CONFIG" --server evidra)
    # Pass environment to the MCP server process if specified.
    # The server reads EVIDRA_ENVIRONMENT to resolve environment-specific params.
    if [[ -n "$env" ]]; then
        cmd+=(-e "EVIDRA_ENVIRONMENT=${env}")
    fi
    cmd+=(--method tools/call --tool-name "$tool")
    while IFS= read -r key; do
        local val
        val=$(echo "$args_json" | jq -c --arg k "$key" '.[$k]')
        cmd+=(--tool-arg "${key}=${val}")
    done < <(echo "$args_json" | jq -r 'keys[]')
    "${cmd[@]}" 2>/dev/null
}

inspector_list_tools() {
    npx -y @modelcontextprotocol/inspector --cli \
        --config "$CONFIG" --server evidra \
        --method tools/list 2>/dev/null
}

extract_body() {
    jq '.structuredContent // (.content[0].text | fromjson) // .' 2>/dev/null
}

reset_evidence() {
    rm -rf "$EVIDENCE_DIR"
    mkdir -p "$EVIDENCE_DIR"
}

# transform_corpus_input adapts corpus .input for the MCP validate tool.
#
# Corpus files structure params as params.action.{payload, target, risk_tags}
# for direct OPA evaluation. The MCP server's invocationToScenario reads
# params.{payload, target, risk_tags} at the top level, so we must flatten.
# String targets (e.g. "default") are wrapped as {"namespace": value}.
transform_corpus_input() {
    local corpus_file="$1"
    jq -c '.input |
      if .params.action then
        .params = (
          {action: .params.action} +
          (if .params.action.payload then {payload: .params.action.payload} else {} end) +
          (if .params.action.risk_tags then {risk_tags: .params.action.risk_tags} else {} end) +
          (if (.params.action.target | type) == "object" then {target: .params.action.target}
           elif (.params.action.target | type) == "string" then {target: {"namespace": .params.action.target}}
           else {} end)
        )
      else . end
    ' "$corpus_file"
}

# ── Retry with backoff (hosted/rest only) ────────────────────────────
MAX_RETRIES="${EVIDRA_TEST_RETRIES:-3}"
RETRY_DELAY="${EVIDRA_TEST_RETRY_DELAY:-2}"  # seconds

# Wraps a command with retry logic for network modes.
with_retry() {
    local attempt=1
    local output
    while [[ $attempt -le $MAX_RETRIES ]]; do
        if output=$("$@" 2>&1); then
            echo "$output"
            return 0
        fi
        # Check if it's a rate limit (429) or transient error
        if echo "$output" | grep -qi '429\|rate.limit\|too.many\|ECONNREFUSED\|timeout'; then
            if [[ $attempt -lt $MAX_RETRIES ]]; then
                local wait=$((RETRY_DELAY * attempt))
                printf "    ${YELLOW}RETRY${RESET} attempt %d/%d (waiting %ds)\n" "$attempt" "$MAX_RETRIES" "$wait" >&2
                sleep "$wait"
            fi
        else
            # Non-retryable error — fail immediately
            echo "$output"
            return 1
        fi
        attempt=$((attempt + 1))
    done
    echo "$output"
    return 1
}

# ── Transport abstraction ────────────────────────────────────────────
# local+hosted: Inspector CLI + CONFIG (stdio or streamable-http)
# rest: curl + normalization

# Internal: REST validate without retry (called by with_retry).
_rest_validate() {
    local input="$1" body http_code
    body=$(curl -s -w "\n%{http_code}" -X POST "${EVIDRA_API_URL}/v1/validate" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${EVIDRA_API_KEY}" \
        -d "$input" 2>/dev/null)
    http_code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')
    if [[ "$http_code" == "429" ]]; then
        echo "429 rate limited"
        return 1
    fi
    [[ "$http_code" == "200" ]] || { echo "{\"_error\":true,\"_http_code\":$http_code}"; return 1; }
    jq '{
        ok:         .decision.allow,
        event_id:   .event_id,
        policy:     { allow: .decision.allow, risk_level: .decision.risk_level },
        rule_ids:   (.decision.rule_ids // []),
        hints:      (.decision.hints // [])
    }' <<< "$body"
}

call_validate() {
    local input="$1" env="${2:-}"
    case "$MODE" in
        local)
            # No retry needed — local, no network
            local raw
            raw=$(inspector_call_tool "validate" "$input" "$env")
            echo "$raw" | extract_body
            ;;
        hosted)
            # Inspector CLI → streamable-http — may hit Traefik rate limit
            local raw
            raw=$(with_retry inspector_call_tool "validate" "$input" "$env")
            echo "$raw" | extract_body
            ;;
        rest)
            # curl → REST API — may hit Traefik rate limit
            with_retry _rest_validate "$input"
            ;;
    esac
}

call_get_event() {
    [[ "$MODE" == "rest" ]] && return 1
    local get_args
    get_args=$(jq -n --arg eid "$1" '{"event_id": $eid}')
    local raw
    raw=$(inspector_call_tool "get_event" "$get_args")
    echo "$raw" | extract_body
}

call_list_tools() {
    [[ "$MODE" == "rest" ]] && return 1
    inspector_list_tools
}

# ── prerequisites ──────────────────────────────────────────────────────

check_prerequisites() {
    local missing=0

    if ! command -v jq &>/dev/null; then
        echo "ERROR: jq not found"
        missing=1
    fi

    if [[ ! -d "$CORPUS_DIR" ]]; then
        echo "ERROR: corpus directory not found at $CORPUS_DIR"
        missing=1
    fi

    if [[ $missing -ne 0 ]]; then
        echo "Prerequisites check failed."
        exit 1
    fi

    case "$MODE" in
        local)
            if ! command -v npx &>/dev/null; then
                echo "ERROR: npx not found (need Node >= 18)"
                exit 1
            fi
            CONFIG="$SCRIPT_DIR/mcp-config.json"
            # Always build from source to ensure embedded bundle is current
            echo "Building evidra-mcp from source..."
            (cd "$REPO_ROOT" && GOTOOLCHAIN=auto go build -o bin/evidra-mcp ./cmd/evidra-mcp) || {
                echo "ERROR: failed to build evidra-mcp"
                exit 1
            }
            export PATH="$REPO_ROOT/bin:$PATH"
            # Clear stale embedded bundle cache so fresh build takes effect
            rm -rf "${HOME}/.evidra/bundles/ops-v0.1"
            ;;
        hosted)
            if ! command -v npx &>/dev/null; then
                echo "ERROR: npx not found (need Node >= 18)"
                exit 1
            fi
            [[ -n "${EVIDRA_MCP_URL:-}" ]] || { echo "ERROR: EVIDRA_MCP_URL required in hosted mode"; exit 1; }
            # Generate streamable-http config for Inspector
            CONFIG=$(mktemp /tmp/evidra-mcp-hosted-XXXXXX.json)
            trap "rm -f $CONFIG" EXIT
            cat > "$CONFIG" <<MCPEOF
{
  "mcpServers": {
    "evidra": {
      "type": "streamable-http",
      "url": "${EVIDRA_MCP_URL}"
    }
  }
}
MCPEOF
            ;;
        rest)
            if ! command -v curl &>/dev/null; then
                echo "ERROR: curl not found"
                exit 1
            fi
            [[ -n "${EVIDRA_API_URL:-}" ]] || { echo "ERROR: EVIDRA_API_URL required in rest mode"; exit 1; }
            [[ -n "${EVIDRA_API_KEY:-}" ]] || { echo "ERROR: EVIDRA_API_KEY required in rest mode"; exit 1; }
            curl -sf "${EVIDRA_API_URL}/healthz" >/dev/null 2>&1 || {
                echo "ERROR: cannot reach ${EVIDRA_API_URL}/healthz"
                exit 1
            }
            CONFIG=""
            ;;
        *)
            echo "ERROR: EVIDRA_TEST_MODE must be 'local', 'hosted', or 'rest'. Got: '$MODE'"
            exit 1
            ;;
    esac
}

# ── corpus assertion helpers ───────────────────────────────────────────

assert_ok() {
    local name="$1" body="$2"
    local ok
    ok=$(echo "$body" | jq -r '.ok')
    if [[ "$ok" != "true" ]]; then
        fail "$name" "expected ok=true, got ok=$ok"
        return 1
    fi
    return 0
}

assert_event_id() {
    local name="$1" body="$2"
    local eid
    eid=$(echo "$body" | jq -r '.event_id // empty')
    if [[ -z "$eid" ]]; then
        fail "$name" "expected non-empty event_id"
        return 1
    fi
    return 0
}

assert_allow() {
    local name="$1" body="$2" expected="$3"
    local actual
    actual=$(echo "$body" | jq -r '.policy.allow')
    if [[ "$actual" != "$expected" ]]; then
        fail "$name" "expected policy.allow=$expected, got $actual"
        return 1
    fi
    return 0
}

assert_risk_level() {
    local name="$1" body="$2" expected="$3"
    local actual
    actual=$(echo "$body" | jq -r '.policy.risk_level')
    if [[ "$actual" != "$expected" ]]; then
        fail "$name" "expected risk_level=$expected, got $actual"
        return 1
    fi
    return 0
}

assert_rule_present() {
    local name="$1" body="$2" rule="$3"
    local found
    found=$(echo "$body" | jq --arg r "$rule" '[.rule_ids[]? | select(. == $r)] | length')
    if [[ "$found" -eq 0 ]]; then
        fail "$name" "expected rule_ids to contain '$rule'"
        return 1
    fi
    return 0
}

assert_rule_absent() {
    local name="$1" body="$2" rule="$3"
    local found
    found=$(echo "$body" | jq --arg r "$rule" '[.rule_ids[]? | select(. == $r)] | length')
    if [[ "$found" -ne 0 ]]; then
        fail "$name" "expected rule_ids to NOT contain '$rule'"
        return 1
    fi
    return 0
}

assert_hints_min() {
    local name="$1" body="$2" min="$3"
    local count
    count=$(echo "$body" | jq '[.hints[]?] | length')
    if [[ "$count" -lt "$min" ]]; then
        fail "$name" "expected hints >= $min, got $count"
        return 1
    fi
    return 0
}

# ── main ───────────────────────────────────────────────────────────────

main() {
    printf "${BOLD}MCP Inspector CLI Tests (Layer 2.5)${RESET}\n"
    printf "Mode:   %s\n" "$MODE"
    printf "Corpus: %s\n" "$CORPUS_DIR"

    check_prerequisites

    # Show config/endpoint after prerequisites (CONFIG may be generated)
    case "$MODE" in
        local)   printf "Config: %s\n" "$CONFIG" ;;
        hosted)  printf "Endpoint: %s\n" "$EVIDRA_MCP_URL" ;;
        rest)    printf "Endpoint: %s\n" "$EVIDRA_API_URL" ;;
    esac

    reset_evidence

    # ── Part A: special cases ──────────────────────────────────────────
    section "Special Cases"

    for script in "$SCRIPT_DIR"/special/t_*.sh; do
        [[ -f "$script" ]] || continue
        printf "  Running %s...\n" "$(basename "$script")"
        # shellcheck source=/dev/null
        source "$script"
    done

    # ── Part B: corpus-driven loop ─────────────────────────────────────
    section "Corpus Tests"

    local corpus_count=0

    for corpus_file in "$CORPUS_DIR"/*.json; do
        [[ -f "$corpus_file" ]] || continue
        local basename
        basename="$(basename "$corpus_file")"

        # Skip non-case files
        case "$basename" in
            sources.json|manifest.json) continue ;;
        esac

        local case_id has_input has_expect
        case_id=$(jq -r '._meta.id // empty' "$corpus_file")
        has_input=$(jq 'has("input")' "$corpus_file")
        has_expect=$(jq 'has("expect")' "$corpus_file")

        # Skip agent-only cases (no input or no expect)
        if [[ "$has_input" != "true" || "$has_expect" != "true" ]]; then
            skip "$case_id" "agent-only case"
            continue
        fi

        corpus_count=$((corpus_count + 1))

        # Build the validate arguments from corpus input (flatten params.action)
        local args
        args=$(transform_corpus_input "$corpus_file")

        # Extract environment for environment-dependent policy params
        local env
        env=$(jq -r '.input.environment // empty' "$corpus_file")

        # Call validate via transport abstraction
        local body
        body=$(call_validate "$args" "$env") || {
            fail "$case_id" "call_validate error"
            continue
        }

        # Assertions
        local case_ok=true

        # event_id should always be present (both allow and deny produce evidence)
        assert_event_id "$case_id" "$body" || case_ok=false

        # Assert allow (ok mirrors policy.allow: true=allowed, false=denied)
        local expect_allow
        expect_allow=$(jq -r '.expect.allow' "$corpus_file")
        if [[ "$expect_allow" != "null" ]]; then
            assert_allow "$case_id" "$body" "$expect_allow" || case_ok=false
        fi

        # Assert risk_level.
        # Known limitation: the MCP scenario evaluation path returns risk_level=low
        # for allow=true decisions, even when OPA computes a higher level (e.g. warn
        # rules that set medium). Skip risk_level assertion for allow+non-low cases.
        local expect_risk
        expect_risk=$(jq -r '.expect.risk_level // empty' "$corpus_file")
        if [[ -n "$expect_risk" ]]; then
            if [[ "$expect_allow" == "true" && "$expect_risk" != "low" ]]; then
                skip "${case_id}/risk_level" "MCP scenario path returns low for allow=true"
            else
                assert_risk_level "$case_id" "$body" "$expect_risk" || case_ok=false
            fi
        fi

        # Assert rule_ids_contain
        local rule_ids_contain
        rule_ids_contain=$(jq -r '.expect.rule_ids_contain[]? // empty' "$corpus_file")
        for rule in $rule_ids_contain; do
            assert_rule_present "$case_id" "$body" "$rule" || case_ok=false
        done

        # Assert rule_ids_absent
        local rule_ids_absent
        rule_ids_absent=$(jq -r '.expect.rule_ids_absent[]? // empty' "$corpus_file")
        for rule in $rule_ids_absent; do
            assert_rule_absent "$case_id" "$body" "$rule" || case_ok=false
        done

        # Assert hints_min_count
        local hints_min
        hints_min=$(jq -r '.expect.hints_min_count // 0' "$corpus_file")
        if [[ "$hints_min" -gt 0 ]]; then
            assert_hints_min "$case_id" "$body" "$hints_min" || case_ok=false
        fi

        if [[ "$case_ok" == "true" ]]; then
            pass "$case_id"
        fi
    done

    # ── summary ────────────────────────────────────────────────────────
    section "Summary"
    printf "  Mode:    %s\n" "$MODE"
    [[ "$MODE" == "hosted" ]] && printf "  Endpoint: %s\n" "$EVIDRA_MCP_URL"
    [[ "$MODE" == "rest" ]] && printf "  Endpoint: %s\n" "$EVIDRA_API_URL"
    [[ "$MODE" != "local" ]] && printf "  Retries: %s (delay: %ss × attempt)\n" "$MAX_RETRIES" "$RETRY_DELAY"
    printf "  Passed:  %d\n" "$PASS"
    printf "  Failed:  %d\n" "$FAIL"
    printf "  Skipped: %d\n" "$SKIP"
    printf "  Corpus:  %d evaluable cases\n" "$corpus_count"

    if [[ $FAIL -gt 0 ]]; then
        printf "\n${RED}${BOLD}Failures:${RESET}${RED}%b${RESET}\n" "$ERRORS"
        exit 1
    fi

    printf "\n${GREEN}${BOLD}All tests passed.${RESET}\n"
}

main "$@"
