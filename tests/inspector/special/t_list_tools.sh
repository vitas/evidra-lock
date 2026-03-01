#!/usr/bin/env bash
# Special test: verify tool registration and schemas.
# Sourced by run_inspector_tests.sh — has access to helpers.

if [[ "$MODE" == "rest" ]]; then
    skip "list_tools" "not available in rest mode"
    return
fi

local raw_tools
raw_tools=$(call_list_tools) || {
    fail "list_tools" "inspector list-tools failed"
    return
}

# Check validate tool is registered
local has_validate
has_validate=$(echo "$raw_tools" | jq '[.tools[]? | select(.name == "validate")] | length')
if [[ "$has_validate" -gt 0 ]]; then
    pass "list_tools/validate_registered"
else
    fail "list_tools/validate_registered" "validate tool not found"
fi

# Check get_event tool is registered
local has_get_event
has_get_event=$(echo "$raw_tools" | jq '[.tools[]? | select(.name == "get_event")] | length')
if [[ "$has_get_event" -gt 0 ]]; then
    pass "list_tools/get_event_registered"
else
    fail "list_tools/get_event_registered" "get_event tool not found"
fi

# Check validate schema required fields
local validate_required
validate_required=$(echo "$raw_tools" | jq -r '[.tools[]? | select(.name == "validate")] | .[0].inputSchema.required // []')
for field in actor tool operation params context; do
    local has_field
    has_field=$(echo "$validate_required" | jq --arg f "$field" '[.[]? | select(. == $f)] | length')
    if [[ "$has_field" -gt 0 ]]; then
        pass "list_tools/validate_requires_$field"
    else
        fail "list_tools/validate_requires_$field" "validate schema missing required field '$field'"
    fi
done

# Check get_event schema required fields
local get_event_required
get_event_required=$(echo "$raw_tools" | jq -r '[.tools[]? | select(.name == "get_event")] | .[0].inputSchema.required // []')
local has_event_id
has_event_id=$(echo "$get_event_required" | jq '[.[]? | select(. == "event_id")] | length')
if [[ "$has_event_id" -gt 0 ]]; then
    pass "list_tools/get_event_requires_event_id"
else
    fail "list_tools/get_event_requires_event_id" "get_event schema missing required field 'event_id'"
fi
