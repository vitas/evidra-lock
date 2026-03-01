#!/usr/bin/env bash
# Validate corpus integrity: JSON syntax, required fields, unique IDs,
# params key allowlist, rule ID existence, manifest sync.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORPUS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$CORPUS_DIR/../.." && pwd)"
RULE_HINTS="$REPO_ROOT/policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json"
MANIFEST="$CORPUS_DIR/manifest.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

ERRORS=0

fail() { echo -e "${RED}[FAIL]${NC} $*"; ERRORS=$((ERRORS + 1)); }
pass() { echo -e "${GREEN}[OK]${NC} $*"; }

# Collect all corpus case files (exclude sources.json, manifest.json, README)
CASE_FILES=()
for f in "$CORPUS_DIR"/*.json; do
    base=$(basename "$f")
    case "$base" in
        sources.json|manifest.json) continue ;;
        *) CASE_FILES+=("$f") ;;
    esac
done

echo "Validating ${#CASE_FILES[@]} corpus files..."
echo ""

# 1. Valid JSON
echo "=== JSON syntax ==="
for f in "${CASE_FILES[@]}"; do
    if ! jq empty "$f" 2>/dev/null; then
        fail "Invalid JSON: $(basename "$f")"
    fi
done
pass "All files are valid JSON"
echo ""

# 2. Required fields
echo "=== Required fields ==="
for f in "${CASE_FILES[@]}"; do
    base=$(basename "$f")
    id=$(jq -r '._meta.id // empty' "$f")
    rules=$(jq -r '._meta.rules // empty' "$f")

    if [ -z "$id" ]; then
        fail "$base: missing _meta.id"
    fi

    # Files with input must have actor, tool, operation, params
    has_input=$(jq 'has("input")' "$f")
    if [ "$has_input" = "true" ]; then
        for field in actor tool operation params; do
            val=$(jq -r ".input.$field // empty" "$f")
            if [ -z "$val" ] || [ "$val" = "null" ]; then
                fail "$base: missing input.$field"
            fi
        done

        # Check actor subfields
        for afield in type id origin; do
            val=$(jq -r ".input.actor.$afield // empty" "$f")
            if [ -z "$val" ]; then
                fail "$base: missing input.actor.$afield"
            fi
        done
    fi
done
pass "Required fields present"
echo ""

# 3. Unique IDs
echo "=== Unique IDs ==="
ids=$(for f in "${CASE_FILES[@]}"; do jq -r '._meta.id' "$f"; done)
dupes=$(echo "$ids" | sort | uniq -d)
if [ -n "$dupes" ]; then
    fail "Duplicate IDs: $dupes"
else
    pass "All IDs unique ($(echo "$ids" | wc -l | tr -d ' ') cases)"
fi
echo ""

# 4. Params key allowlist
echo "=== Params key allowlist ==="
ALLOWED_KEYS="target payload risk_tags scenario_id action"
for f in "${CASE_FILES[@]}"; do
    has_input=$(jq 'has("input")' "$f")
    [ "$has_input" = "true" ] || continue
    base=$(basename "$f")
    keys=$(jq -r '.input.params | keys[]' "$f" 2>/dev/null || true)
    for k in $keys; do
        if ! echo "$ALLOWED_KEYS" | grep -qw "$k"; then
            fail "$base: unknown params key: $k"
        fi
    done
done
pass "Params keys valid"
echo ""

# 5. Rule IDs exist in policy bundle
echo "=== Rule ID existence ==="
if [ -f "$RULE_HINTS" ]; then
    known_rules=$(jq -r 'keys[]' "$RULE_HINTS")
    for f in "${CASE_FILES[@]}"; do
        base=$(basename "$f")
        file_rules=$(jq -r '._meta.rules[]?' "$f" 2>/dev/null || true)
        for r in $file_rules; do
            if ! echo "$known_rules" | grep -qx "$r"; then
                fail "$base: unknown rule ID: $r"
            fi
        done
    done
    pass "All rule IDs exist in rule_hints"
else
    fail "rule_hints/data.json not found at $RULE_HINTS"
fi
echo ""

# 6. Manifest sync
echo "=== Manifest sync ==="
if [ -f "$MANIFEST" ]; then
    manifest_ids=$(jq -r '.cases[].id' "$MANIFEST" | sort)
    file_ids=$(for f in "${CASE_FILES[@]}"; do jq -r '._meta.id' "$f"; done | sort)

    missing_from_manifest=$(comm -23 <(echo "$file_ids") <(echo "$manifest_ids"))
    missing_from_files=$(comm -13 <(echo "$file_ids") <(echo "$manifest_ids"))

    if [ -n "$missing_from_manifest" ]; then
        fail "In files but not manifest: $missing_from_manifest"
    fi
    if [ -n "$missing_from_files" ]; then
        fail "In manifest but not files: $missing_from_files"
    fi
    if [ -z "$missing_from_manifest" ] && [ -z "$missing_from_files" ]; then
        pass "Manifest synced with $(echo "$manifest_ids" | wc -l | tr -d ' ') cases"
    fi
else
    fail "manifest.json not found"
fi
echo ""

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$ERRORS" -gt 0 ]; then
    echo -e "${RED}FAILED: $ERRORS errors${NC}"
    exit 1
else
    echo -e "${GREEN}PASSED: all checks passed${NC}"
fi
