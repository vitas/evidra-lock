#!/usr/bin/env bash
# Special test: invalid input → error response.
# Sourced by run_inspector_tests.sh — has access to helpers.

BAD_INPUT='{"actor":{"type":"","id":"","origin":""},"tool":"kubectl","operation":"get","params":{},"context":{}}'

case "$MODE" in
    local|hosted)
        local raw_output
        raw_output=$(inspector_call_tool "validate" "$BAD_INPUT" 2>&1 || true)

        local body
        body=$(echo "$raw_output" | extract_body) || {
            fail "schema_error/extract" "failed to extract body from error response"
            return
        }

        # Assert ok=false
        local ok
        ok=$(echo "$body" | jq -r '.ok')
        if [[ "$ok" == "false" ]]; then
            pass "schema_error/ok_false"
        else
            fail "schema_error/ok_false" "expected ok=false, got $ok"
        fi

        # Assert error.code=invalid_input
        local error_code
        error_code=$(echo "$body" | jq -r '.error.code // empty')
        if [[ "$error_code" == "invalid_input" ]]; then
            pass "schema_error/error_code"
        else
            fail "schema_error/error_code" "expected error.code=invalid_input, got '$error_code'"
        fi

        # Assert error.message mentions "actor"
        local error_msg
        error_msg=$(echo "$body" | jq -r '.error.message // empty')
        if echo "$error_msg" | grep -qi "actor"; then
            pass "schema_error/error_mentions_actor"
        else
            fail "schema_error/error_mentions_actor" "expected error.message to mention 'actor', got '$error_msg'"
        fi

        # Assert policy.allow=false
        local allow
        allow=$(echo "$body" | jq -r '.policy.allow')
        if [[ "$allow" == "false" ]]; then
            pass "schema_error/policy_deny"
        else
            fail "schema_error/policy_deny" "expected policy.allow=false, got $allow"
        fi

        # Assert policy.risk_level=high
        local risk
        risk=$(echo "$body" | jq -r '.policy.risk_level // empty')
        if [[ "$risk" == "high" ]]; then
            pass "schema_error/risk_high"
        else
            fail "schema_error/risk_high" "expected policy.risk_level=high, got '$risk'"
        fi
        ;;
    rest)
        local http_code
        http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${EVIDRA_API_URL}/v1/validate" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${EVIDRA_API_KEY}" \
            -d "$BAD_INPUT" 2>/dev/null)
        if [[ "$http_code" == "400" ]]; then
            pass "schema_error/http_400"
        else
            fail "schema_error/http_400" "expected HTTP 400, got $http_code"
        fi
        ;;
esac
