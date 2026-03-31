#!/usr/bin/env bash
# Install local git hooks to prevent direct commits to main.
set -euo pipefail

HOOK=".git/hooks/pre-commit"

cat > "$HOOK" << 'HOOK_CONTENT'
#!/usr/bin/env bash
branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$branch" = "main" ]; then
    echo "ERROR: Direct commits to main are not allowed."
    echo "Create a feature branch: git checkout -b feature/my-change"
    exit 1
fi
HOOK_CONTENT

chmod +x "$HOOK"
echo "Pre-commit hook installed: direct commits to main blocked."
