#!/usr/bin/env bash
# Special test: validate → get_event round-trip.
# Sourced by run_inspector_tests.sh — has access to helpers.

if [[ "$MODE" == "rest" ]]; then
    skip "chain" "get_event not available in rest mode"
    return
fi

# Reset evidence for isolation
reset_evidence

# Step 1: validate a safe manifest (k8s_safe_manifest_allow seed case)
local validate_args
validate_args=$(transform_corpus_input "$CORPUS_DIR/k8s_safe_manifest_allow.json")

local validate_body
validate_body=$(call_validate "$validate_args") || {
    fail "chain/validate" "call_validate failed"
    return
}

# Assert validate succeeded
local ok
ok=$(echo "$validate_body" | jq -r '.ok')
if [[ "$ok" != "true" ]]; then
    fail "chain/validate_ok" "expected ok=true, got $ok"
    return
fi
pass "chain/validate_ok"

local allow
allow=$(echo "$validate_body" | jq -r '.policy.allow')
if [[ "$allow" != "true" ]]; then
    fail "chain/validate_allow" "expected policy.allow=true, got $allow"
    return
fi
pass "chain/validate_allow"

# Extract event_id
local event_id
event_id=$(echo "$validate_body" | jq -r '.event_id // empty')
if [[ -z "$event_id" ]]; then
    fail "chain/event_id" "validate returned empty event_id"
    return
fi
pass "chain/event_id_present"

# Step 2: get_event with the event_id
local get_body
get_body=$(call_get_event "$event_id") || {
    fail "chain/get_event" "call_get_event failed"
    return
}

# Assert get_event returned ok
local get_ok
get_ok=$(echo "$get_body" | jq -r '.ok')
if [[ "$get_ok" != "true" ]]; then
    fail "chain/get_event_ok" "expected ok=true, got $get_ok"
    return
fi
pass "chain/get_event_ok"

# Assert event_id matches
local returned_eid
returned_eid=$(echo "$get_body" | jq -r '.record.event_id // empty')
if [[ "$returned_eid" == "$event_id" ]]; then
    pass "chain/get_event_id_match"
else
    fail "chain/get_event_id_match" "expected event_id=$event_id, got $returned_eid"
fi

# Assert record has policy_decision.allow=true
local record_allow
record_allow=$(echo "$get_body" | jq -r '.record.policy_decision.allow // empty')
if [[ "$record_allow" == "true" ]]; then
    pass "chain/get_event_allow"
else
    fail "chain/get_event_allow" "expected record.policy_decision.allow=true, got '$record_allow'"
fi

# Reset evidence after chain test
reset_evidence
