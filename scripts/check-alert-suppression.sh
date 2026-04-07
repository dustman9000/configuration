#!/bin/bash
# Check that all HCP alert expressions include the standard suppression clause.
#
# Every per-HCP alert should include:
#   unless on (_id) sre:hcp:alerts_suppressed
#
# Alerts are excluded if they:
#   - Are infrastructure/synthetics alerts (no _id label)
#   - Are recording rules (not alerts)
#
# Usage:
#   ./scripts/check-alert-suppression.sh resources/tenant-rules/hcp/*.yaml

set -euo pipefail

SUPPRESSION_RULE="sre:hcp:alerts_suppressed"

# Infrastructure alerts that don't have _id and should be excluded
EXCLUDE_PATTERN="Synthetics|RMO|NoMetricReceived"

ERRORS=0
CHECKED=0

for file in "$@"; do
    # Extract alert names and their expressions
    # Use awk to find alert blocks and check their expr
    awk -v suppress="$SUPPRESSION_RULE" -v exclude="$EXCLUDE_PATTERN" -v file="$file" '
    /^[[:space:]]*- alert:/ || /^[[:space:]]*alert:/ {
        alert_name = $NF
        in_expr = 0
        expr_content = ""
        next
    }
    /^[[:space:]]*expr:/ {
        in_expr = 1
        # Check if expr is on the same line
        sub(/^[[:space:]]*expr:[[:space:]]*/, "")
        if ($0 != "" && $0 != "|") {
            expr_content = $0
        }
        next
    }
    in_expr && /^[[:space:]]*[a-z]/ && !/^[[:space:]]*(for:|labels:|annotations:|severity:|alert:|record:|description:|summary:|runbook)/ {
        expr_content = expr_content " " $0
        next
    }
    in_expr && (/^[[:space:]]*(for:|labels:|annotations:|severity:|alert:|record:)/ || /^[[:space:]]*$/) {
        in_expr = 0
        if (alert_name != "" && expr_content != "") {
            # Check if alert should be excluded
            if (alert_name ~ exclude) {
                next
            }
            # Check if suppression is present
            if (expr_content !~ suppress) {
                printf "  MISSING: %s:%s\n", file, alert_name
                errors++
            } else {
                checked++
            }
        }
        alert_name = ""
        expr_content = ""
    }
    END {
        # Handle last alert in file
        if (alert_name != "" && expr_content != "") {
            if (alert_name !~ exclude) {
                if (expr_content !~ suppress) {
                    printf "  MISSING: %s:%s\n", file, alert_name
                    errors++
                } else {
                    checked++
                }
            }
        }
    }
    ' "$file"
done

# Count results
MISSING=$(for file in "$@"; do
    awk -v suppress="$SUPPRESSION_RULE" -v exclude="$EXCLUDE_PATTERN" '
    /^[[:space:]]*- alert:/ || /^[[:space:]]*alert:/ { alert_name = $NF; in_expr = 0; expr = "" }
    /^[[:space:]]*expr:/ { in_expr = 1; sub(/^[[:space:]]*expr:[[:space:]]*/, ""); if ($0 != "" && $0 != "|") expr = $0 }
    in_expr && /^[[:space:]]*[a-z(]/ && !/^[[:space:]]*(for:|labels:|annotations:|severity:|alert:|record:)/ { expr = expr " " $0 }
    in_expr && (/^[[:space:]]*(for:|labels:|annotations:|severity:|alert:|record:)/ || /^[[:space:]]*$/) {
        in_expr = 0
        if (alert_name != "" && expr != "" && alert_name !~ exclude && expr !~ suppress) print alert_name
        alert_name = ""; expr = ""
    }
    END { if (alert_name != "" && expr != "" && alert_name !~ exclude && expr !~ suppress) print alert_name }
    ' "$file"
done | wc -l | tr -d ' ')

if [ "$MISSING" -gt 0 ]; then
    echo ""
    echo "ERROR: $MISSING alert(s) missing 'unless on (_id) $SUPPRESSION_RULE'"
    echo "Add the suppression clause to each alert's expr, or add the alert to the exclude list if it's an infrastructure alert."
    exit 1
else
    echo "All per-HCP alerts include suppression clause."
fi
