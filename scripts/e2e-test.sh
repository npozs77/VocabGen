#!/usr/bin/env bash
# E2E integration test for vocabgen CLI.
# Requires: built binary, AWS credentials (bedrock provider), network access.
# Uses a temp directory for DB — does not touch ~/.vocabgen/vocabgen.db.
#
# Usage: ./scripts/e2e-test.sh [-s SECTION] [MODEL_ID]
#   -s SECTION  Run only the given section number (1-10). Omit to run all.
#               Sections: 1=Version, 2=Errors, 3=Dry-Run, 4=Word Lookup,
#               5=Cache Hit, 6=Expression, 7=Batch, 8=Batch Limit,
#               9=Backup, 10=Context Bypass
#   Default model: us.anthropic.claude-sonnet-4-20250514-v1:0

set -euo pipefail

SECTION=""
while getopts "s:" opt; do
    case $opt in
        s) SECTION="$OPTARG" ;;
        *) echo "Usage: $0 [-s SECTION] [MODEL_ID]"; exit 1 ;;
    esac
done
shift $((OPTIND - 1))

MODEL_ID="${1:-us.anthropic.claude-sonnet-4-20250514-v1:0}"
BINARY="./vocabgen"
TMPDIR=$(mktemp -d)
DB_PATH="$TMPDIR/test.db"
PASS=0
FAIL=0
ERRORS=""

trap 'rm -rf "$TMPDIR"' EXIT

DB="--db-path $DB_PATH"

pass() { PASS=$((PASS + 1)); echo "  ✓ $1"; }
fail() { FAIL=$((FAIL + 1)); ERRORS="$ERRORS\n  ✗ $1: $2"; echo "  ✗ $1: $2"; }

assert_exit_zero() {
    local name="$1"; shift
    if "$@" > "$TMPDIR/out" 2> "$TMPDIR/err"; then
        pass "$name"
    else
        fail "$name" "exit $? — $(head -1 "$TMPDIR/err")"
    fi
}

assert_exit_nonzero() {
    local name="$1"; shift
    if "$@" > "$TMPDIR/out" 2> "$TMPDIR/err"; then
        fail "$name" "expected non-zero exit"
    else
        pass "$name"
    fi
}

stdout_contains() { grep -q "$1" "$TMPDIR/out"; }
stderr_contains() { grep -q "$1" "$TMPDIR/err"; }

# run_section returns 0 if the section should run.
run_section() { [ -z "$SECTION" ] || [ "$SECTION" = "$1" ]; }

echo "=== E2E Integration Tests ==="
echo "Model: $MODEL_ID"
echo "DB: $DB_PATH"
[ -n "$SECTION" ] && echo "Section: $SECTION"

if [ ! -f "$BINARY" ]; then
    echo "Building..."
    go build -o "$BINARY" ./cmd/vocabgen/
fi

# --- 1. Version & Help ---
if run_section 1; then
echo ""
echo "--- 1. Version & Help ---"
assert_exit_zero "version subcommand" $BINARY version
stdout_contains "vocabgen" && pass "version output" || fail "version output" "missing vocabgen"
assert_exit_zero "--version flag" $BINARY --version
assert_exit_zero "--help flag" $BINARY --help
stdout_contains "lookup" && pass "help lists subcommands" || fail "help lists subcommands" "missing lookup"
fi

# --- 2. Error Cases ---
if run_section 2; then
echo ""
echo "--- 2. Error Cases ---"
assert_exit_nonzero "lookup no source-lang" $BINARY lookup "test" --source-language "" $DB
assert_exit_nonzero "batch missing input-file" $BINARY batch --mode words -l nl $DB
assert_exit_nonzero "batch file not found" $BINARY batch --input-file /nonexistent.csv --mode words -l nl $DB
assert_exit_nonzero "invalid provider" $BINARY lookup "test" -l nl --provider fakeprovider $DB
assert_exit_nonzero "invalid batch mode" $BINARY batch --input-file /dev/null --mode invalid -l nl $DB
fi

# --- 3. Dry-Run ---
if run_section 3; then
echo ""
echo "--- 3. Dry-Run ---"
cat > "$TMPDIR/words.csv" <<EOF
fiets
huis
boek
EOF
assert_exit_zero "batch dry-run" $BINARY batch \
    --input-file "$TMPDIR/words.csv" --mode words -l nl \
    --model-id "$MODEL_ID" --dry-run $DB
stderr_contains "Processed" && pass "dry-run summary" || fail "dry-run summary" "missing Processed"
fi

# --- 4. Word Lookup (live LLM) ---
if run_section 4; then
echo ""
echo "--- 4. Word Lookup ---"
assert_exit_zero "lookup fiets" $BINARY lookup "fiets" -l nl --model-id "$MODEL_ID" $DB
for field in '"word"' '"definition"' '"english"' '"target_translation"' '"english_definition"'; do
    stdout_contains "$field" && pass "lookup has $field" || fail "lookup has $field" "missing"
done
fi

# --- 5. Cache Hit ---
if run_section 5; then
echo ""
echo "--- 5. Cache Hit ---"
assert_exit_zero "lookup fiets cached" $BINARY lookup "fiets" -l nl --model-id "$MODEL_ID" $DB
stderr_contains "cache hit" && pass "cache hit logged" || fail "cache hit logged" "missing"
fi

# --- 6. Expression Lookup ---
if run_section 6; then
echo ""
echo "--- 6. Expression Lookup ---"
assert_exit_zero "lookup expression" $BINARY lookup "op de hoogte zijn" \
    -l nl --type expression --model-id "$MODEL_ID" $DB
stdout_contains '"expression"' && pass "expression field" || fail "expression field" "missing"
fi

# --- 7. Batch Processing ---
if run_section 7; then
echo ""
echo "--- 7. Batch Processing ---"
cat > "$TMPDIR/batch.csv" <<EOF
fiets
straat
EOF
assert_exit_zero "batch process" $BINARY batch \
    --input-file "$TMPDIR/batch.csv" --mode words -l nl \
    --model-id "$MODEL_ID" --on-conflict skip $DB
stderr_contains "Batch Summary" && pass "batch summary" || fail "batch summary" "missing"
stderr_contains "Cached" && pass "batch cached count" || fail "batch cached count" "missing"
fi

# --- 8. Batch with Limit ---
if run_section 8; then
echo ""
echo "--- 8. Batch with Limit ---"
cat > "$TMPDIR/limit.csv" <<EOF
boom
kat
hond
vis
EOF
assert_exit_zero "batch limit=1" $BINARY batch \
    --input-file "$TMPDIR/limit.csv" --mode words -l nl \
    --model-id "$MODEL_ID" --limit 1 --on-conflict skip $DB
stderr_contains "Processed" && pass "limit summary" || fail "limit summary" "missing"
fi

# --- 9. Backup & Restore ---
if run_section 9; then
echo ""
echo "--- 9. Backup & Restore ---"
assert_exit_zero "backup" $BINARY backup $DB
BACKUP_FILE=$(cat "$TMPDIR/out" | tr -d '[:space:]')
[ -f "$BACKUP_FILE" ] && pass "backup file exists" || fail "backup file exists" "$BACKUP_FILE"
assert_exit_zero "restore" $BINARY restore "$BACKUP_FILE" $DB
stdout_contains "restored" && pass "restore message" || fail "restore message" "missing"
fi

# --- 10. Context Bypass ---
if run_section 10; then
echo ""
echo "--- 10. Context Bypass ---"
assert_exit_zero "lookup with context" $BINARY lookup "fiets" -l nl \
    --context "De elektrische fiets is populair." \
    --model-id "$MODEL_ID" --on-conflict add $DB
stdout_contains '"word"' && pass "context result" || fail "context result" "missing"
fi

# --- Summary ---
echo ""
echo "============================================"
echo "  E2E: $PASS passed, $FAIL failed"
echo "============================================"
if [ $FAIL -gt 0 ]; then
    echo -e "\nFailures:$ERRORS"
    exit 1
fi
echo "All E2E tests passed."
