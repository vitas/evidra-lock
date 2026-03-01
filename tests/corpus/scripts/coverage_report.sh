#!/usr/bin/env bash
# Cross-reference corpus rule coverage with policy bundle rule catalog.
# Outputs human-readable table showing which rules have test cases.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORPUS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$CORPUS_DIR/../.." && pwd)"
RULE_HINTS="$REPO_ROOT/policy/bundles/ops-v0.1/evidra/data/rule_hints/data.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

if [ ! -f "$RULE_HINTS" ]; then
    echo "Error: rule_hints not found: $RULE_HINTS"
    exit 1
fi

# Get all rule IDs from catalog (sorted)
catalog_rules=$(jq -r 'keys[]' "$RULE_HINTS" | sort)
catalog_count=$(echo "$catalog_rules" | wc -l | tr -d ' ')

# Get all rule IDs covered by corpus files
corpus_rules=$(
    for f in "$CORPUS_DIR"/*.json; do
        base=$(basename "$f")
        if [ "$base" = "sources.json" ] || [ "$base" = "manifest.json" ]; then
            continue
        fi
        jq -r '._meta.rules[]?' "$f" 2>/dev/null
    done | sort -u
)
covered_count=$(echo "$corpus_rules" | grep -c . || true)

# Build coverage table
echo ""
echo -e "${BLUE}Evidra Corpus Coverage Report${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
printf "%-35s %-8s %-8s %s\n" "Rule ID" "Deny" "Allow" "Status"
echo "───────────────────────────────────────────────────────────────"

missing_rules=""
for rule in $catalog_rules; do
    # Count deny/warn cases for this rule
    deny_count=0
    allow_count=0
    for f in "$CORPUS_DIR"/*.json; do
        base=$(basename "$f")
        if [ "$base" = "sources.json" ] || [ "$base" = "manifest.json" ]; then
            continue
        fi
        has_rule=$(jq -r --arg r "$rule" '._meta.rules[]? | select(. == $r)' "$f" 2>/dev/null || true)
        if [ -n "$has_rule" ]; then
            expect_allow=$(jq -r '.expect.allow' "$f" 2>/dev/null || true)
            if [ "$expect_allow" = "false" ]; then
                deny_count=$((deny_count + 1))
            else
                allow_count=$((allow_count + 1))
            fi
        fi
    done

    if [ "$deny_count" -gt 0 ] || [ "$allow_count" -gt 0 ]; then
        status="${GREEN}covered${NC}"
    else
        status="${RED}MISSING${NC}"
        missing_rules="$missing_rules $rule"
    fi

    printf "%-35s %-8s %-8s " "$rule" "$deny_count" "$allow_count"
    echo -e "$status"
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "Catalog rules:  ${BLUE}$catalog_count${NC}"
echo -e "Covered rules:  ${GREEN}$covered_count${NC}"

if [ -n "$missing_rules" ]; then
    missing_count=$(echo "$missing_rules" | wc -w | tr -d ' ')
    echo -e "Missing rules:  ${RED}$missing_count${NC} -$missing_rules"
    echo ""
    exit 1
else
    echo -e "Missing rules:  ${GREEN}0${NC}"
    echo ""
    echo -e "${GREEN}Full coverage: all $catalog_count catalog rules have corpus cases.${NC}"
fi
