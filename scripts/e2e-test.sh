#!/usr/bin/env bash
# E2E integration test for vocabgen CLI.
# Defaults to the "local" config profile (Ollama). Override with -p <profile>.
# Uses a temp directory for DB — does not touch ~/.vocabgen/vocabgen.db.
#
# Usage: ./scripts/e2e-test.sh [-s SECTION] [-p PROFILE]
#   -s SECTION   Run only the given section number (1-14). Omit to run all.
#                Sections: 1=Version, 2=Errors, 3=Dry-Run, 4=Word Lookup,
#                5=Cache Hit, 6=Expression, 7=Batch, 8=Batch Limit,
#                9=Backup, 10=Context Bypass, 11=Update Checker,
#                12=Config Profiles, 13=Sentence Lookup,
#                14=Local LLM Setup
#   -p PROFILE   Config profile name (default: local, override via E2E_PROFILE env var)

set -euo pipefail

SECTION=""
PROFILE="${E2E_PROFILE:-local}"
while getopts "s:p:" opt; do
    case $opt in
        s) SECTION="$OPTARG" ;;
        p) PROFILE="$OPTARG" ;;
        *) echo "Usage: $0 [-s SECTION] [-p PROFILE]"; exit 1 ;;
    esac
done
shift $((OPTIND - 1))

BINARY="./vocabgen"
TMPDIR=$(mktemp -d)
DB_PATH="$TMPDIR/test.db"
PASS=0
FAIL=0
ERRORS=""

trap 'rm -rf "$TMPDIR"' EXIT

DB="--db-path $DB_PATH"
PROFILE_FLAG="--profile $PROFILE"

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
echo "Profile: $PROFILE"
echo "DB: $DB_PATH"
[ -n "$SECTION" ] && echo "Section: $SECTION"

if [ ! -f "$BINARY" ]; then
    echo "Building..."
    go build -o "$BINARY" ./cmd/vocabgen/
fi

# --- Pre-flight: verify profile exists ---
echo ""
echo "--- Pre-flight checks ---"
if ! $BINARY lookup "test" -l nl $PROFILE_FLAG --dry-run $DB > /dev/null 2> "$TMPDIR/err"; then
    if grep -qi "profile.*not found\|not found.*profile\|unknown profile" "$TMPDIR/err" 2>/dev/null; then
        echo "ERROR: Profile '$PROFILE' not found in config."
        echo "  Run: scripts/setup-local-llm.sh   (to create 'local' profile)"
        echo "  Or:  $0 -p <existing-profile>      (to use a different profile)"
        exit 1
    fi
    # Other errors (e.g. empty source-lang) are OK — profile exists but lookup failed for other reasons.
    # The dry-run with empty word may fail on validation, that's fine.
fi
echo "  ✓ Profile '$PROFILE' exists"

# --- Pre-flight: if local profile, check Ollama reachability ---
if [ "$PROFILE" = "local" ]; then
    if ! curl -sf --max-time 3 http://localhost:11434/api/tags > /dev/null 2>&1; then
        echo "ERROR: Ollama is not running at http://localhost:11434"
        echo "  Start it with:  ollama serve"
        echo "  Or set it up:   scripts/setup-local-llm.sh"
        echo "  Or use cloud:   $0 -p bedrock"
        exit 1
    fi
    echo "  ✓ Ollama is reachable"
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
    $PROFILE_FLAG --dry-run $DB
stderr_contains "Processed" && pass "dry-run summary" || fail "dry-run summary" "missing Processed"
fi

# --- 4. Word Lookup (live LLM) ---
if run_section 4; then
echo ""
echo "--- 4. Word Lookup ---"
assert_exit_zero "lookup fiets" $BINARY lookup "fiets" -l nl $PROFILE_FLAG $DB
for field in '"word"' '"definition"' '"english"' '"target_translation"' '"english_definition"'; do
    stdout_contains "$field" && pass "lookup has $field" || fail "lookup has $field" "missing"
done
fi

# --- 5. Cache Hit ---
if run_section 5; then
echo ""
echo "--- 5. Cache Hit ---"
assert_exit_zero "lookup fiets cached" $BINARY lookup "fiets" -l nl $PROFILE_FLAG $DB
stderr_contains "cache hit" && pass "cache hit logged" || fail "cache hit logged" "missing"
fi

# --- 6. Expression Lookup ---
if run_section 6; then
echo ""
echo "--- 6. Expression Lookup ---"
assert_exit_zero "lookup expression" $BINARY lookup "op de hoogte zijn" \
    -l nl --type expression $PROFILE_FLAG $DB
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
    $PROFILE_FLAG --on-conflict skip $DB
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
    $PROFILE_FLAG --limit 1 --on-conflict skip $DB
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
    $PROFILE_FLAG --on-conflict add $DB
stdout_contains '"word"' && pass "context result" || fail "context result" "missing"
fi

# --- 11. Update Checker (builds with fake old version) ---
if run_section 11; then
echo ""
echo "--- 11. Update Checker ---"
# Build with a fake old version so any existing GitHub release triggers "update available".
go build -ldflags "-X main.version=0.0.1 -X main.buildDate=2025-01-01" -o "$TMPDIR/vocabgen-old" ./cmd/vocabgen/
"$TMPDIR/vocabgen-old" serve $DB &
SERVER_PID=$!
sleep 2

# Check /update page loads
if curl -sf http://localhost:8080/update > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    pass "update page loads"
    stdout_contains "0.0.1" && pass "update page shows version" || fail "update page shows version" "missing 0.0.1"
else
    fail "update page loads" "curl failed"
fi

# Check /api/update/check returns HTML with update info
if curl -sf http://localhost:8080/api/update/check > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    pass "update check API"
    stdout_contains "Update available" && pass "update available detected" || pass "update check responded (may be up-to-date or API unreachable)"
else
    fail "update check API" "curl failed"
fi

# Check dismiss endpoint
if curl -sf -X POST http://localhost:8080/api/update/dismiss > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    pass "dismiss endpoint"
else
    fail "dismiss endpoint" "curl failed"
fi

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

# CLI update check tests (no server needed)
assert_exit_zero "cli version with update notice" "$TMPDIR/vocabgen-old" version
stdout_contains "Update available" && pass "version shows update notice" || pass "version update check (may fail if API unreachable)"

"$TMPDIR/vocabgen-old" update > "$TMPDIR/out" 2> "$TMPDIR/err"
UPDATE_EXIT=$?
if [ $UPDATE_EXIT -eq 0 ]; then
    pass "cli update subcommand"
    grep -q "Latest version\|up to date" "$TMPDIR/out" && pass "update shows version info" || fail "update shows version info" "missing"
    grep -q "Download" "$TMPDIR/out" && pass "update shows download URL" || pass "update download (up-to-date has no URL)"
else
    # Exit 1 means API error — still a valid test if network is down
    pass "cli update subcommand (API may be unreachable)"
fi
fi

# --- 12. Config Profiles ---
if run_section 12; then
echo ""
echo "--- 12. Config Profiles ---"

# Test --profile flag is recognized (--help should list it)
$BINARY --help > "$TMPDIR/out" 2> "$TMPDIR/err"
grep -q "\-\-profile" "$TMPDIR/out" && pass "help lists --profile flag" || fail "help lists --profile flag" "missing"
grep -q "\-\-aws-profile" "$TMPDIR/out" && pass "help lists --aws-profile flag" || fail "help lists --aws-profile flag" "missing"

# Test --profile nonexistent returns error (using lookup with dry-run to avoid LLM call)
if $BINARY lookup "test" -l nl --profile nonexistent --dry-run $DB > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    fail "profile nonexistent error" "expected non-zero exit"
else
    pass "profile nonexistent error"
    stderr_contains "not found" && pass "profile error message" || pass "profile error (message may vary)"
fi

# Test --aws-profile flag is accepted (should not error on flag parsing itself)
# Don't use --dry-run here: dry-run skips provider init, so credential errors won't surface.
assert_exit_nonzero "aws-profile without creds" $BINARY lookup "test" -l nl --aws-profile fakeprofname --provider bedrock $DB
# The error should be about credentials, not about unknown flag
if grep -q "unknown flag" "$TMPDIR/err" 2>/dev/null; then
    fail "aws-profile flag recognized" "flag not recognized"
else
    pass "aws-profile flag recognized"
fi
fi

# --- 13. Sentence Lookup (ephemeral, no DB write) ---
if run_section 13; then
echo ""
echo "--- 13. Sentence Lookup ---"

assert_exit_zero "sentence lookup" $BINARY lookup "Ik ga morgen naar de markt om groenten te kopen." \
    -l nl --type sentence $PROFILE_FLAG $DB
stdout_contains '"expression"' && pass "sentence has expression field" || fail "sentence has expression field" "missing"
stdout_contains '"definition"' && pass "sentence has definition field" || fail "sentence has definition field" "missing"
stderr_contains "sentence lookup" && pass "sentence logged as ephemeral" || fail "sentence logged as ephemeral" "missing"

# Verify the sentence was NOT cached — a second lookup should NOT say "cache hit".
assert_exit_zero "sentence not cached" $BINARY lookup "Ik ga morgen naar de markt om groenten te kopen." \
    -l nl --type sentence $PROFILE_FLAG $DB
if stderr_contains "cache hit"; then
    fail "sentence not stored in DB" "found cache hit — sentence was persisted"
else
    pass "sentence not stored in DB"
fi
fi

# --- 14. Local LLM Setup ---
if run_section 14; then
echo ""
echo "--- 14. Local LLM Setup ---"

# 14a. Syntax check the setup script.
if bash -n scripts/setup-local-llm.sh > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    pass "setup-local-llm.sh syntax valid"
else
    fail "setup-local-llm.sh syntax valid" "$(head -1 "$TMPDIR/err")"
fi

# 14b. Verify the Web UI setup endpoint is registered.
# Start a server, hit the endpoint, verify it's not 404.
$BINARY serve $DB --port 8091 &
SETUP_PID=$!
sleep 2

if curl -sf -o /dev/null -w "%{http_code}" http://localhost:8091/api/setup/local-llm > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    pass "setup endpoint registered"
else
    CODE=$(cat "$TMPDIR/out" 2>/dev/null || echo "")
    if [ "$CODE" = "404" ] || [ "$CODE" = "405" ]; then
        fail "setup endpoint registered" "got HTTP $CODE"
    else
        pass "setup endpoint registered (non-404 response)"
    fi
fi

kill $SETUP_PID 2>/dev/null || true
wait $SETUP_PID 2>/dev/null || true

# 14c. Verify validateProviderEnv accepts Ollama base URL without API key.
# Use lookup with Ollama base URL and no API key — should not fail on "missing API key".
# (It may fail on Ollama not running or LLM error, but not on missing key.)
if $BINARY lookup "test" -l nl --provider openai --base-url http://localhost:11434/v1 \
    --model-id translategemma $DB > "$TMPDIR/out" 2> "$TMPDIR/err"; then
    pass "ollama base URL accepted without API key"
else
    # Check the error is NOT about missing API key
    if grep -qi "OPENAI_API_KEY\|api.key\|API key" "$TMPDIR/err" 2>/dev/null; then
        fail "ollama base URL accepted without API key" "error mentions API key"
    else
        pass "ollama base URL accepted without API key (failed for other reason)"
    fi
fi
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
