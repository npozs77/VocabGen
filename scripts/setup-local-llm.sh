#!/usr/bin/env bash
# One-click local LLM setup via Ollama.
# Detects OS, installs Ollama if needed, pulls model, writes config.
#
# Usage: ./scripts/setup-local-llm.sh
#
# After running, vocabgen will use the local Ollama instance by default:
#   vocabgen lookup "fiets" -l nl --profile local

set -euo pipefail

MODEL="translategemma"
OLLAMA_URL="http://localhost:11434"
CONFIG_DIR="$HOME/.vocabgen"
CONFIG_FILE="$CONFIG_DIR/config.yaml"
MAX_WAIT=30

# --- Helpers ---

info()  { echo "==> $*"; }
warn()  { echo "WARNING: $*" >&2; }
die()   { echo "ERROR: $*" >&2; exit 1; }

# --- 1. Detect OS ---

info "Detecting OS..."
OS="$(uname -s)"
case "$OS" in
    Darwin) info "macOS detected" ;;
    Linux)  info "Linux detected" ;;
    *)      die "Unsupported OS: $OS. This script supports macOS and Linux only." ;;
esac

# --- 2. Check / Install Ollama ---

if command -v ollama > /dev/null 2>&1; then
    info "Ollama is already installed: $(command -v ollama)"
else
    info "Ollama not found. Installing..."
    case "$OS" in
        Darwin)
            if command -v brew > /dev/null 2>&1; then
                info "Installing via Homebrew..."
                brew install ollama
            else
                info "Homebrew not found. Installing via curl..."
                curl -fsSL https://ollama.com/install.sh | sh
            fi
            ;;
        Linux)
            info "Installing via curl..."
            curl -fsSL https://ollama.com/install.sh | sh
            ;;
    esac

    command -v ollama > /dev/null 2>&1 || die "Ollama installation failed. Install manually: https://ollama.com/download"
    info "Ollama installed successfully."
fi

# --- 3. Check / Start Ollama Server ---

ollama_ready() {
    curl -sf "$OLLAMA_URL/api/tags" > /dev/null 2>&1
}

if ollama_ready; then
    info "Ollama server is already running."
else
    info "Starting Ollama server..."
    ollama serve > /dev/null 2>&1 &
    OLLAMA_PID=$!

    elapsed=0
    while [ $elapsed -lt $MAX_WAIT ]; do
        if ollama_ready; then
            break
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done

    if ! ollama_ready; then
        die "Ollama server did not start within ${MAX_WAIT}s. Check 'ollama serve' manually."
    fi
    info "Ollama server is ready (waited ${elapsed}s)."
fi

# --- 4. Pull Model ---

info "Pulling model '$MODEL' (this may take a few minutes on first run)..."
ollama pull "$MODEL" || die "Failed to pull model '$MODEL'."
info "Model '$MODEL' is available."

# --- 5. Verify Model Responds ---

info "Verifying model responds..."
RESPONSE=$(curl -sf "$OLLAMA_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Say hello in one word.\"}],\"max_tokens\":10}" \
    2>/dev/null) || die "Model verification failed. Ollama may not be serving '$MODEL' correctly."

if echo "$RESPONSE" | grep -q '"choices"'; then
    info "Model verification passed."
else
    die "Unexpected response from model. Expected OpenAI-compatible format."
fi

# --- 6. Write Config ---

info "Writing config to $CONFIG_FILE..."
mkdir -p "$CONFIG_DIR"

if [ -f "$CONFIG_FILE" ]; then
    # Existing config — check if it already has profiles
    if grep -q "^profiles:" "$CONFIG_FILE" 2>/dev/null; then
        info "Existing multi-profile config found. Adding/updating 'local' profile..."

        # Use a temp file for safe rewrite
        TMPFILE=$(mktemp)
        trap 'rm -f "$TMPFILE"' EXIT

        # Strategy: read existing YAML, inject/replace the local profile block,
        # and set default_profile to local.
        # We use awk for reliable YAML manipulation of the known structure.
        awk '
        BEGIN { in_local = 0; wrote_local = 0; skip_until_next = 0 }

        # Set default_profile to local
        /^default_profile:/ {
            print "default_profile: local"
            next
        }

        # Detect start of local: profile block
        /^  local:/ {
            in_local = 1
            skip_until_next = 1
            if (!wrote_local) {
                print "  local:"
                print "    provider: openai"
                print "    base_url: http://localhost:11434/v1"
                print "    model_id: translategemma"
                wrote_local = 1
            }
            next
        }

        # Skip lines belonging to the old local profile
        skip_until_next && /^  [a-zA-Z_]/ && !/^  local:/ {
            skip_until_next = 0
            in_local = 0
        }
        skip_until_next && /^    / { next }
        skip_until_next && /^  [a-zA-Z_]/ { skip_until_next = 0; in_local = 0 }

        # If we reach a top-level key after profiles: and never wrote local, inject it
        /^[a-zA-Z_]/ && !/^profiles:/ && !/^default_profile:/ {
            if (!wrote_local && seen_profiles) {
                print "  local:"
                print "    provider: openai"
                print "    base_url: http://localhost:11434/v1"
                print "    model_id: translategemma"
                wrote_local = 1
            }
        }

        /^profiles:/ { seen_profiles = 1 }

        { print }

        END {
            if (!wrote_local) {
                print "  local:"
                print "    provider: openai"
                print "    base_url: http://localhost:11434/v1"
                print "    model_id: translategemma"
            }
        }
        ' "$CONFIG_FILE" > "$TMPFILE"

        mv "$TMPFILE" "$CONFIG_FILE"
    else
        # Flat config — convert to multi-profile, preserving existing as "default"
        info "Converting flat config to multi-profile format..."
        TMPFILE=$(mktemp)
        trap 'rm -f "$TMPFILE"' EXIT

        # Extract existing flat values
        EXISTING_PROVIDER=$(grep "^provider:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed 's/^provider: *//' || echo "bedrock")
        EXISTING_REGION=$(grep "^aws_region:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed 's/^aws_region: *//' || echo "us-east-1")
        EXISTING_MODEL=$(grep "^model_id:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed 's/^model_id: *//' || echo "")
        EXISTING_SRC=$(grep "^default_source_language:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed 's/^default_source_language: *//' || echo "nl")
        EXISTING_TGT=$(grep "^default_target_language:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed 's/^default_target_language: *//' || echo "hu")
        EXISTING_DB=$(grep "^db_path:" "$CONFIG_FILE" 2>/dev/null | head -1 | sed 's/^db_path: *//' || echo "~/.vocabgen/vocabgen.db")

        cat > "$TMPFILE" <<YAML
default_profile: local
profiles:
  default:
    provider: ${EXISTING_PROVIDER}
    aws_region: ${EXISTING_REGION}
YAML
        # Only add model_id if it was set
        if [ -n "$EXISTING_MODEL" ]; then
            echo "    model_id: ${EXISTING_MODEL}" >> "$TMPFILE"
        fi
        cat >> "$TMPFILE" <<YAML
  local:
    provider: openai
    base_url: http://localhost:11434/v1
    model_id: translategemma
default_source_language: ${EXISTING_SRC}
default_target_language: ${EXISTING_TGT}
db_path: ${EXISTING_DB}
YAML
        mv "$TMPFILE" "$CONFIG_FILE"
    fi
else
    # No config file — create fresh
    cat > "$CONFIG_FILE" <<YAML
default_profile: local
profiles:
  local:
    provider: openai
    base_url: http://localhost:11434/v1
    model_id: translategemma
default_source_language: nl
default_target_language: hu
db_path: ~/.vocabgen/vocabgen.db
YAML
fi

info "Config written successfully."

# --- 7. Done ---

echo ""
echo "============================================"
echo "  Local LLM setup complete!"
echo "============================================"
echo ""
echo "Ollama is running with model '$MODEL'."
echo "Config: $CONFIG_FILE (profile: local)"
echo ""
echo "Next steps:"
echo "  vocabgen lookup \"fiets\" -l nl"
echo "  vocabgen serve"
echo ""
echo "To switch back to a cloud provider:"
echo "  vocabgen lookup \"fiets\" -l nl --profile default"
echo ""
