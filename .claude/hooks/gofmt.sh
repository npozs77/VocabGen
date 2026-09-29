#!/usr/bin/env bash
# PostToolUse hook: format Go files Claude edits (editor format-on-save doesn't
# apply to them). Prefers goimports, falls back to gofmt, no-op if neither exists.
set -uo pipefail

file=$(jq -r '.tool_input.file_path // empty')
[[ "$file" == *.go && -f "$file" ]] || exit 0

if command -v goimports >/dev/null 2>&1; then
    goimports -w "$file"
elif command -v gofmt >/dev/null 2>&1; then
    gofmt -w "$file"
fi
exit 0
