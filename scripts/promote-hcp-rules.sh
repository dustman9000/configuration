#!/bin/bash
# Promote HCP tenant rules to production RHOBS cells.
#
# Usage:
#   ./scripts/promote-hcp-rules.sh [SHA]
#
# If SHA is omitted, uses the current HEAD of the main branch.
# This script updates the app-interface saas file to pin production
# targets to the specified SHA. Integration and stage targets remain
# on ref: main for continuous validation.
#
# Prerequisites:
#   - app-interface repo checked out at $APP_INTERFACE_PATH (default: ~/src/app-interface)
#   - glab CLI configured for gitlab.cee.redhat.com
#
# After running this script:
#   1. Review the diff: cd $APP_INTERFACE_PATH && git diff
#   2. Commit: git add ... && git commit
#   3. Create MR: glab mr create --repo service/app-interface ...

set -euo pipefail

APP_INTERFACE_PATH="${APP_INTERFACE_PATH:-$HOME/src/app-interface}"
SAAS_FILE="$APP_INTERFACE_PATH/data/services/rhobs/rhobs/cicd/saas-hcp-rules.yaml"

if [ ! -f "$SAAS_FILE" ]; then
    echo "Error: saas file not found at $SAAS_FILE"
    echo "Set APP_INTERFACE_PATH to your app-interface checkout"
    exit 1
fi

# Determine SHA to promote
if [ -n "${1:-}" ]; then
    SHA="$1"
    echo "Promoting to specified SHA: $SHA"
else
    SHA=$(git rev-parse HEAD)
    echo "Promoting to current HEAD: $SHA"
fi

# Verify SHA exists
if ! git cat-file -t "$SHA" &>/dev/null; then
    echo "Error: SHA $SHA not found in rhobs/configuration repo"
    exit 1
fi

echo ""
echo "Commits being promoted (since current production ref):"
CURRENT_REF=$(grep -A2 'rhobs-production.yml' "$SAAS_FILE" | grep 'ref:' | head -1 | awk '{print $2}')
if [ "$CURRENT_REF" = "main" ]; then
    echo "  Production currently on ref: main (not pinned)"
else
    echo "  Current production ref: ${CURRENT_REF:0:12}"
    git log --oneline "$CURRENT_REF..$SHA" 2>/dev/null || echo "  (cannot show log, current ref may not be in local repo)"
fi

echo ""

# Pin production targets
python3 - "$SAAS_FILE" "$SHA" << 'PYEOF'
import sys

saas_file = sys.argv[1]
sha = sys.argv[2]

with open(saas_file) as f:
    lines = f.readlines()

new_lines = []
pin_next_ref = False
count = 0
for line in lines:
    if 'rhobs-production.yml' in line:
        pin_next_ref = True
    if pin_next_ref and 'ref:' in line and line.strip().startswith('ref:'):
        old_ref = line.strip().split('ref:')[1].strip()
        line = line.replace(f'ref: {old_ref}', f'ref: {sha}')
        pin_next_ref = False
        count += 1
    new_lines.append(line)

with open(saas_file, 'w') as f:
    f.writelines(new_lines)

print(f"Pinned {count} production targets to {sha[:12]}")
PYEOF

echo ""
echo "Next steps:"
echo "  cd $APP_INTERFACE_PATH"
echo "  git diff data/services/rhobs/rhobs/cicd/saas-hcp-rules.yaml"
echo "  git checkout -b drow/promote-hcp-rules-${SHA:0:8} upstream/master"
echo "  git add data/services/rhobs/rhobs/cicd/saas-hcp-rules.yaml"
echo "  git commit -m 'Promote HCP tenant rules to ${SHA:0:12}'"
echo "  git push origin drow/promote-hcp-rules-${SHA:0:8}"
echo "  glab mr create --repo service/app-interface --title 'Promote HCP tenant rules to ${SHA:0:12}' --label self-serviceable --label rhobs"
