# User Guide

## Prerequisites

This app calls LLM APIs to generate vocabulary data. It does not include a built-in language model. You need one of:

- A paid API key from OpenAI, Anthropic, or AWS Bedrock (a free ChatGPT/Claude account does not include API access — you need a developer API key with a payment method on file)
- A local LLM server like [Ollama](https://ollama.com) (free, runs on your machine)

The easiest free option is Ollama:

```bash
# Install Ollama, then:
ollama pull llama3
vocabgen lookup "uitkomen" -l nl --provider openai --base-url http://localhost:11434/v1 --model-id llama3
```

Note on model quality: vocabgen's prompts are designed for large, capable models (Claude Sonnet/Opus, GPT-4o). Local models like Llama 3 will work but produce lower quality results — particularly for less common language pairs, connotation/register nuances, and contrastive notes. If translation quality matters to you, a paid API is worth it.

## Installation

Download the binary for your platform from the [GitHub Releases](https://github.com/user/vocabgen/releases) page.

```bash
# macOS (Apple Silicon)
curl -LO https://github.com/user/vocabgen/releases/latest/download/vocabgen_darwin_arm64.tar.gz
tar xzf vocabgen_darwin_arm64.tar.gz
chmod +x vocabgen
sudo mv vocabgen /usr/local/bin/

# Linux (amd64)
curl -LO https://github.com/user/vocabgen/releases/latest/download/vocabgen_linux_amd64.tar.gz
tar xzf vocabgen_linux_amd64.tar.gz
chmod +x vocabgen
sudo mv vocabgen /usr/local/bin/
```

## First Run

```bash
vocabgen lookup "uitkomen" -l nl
```

On first run, vocabgen creates `~/.vocabgen/` with a default config and SQLite database. The command sends "uitkomen" to the LLM provider (default: AWS Bedrock), validates the JSON response, stores it in the database, and prints the structured vocabulary entry as JSON.

Expected output (abbreviated):

```json
{
  "word": "uitkomen",
  "type": "werkwoord",
  "definition": "naar buiten komen; bekend worden; ...",
  "english": "to come out; to turn out",
  "target_translation": "kijönni; kiderülni",
  "notes": "Scheidbaar werkwoord: 'Ik kom uit'",
  "connotation": "neutraal",
  "register": "standaardtaal"
}
```

Running the same command again returns the cached result instantly (no API call).

## Batch Processing

Prepare a CSV file with one word/expression per line. No header row — all lines are treated as data.

| Column | Required | Description |
|--------|----------|-------------|
| 1 | Yes | Word or expression in the source language |
| 2 | No | Context sentence (triggers cache bypass for existing entries) |

```csv
uitkomen
werk,Het werk is klaar
aankomen
opvallen,Dat valt op in de klas
```

Rules:
- No header row — every non-empty line is processed
- Empty lines and whitespace-only lines are skipped
- Single-column lines: word only, no context
- Two-column lines: word + context sentence (comma-separated)
- UTF-8 encoding

Process the batch:

```bash
vocabgen batch --input-file ch1.csv --mode words -l nl --tags "chapter-1"
```

Output summary:

```
--- Batch Summary ---
Processed: 4
Cached:    0
Failed:    0
Skipped:   0
Replaced:  0
Added:     0
```

Useful flags:

```bash
# Limit to 5 new lookups (cached items don't count)
vocabgen batch --input-file ch1.csv --mode words -l nl --limit 5

# Preview without API calls or DB writes
vocabgen batch --input-file ch1.csv --mode words -l nl --dry-run

# Process expressions instead of words
vocabgen batch --input-file idioms.csv --mode expressions -l nl

# Auto-replace existing entries when context triggers a new lookup
vocabgen batch --input-file ch1.csv --mode words -l nl --on-conflict replace
```

## Web UI

Start the embedded web server:

```bash
vocabgen serve --port 8080
```

Open `http://localhost:8080` in your browser.

### Pages

- **Lookup** (`/`): Enter a word or expression, select source/target language, optionally provide context. Results display inline. Conflict resolution UI appears when an existing entry is found with a new context.
- **Batch** (`/batch`): Upload a CSV file, select mode and languages, set conflict strategy. Progress streams via SSE. Summary shows processed/cached/failed/replaced/added counts.
- **Config** (`/config`): View and edit provider settings, test connection to the LLM provider.
- **Database** (`/database`): Browse, search, edit, delete vocabulary entries. Import CSV, export to Excel. Filter by language, search text, or tags.

## Tags

Tag entries during lookup or batch processing:

```bash
vocabgen lookup "werk" -l nl --tags "chapter-1,nouns"
vocabgen batch --input-file ch2.csv --mode words -l nl --tags "chapter-2"
```

Tags are stored as comma-separated strings. Filter by tags in the web UI database page.

## Database Management

### Backup

On-demand via CLI:

```bash
vocabgen backup
# Output: /Users/you/.vocabgen/vocabgen.db.2025-01-15T10-30-00.bak
```

Or use the backup script (safe while DB is in use, auto-prunes old backups):

```bash
./scripts/backup.sh                     # backup to ~/.vocabgen/backups/
./scripts/backup.sh /path/to/backups    # backup to custom directory
```

For automated daily backups, add to cron:

```bash
crontab -e
# Add this line (daily at 2am):
0 2 * * * /path/to/vocabgen/scripts/backup.sh >> /tmp/vocabgen-backup.log 2>&1
```

The script keeps the last 30 backups by default (set `VOCABGEN_MAX_BACKUPS` to change).

### Restore

```bash
vocabgen restore /Users/you/.vocabgen/vocabgen.db.2025-01-15T10-30-00.bak
```

Restore creates a safety backup of the current database before overwriting.

### Import CSV

Via the web UI database page: upload a CSV file with vocabulary entries to bulk-import into the database. Duplicates (same word + source language) are skipped.

### Export to Excel

Via the web UI database page: export filtered entries as an `.xlsx` file. The download filename follows the pattern `vocabgen-{lang}-{type}-{date}.xlsx`.

## Provider Switching

### AWS Bedrock (default)

```bash
# Uses AWS credential chain (env vars, ~/.aws/credentials, IAM role)
vocabgen lookup "werk" -l nl

# Specify profile and region
vocabgen lookup "werk" -l nl --profile my-profile --region eu-west-1
```

#### AWS IAM: Least Privilege Setup

Create a dedicated IAM user or role for vocabgen with only Bedrock invoke permissions. This ensures the credentials can't be used for anything else.

Minimal IAM policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "VocabgenBedrockOnly",
      "Effect": "Allow",
      "Action": "bedrock:InvokeModel",
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/*",
        "arn:aws:bedrock:*:*:inference-profile/*"
      ]
    }
  ]
}
```

To restrict further — specific models and regions only:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "VocabgenBedrockOnly",
      "Effect": "Allow",
      "Action": "bedrock:InvokeModel",
      "Resource": [
        "arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-*",
        "arn:aws:bedrock:us-east-1:*:inference-profile/us.anthropic.claude-*"
      ],
      "Condition": {
        "StringEquals": {
          "aws:RequestedRegion": "us-east-1"
        }
      }
    }
  ]
}
```

You can also add an IP condition to restrict where calls originate:

```json
"Condition": {
  "IpAddress": {
    "aws:SourceIp": "YOUR_HOME_IP/32"
  },
  "StringEquals": {
    "aws:RequestedRegion": "us-east-1"
  }
}
```

Recommended setup:

```bash
# Create a dedicated AWS profile
aws configure --profile vocabgen
# Set in config: provider: bedrock, aws_profile: vocabgen
```

Verify access:

```bash
aws sts get-caller-identity --profile vocabgen
```

### OpenAI

```bash
export OPENAI_API_KEY=sk-...
vocabgen lookup "werk" -l nl --provider openai --model-id gpt-4o
```

### Anthropic

```bash
export ANTHROPIC_API_KEY=sk-ant-...
vocabgen lookup "werk" -l nl --provider anthropic --model-id claude-sonnet-4-20250514
```

### Ollama (local)

```bash
vocabgen lookup "werk" -l nl --provider openai --base-url http://localhost:11434/v1 --model-id llama3
```

No API key needed for local servers.

### Azure OpenAI

```bash
vocabgen lookup "werk" -l nl --provider openai \
  --base-url https://my-resource.openai.azure.com \
  --api-key your-azure-key \
  --model-id gpt-4o
```

## Dry-Run Mode

Preview what would happen without making API calls or writing to the database:

```bash
vocabgen lookup "werk" -l nl --dry-run
vocabgen batch --input-file ch1.csv --mode words -l nl --dry-run
```

Dry-run normalizes tokens and checks the cache but skips LLM invocation and DB writes. Use this to verify your input file and estimate API costs before processing.

## Conflict Resolution

When you look up a word that already exists in the database with a context sentence, vocabgen bypasses the cache and gets a fresh LLM result. You then choose how to handle the conflict:

## Adding Languages

Any language works out of the box — just pass the full name:

```bash
vocabgen lookup "maison" -l French
vocabgen lookup "家" -l Japanese
```

The 11 built-in shorthand codes (nl, hu, it, ru, en, de, fr, es, pt, pl, tr) resolve to full names automatically. To add a new shorthand, add one line to `internal/language/registry.go`:

```go
var SupportedLanguages = map[string]string{
    // ... existing entries ...
    "ja": "Japanese",   // ← add this
}
```

No other code changes needed — templates are language-agnostic.

## Conflict Resolution

- **replace**: Update the existing entry with the new result
- **add**: Keep both entries (multi-version)
- **skip**: Discard the new result

Interactive mode (CLI prompts you):

```bash
vocabgen lookup "werk" -l nl --context "Het werk is klaar"
```

Auto-resolve:

```bash
vocabgen lookup "werk" -l nl --context "Het werk is klaar" --on-conflict add
```

Batch default is `skip`. Override with `--on-conflict`:

```bash
vocabgen batch --input-file ch1.csv --mode words -l nl --on-conflict replace
```
