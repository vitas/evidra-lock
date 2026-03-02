#!/usr/bin/env bash
# Special test: invalid input → error response.
# Sourced by run_inspector_tests.sh — has access to helpers.

BAD_INPUT='{"actor":{"type":"","id":"","origin":""},"tool":"kubectl","operation":"get","params":{},"context":{}}'

case "$MODE" in
    local|hosted)
        local raw_output
        # -32602 is JSON-RPC Invalid params (see docs/PROTOCOL_ERRORS.md).
        raw_output=$(inspector_call_tool "validate" "$BAD_INPUT" "" "1" 2>&1 || true)

        # Assert JSON-RPC invalid params code.
        if echo "$raw_output" | grep -q -- "-32602"; then
            pass "schema_error/jsonrpc_code_-32602"
        else
            fail "schema_error/jsonrpc_code_-32602" "expected JSON-RPC code -32602, got: $raw_output"
        fi

        # Assert error message includes Invalid params wording.
        if echo "$raw_output" | grep -qi "invalid params"; then
            pass "schema_error/invalid_params_text"
        else
            fail "schema_error/invalid_params_text" "expected 'invalid params' in output, got: $raw_output"
        fi

        # Assert output points to actor/origin field issue for debugging clarity.
        if echo "$raw_output" | grep -qi "actor"; then
            pass "schema_error/field_hint_actor"
        else
            fail "schema_error/field_hint_actor" "expected output to reference actor field, got: $raw_output"
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
