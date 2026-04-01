#!/bin/bash
# Lint Loki AlertingRule templates using logcli fmt (LogQL syntax) on each rule expr.
# Extracts specs from the OpenShift Template (same pattern as lint-rules.sh + promtool).
#
# Usage: ./lint-loki-rules.sh <yaml-file> [logcli-path] [yq-path]

set -euo pipefail

TMP_DIR=$(mktemp -d)
trap 'rm -rf $TMP_DIR' EXIT

YAML_FILE="${1:?Usage: $0 <yaml-file> [logcli-path] [yq-path]}"
LOGCLI="${2:-logcli}"
YQ="${3:-yq}"

if ! command -v "$LOGCLI" &> /dev/null && [[ ! -x "$LOGCLI" ]]; then
    echo "Error: logcli not found at $LOGCLI" >&2
    exit 1
fi

if ! command -v "$YQ" &> /dev/null && [[ ! -x "$YQ" ]]; then
    echo "Error: yq not found at $YQ" >&2
    exit 1
fi

export LOKI_ADDR="${LOKI_ADDR:-http://127.0.0.1:65534}"

echo "Linting Loki rules from ${YAML_FILE}"

RULE_COUNT=$("$YQ" eval '.objects | length' "$YAML_FILE")
ERRORS=0

validate_exprs_in_spec_file() {
    local spec_file=$1
    local name=$2
    local n_groups
    n_groups=$("$YQ" eval '.groups | length' "$spec_file")
    if [[ "$n_groups" == "null" ]]; then
        n_groups=0
    fi

    local g r n_rules expr output
    for ((g = 0; g < n_groups; g++)); do
        n_rules=$("$YQ" eval ".groups[$g].rules | length" "$spec_file")
        if [[ "$n_rules" == "null" ]]; then
            n_rules=0
        fi
        for ((r = 0; r < n_rules; r++)); do
            expr=$("$YQ" eval ".groups[$g].rules[$r].expr // \"\"" "$spec_file")
            if [[ -z "$expr" || "$expr" == "null" ]]; then
                echo "  ERROR: ${name}: groups[${g}].rules[${r}] has no expr"
                return 1
            fi
            if ! output=$(echo "$expr" | "$LOGCLI" fmt --stdin -q 2>&1); then
                echo "  ERROR: ${name}: invalid LogQL (groups[${g}].rules[${r}]):"
                echo "$output" | sed 's/^/    /'
                return 1
            fi
        done
    done
    return 0
}

for i in $(seq 0 $((RULE_COUNT - 1))); do
    NAME=$("$YQ" eval ".objects[$i].metadata.name" "$YAML_FILE")
    KIND=$("$YQ" eval ".objects[$i].kind" "$YAML_FILE")

    if [[ "$KIND" != "AlertingRule" ]]; then
        continue
    fi

    RULE_FILE="$TMP_DIR/${NAME}.yaml"
    "$YQ" eval ".objects[$i].spec" "$YAML_FILE" > "$RULE_FILE"

    TENANT=$("$YQ" eval '.tenantID // ""' "$RULE_FILE")
    if [[ -z "$TENANT" || "$TENANT" == "null" ]]; then
        echo "  ERROR: ${NAME}: spec.tenantID missing or empty in template source"
        ERRORS=$((ERRORS + 1))
        continue
    fi

    echo -n "  Checking ${NAME}... "
    if validate_exprs_in_spec_file "$RULE_FILE" "$NAME"; then
        echo "OK"
    else
        echo "FAILED"
        ERRORS=$((ERRORS + 1))
    fi
done

if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "ERROR: $ERRORS AlertingRule(s) failed validation"
    exit 1
fi

echo ""
echo "All rules in ${YAML_FILE} passed validation"
