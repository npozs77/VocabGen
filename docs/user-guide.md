# User Guide

## Prerequisites

This app calls LLM APIs to generate vocabulary data. It does not include a built-in language model. You need one of:

- A paid API key from OpenAI, Anthropic, or AWS Bedrock (a free ChatGPT/Claude account does not include API access — you need a developer API key with a payment method on file)
- A local LLM server like [Ollama](https://ollama.com) (free, runs on your machine)

The easiest free option is Ollama:

```bash
# Install Ollama, then:
ollama pull translategemma
vocabgen lookup "uitkomen" -l nl --provider openai --base-url http://localhost:11434/v1 --model-id translategemma
```

> **💡 Free to try, better with a paid model.** vocabgen's prompts are designed for large, capable models (Claude Sonnet/Opus, GPT-4o). Local models like Llama 3 will work but produce lower quality results — particularly for less common language pairs, connotation/register nuances, and contrastive notes. If translation quality matters to you, a paid API is worth it.

## Installation

Download the binary for your platform from the [GitHub Releases](https://github.com/npozs77/VocabGen/releases) page.

```bash
# macOS (Apple Silicon)
curl -LO https://github.com/npozs77/VocabGen/releases/latest/download/vocabgen_darwin_arm64.tar.gz
tar xzf vocabgen_darwin_arm64.tar.gz
chmod +x vocabgen
sudo mv vocabgen /usr/local/bin/

# macOS (Intel)
curl -LO https://github.com/npozs77/VocabGen/releases/latest/download/vocabgen_darwin_amd64.tar.gz
tar xzf vocabgen_darwin_amd64.tar.gz
chmod +x vocabgen
sudo mv vocabgen /usr/local/bin/

# Linux (amd64)
curl -LO https://github.com/npozs77/VocabGen/releases/latest/download/vocabgen_linux_amd64.tar.gz
tar xzf vocabgen_linux_amd64.tar.gz
chmod +x vocabgen
sudo mv vocabgen /usr/local/bin/

# Windows (amd64)
# Download vocabgen_windows_amd64.zip from the Releases page, extract, and add to PATH.
```

### Docker (alternative)

Run VocabGen as a container without downloading platform-specific binaries:

```bash
docker run -d \
  -p 8080:8080 \
  -v ./data:/data \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  ghcr.io/npozs77/vocabgen:latest
```

Inside the container, vocabgen automatically defaults to `/data/vocabgen.db` — just mount a host directory to `/data` and you're set. No `chown` or UID mapping needed. Pass API keys via `-e`. All CLI commands work via Docker:

```bash
docker run ghcr.io/npozs77/vocabgen:latest version
docker run -e OPENAI_API_KEY=sk-... ghcr.io/npozs77/vocabgen:latest lookup "werk" -l nl --provider openai
```

See [Deployment — Docker](deployment.md#docker) for image tags and detailed usage.

## Configuration

On first run, vocabgen creates `~/.vocabgen/config.yaml` with defaults:

| Setting | Default |
|---------|---------|
| Provider | `bedrock` |
| Region | `us-east-1` |
| Source language | `nl` (Dutch) |
| Target language | `hu` (Hungarian) |
| Database | `~/.vocabgen/vocabgen.db` |

You can configure the app in two ways: the Web UI (recommended for most users) or CLI flags.

### Quick Setup via Web UI

The easiest way to configure vocabgen — no file editing required:

```bash
vocabgen serve --port 8080
```

Open `http://localhost:8080/config` in your browser. From there you can:

- Select your LLM provider (Bedrock, OpenAI, Anthropic, Vertex AI)
- Set your default source and target languages
- Choose a model ID
- Test the connection (uses the API key from your environment variables automatically)

Credentials must still be configured outside the Web UI — see [Provider Credentials](#provider-credentials) below. The config page saves provider, model, and language settings to `~/.vocabgen/config.yaml`.

### CLI Flag Overrides

You can also override any config setting per-command without editing files:

```bash
# OpenAI with API key from environment
export OPENAI_API_KEY=sk-...
vocabgen lookup "maison" -l French --provider openai --model-id gpt-4o

# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...
vocabgen lookup "Haus" -l German --provider anthropic --model-id claude-sonnet-4-20250514

# Ollama (local, free, no API key needed)
vocabgen lookup "casa" -l Italian --provider openai --base-url http://localhost:11434/v1 --model-id translategemma
```

CLI flags always take precedence over `config.yaml` values.

### Provider Credentials

Each provider authenticates differently:

| Provider | Credentials |
|----------|-------------|
| Bedrock | AWS credential chain (`~/.aws/credentials`, env vars, or IAM role) |
| OpenAI | `OPENAI_API_KEY` env var or `--api-key` flag |
| Anthropic | `ANTHROPIC_API_KEY` env var or `--api-key` flag |
| Vertex AI | Google Application Default Credentials + `--gcp-project` or `GCP_PROJECT` env var |
| Ollama | None (local server) |

API keys are never stored in `config.yaml`. Use environment variables or CLI flags.

### Config Profiles

vocabgen supports multiple named config profiles, so you can save different LLM setups and switch between them without re-editing `config.yaml` each time.

A multi-profile `config.yaml` looks like this:

```yaml
default_profile: default
default_source_language: nl
default_target_language: hu
db_path: ~/.vocabgen/vocabgen.db
profiles:
  default:
    provider: bedrock
    aws_region: us-east-1
    aws_profile: vocabgen
  local:
    provider: openai
    base_url: http://localhost:11434/v1
    model_id: mistral
  anthropic:
    provider: anthropic
```

Each profile stores provider-specific fields (`provider`, `aws_profile`, `aws_region`, `model_id`, `base_url`, `gcp_project`, `gcp_region`). Global settings like `default_source_language`, `default_target_language`, and `db_path` live outside the profiles block.

Switch profiles on the CLI with `--profile`:

```bash
vocabgen lookup "werk" -l nl --profile local
vocabgen batch --input-file ch1.csv --mode words -l nl --profile anthropic
```

The `default_profile` setting in `config.yaml` determines which profile is used when `--profile` is not specified. If you omit `--profile`, vocabgen uses the `default_profile` value.

In the Web UI (`/config`), a profile dropdown at the top of the Config page lets you switch between profiles. Select "Add new profile…" from the dropdown to create a new profile — it copies the current profile's values as a starting point.

Old config files without a `profiles:` key still work. vocabgen treats a flat config as a single implicit `default` profile, so existing setups are fully backward compatible.

> **Note:** The `--profile` flag now selects a config profile. The old `--profile` flag (which selected an AWS credential profile) has been renamed to `--aws-profile`.

### Local LLM Setup

The `scripts/setup-local-llm.sh` script automates local LLM setup via Ollama. It detects your OS (macOS or Linux), installs Ollama if needed, pulls a recommended model, verifies it responds, and writes a `local` profile to `~/.vocabgen/config.yaml`.

```bash
./scripts/setup-local-llm.sh
```

After setup completes, use the local profile:

```bash
vocabgen lookup "fiets" -l nl --profile local
```

Requirements: macOS or Linux, ~4 GB disk space for the model. No API key or cloud account needed.

In the Web UI, the Config page includes a "Setup Local LLM" button that runs the same setup logic. Progress streams to the browser via SSE (detecting OS, checking Ollama, installing, pulling model, verifying, writing config).

If you already have Ollama installed, the script skips installation and proceeds to model setup. See [Prerequisites](#prerequisites) for more on local models.

## First Run

Make sure you have configured a provider (see [Configuration](#configuration) above). The default provider is AWS Bedrock — if you don't have AWS credentials set up, switch to a different provider first.

The recommended way to get started is the web app:

```bash
vocabgen serve --port 8080
```

Open `http://localhost:8080` in your browser. On first launch, go to the **Config** page (`http://localhost:8080/config`) to set up your LLM provider — lookups won't work until a provider is configured. From there you can look up words, upload CSV files for batch processing, study with flashcards, and manage your vocabulary database — all without touching the command line.

For power users and scripting, the CLI is also available:

```bash
# Look up a word from the terminal
vocabgen lookup "uitkomen" -l nl

# Or with OpenAI
vocabgen lookup "uitkomen" -l nl --provider openai --model-id gpt-4o

# Or with a free local model via Ollama
vocabgen lookup "uitkomen" -l nl --provider openai --base-url http://localhost:11434/v1 --model-id translategemma
```

On first run, vocabgen creates `~/.vocabgen/` with a default config and SQLite database. Lookups send the word to the LLM provider, validate the JSON response, and store it in the database. Running the same lookup again returns the cached result instantly (no API call).

## Web UI

Start the web app:

```bash
vocabgen serve --port 8080
```

Open `http://localhost:8080` in your browser. This is the primary way to use VocabGen — everything is available from the browser.

### Pages

- **Lookup** (`/`): Enter a word or expression, select source/target language, optionally provide context. Results display inline. Conflict resolution UI appears when an existing entry is found with a new context. Select "Sentence" type to analyze a full sentence — the LLM checks grammar, provides corrections, translates the sentence, and extracts key vocabulary. Sentence lookups are ephemeral (not stored in the database).
- **Batch** (`/batch`): Upload a CSV file, select mode and languages, set conflict strategy. Progress streams via SSE. Cancel a running batch at any time — partial results are preserved. Summary shows processed/cached/failed/replaced/added counts.
- **Flashcards** (`/flashcards`): Study vocabulary with a flip-card interface. Filter by language, tags, and difficulty. Rate cards as easy/hard/natural to focus future sessions. See [Flashcards](#flashcards) below.
- **Config** (`/config`): View and edit provider settings, test connection to the LLM provider. Credential env var hints are shown per provider; API keys are read from environment variables automatically. On first launch, the "default" profile is shown — edit the fields and click Save to configure your provider. Use "Add new profile…" in the profile dropdown to create additional setups (e.g., a "local" profile for Ollama and a "prod" profile for Bedrock).
- **Database** (`/database`): Browse, search, edit, delete vocabulary entries. Select individual entries or use select-all to bulk delete. Import CSV, export to Excel. Filter by language, search text, or tags.

### Help Menu

The navigation bar includes a Help dropdown with:

- **About** (`/about`): Version info, build date, Go version, and links to the GitHub repository.
- **Report an Issue**: Opens the [GitHub Issues](https://github.com/npozs77/VocabGen/issues) page in a new tab.
- **Documentation** (`/docs`): Browsable documentation index with deep links into Architecture, Deployment, and User Guide sections. Rendered from the embedded `docs/*.md` files via goldmark.
- **Changelog** (`/changelog`): Full project changelog rendered as formatted HTML from the embedded `CHANGELOG.md`.
- **Check for Update** (`/update`): Displays the current version, build date, and OS/architecture. On page load, queries the GitHub Releases API to check for newer versions. If an update is available, shows the latest version, a direct download link for your platform, and a delta changelog covering all releases between your version and the latest. If the API is unreachable, displays a fallback message with a manual link to GitHub Releases.

When the web server starts, it performs a background update check. If a newer version is detected, a dismissible banner appears below the navigation bar on all pages with a link to the update page. The banner does not reappear after dismissal until the server is restarted.

## Batch Processing (CLI)

For scripting and automation, batch processing is also available from the command line. Most users will prefer the web UI Batch page instead.

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

## Sentence Lookup

Sentence lookup analyzes a full sentence for grammar, vocabulary, and meaning. Select "Sentence" as the lookup type in the web UI or use `--type sentence` from the CLI:

```bash
vocabgen lookup "Ik ga morgen naar de markt" -l nl --type sentence
```

The LLM response includes:
- Grammar check with corrections and explanations for each error
- Full sentence translation (English and target language)
- Key vocabulary extracted from the sentence (2–5 items)
- Notes on register and usage

Sentence lookups are ephemeral — results are displayed but not stored in the database. Each lookup always invokes the LLM (no caching).

## Flashcards

The Flashcards page (`/flashcards`) lets you study vocabulary entries as flip cards. Click "Flashcards" in the navigation bar or go to `http://localhost:8080/flashcards`.

### Filter Controls

Five dropdowns at the top of the page control which cards appear:

| Filter | Options | Default |
|--------|---------|---------|
| Source Language | All supported languages, or "All" | All |
| Target Language | All supported languages, or "All" | All |
| Tags | All tags present in the database, or "All" | All |
| Difficulty | Hard + Natural, Hard only, All, Easy only | Hard + Natural |
| Display Mode | Word first, Detail first | Word first |

Changing any filter reloads the deck automatically (no page refresh). The default difficulty filter hides easy cards so you focus on words that still need practice.

### Card Flip

- Click the card to flip between front and back
- In "Word first" mode (default): front shows the source-language word, back shows definition, English translation, and target translation
- In "Detail first" mode: front shows definition and translations, back shows the word
- Flipping is instant (client-side animation, no server call)

### Navigation

- Use the **Prev** and **Next** buttons below the card to move through the deck
- The status bar between the buttons shows your position as "N / T" (e.g., "3 / 47")
- Prev is disabled on the first card; Next is disabled on the last card
- Navigating to a new card always shows the front face

### Difficulty Rating

Three buttons below the card let you rate each entry:

| Button | Meaning |
|--------|---------|
| Easy | Mastered — hidden by default filter |
| Natural | Normal — included by default |
| Hard | Needs practice — included by default |

Ratings persist in the database across sessions. The active rating is highlighted on the current card. Rating a card does not advance to the next card or flip it.

### Display Mode

Toggle between "Word first" and "Detail first" in the Display Mode dropdown:

- **Word first**: See the source-language word, then flip to reveal the definition and translations
- **Detail first**: See the definition and translations, then flip to reveal the word

Changing the mode keeps your position in the deck and resets the card to the front face.

### Empty State

If no entries match your filters, the card area shows: "No flashcards match your filters. Try adjusting your filters or add vocabulary via Lookup or Batch."

## Tags

Tag entries during lookup or batch processing:

```bash
vocabgen lookup "werk" -l nl --tags "chapter-1,nouns"
vocabgen batch --input-file ch2.csv --mode words -l nl --tags "chapter-2"
```

Tags are stored as comma-separated strings. Filter by tags in the web UI database page.

### Tag Picker (Web UI)

All pages that use tags share a unified tag picker component:

- **Database page**: A dropdown filter populated from existing tags in the database. Select one or more tags to filter the entry list — works the same way as the language and type filters. Select "All" to clear the filter.
- **Lookup and Batch pages**: A text input with autocomplete suggestions from existing tags. Start typing to see matching tags, or click the field to see all available tags. You can also type a new tag that doesn't exist yet. Separate multiple tags with commas.
- **Flashcards page**: A dropdown filter identical to the Database page — select tags to narrow the study deck.

Tags are fetched from the `GET /api/tags` endpoint, which returns all distinct tags across words and expressions.

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
# For cross-region inference profiles, prefix with region (e.g., us.)
vocabgen lookup "werk" -l nl --model-id us.anthropic.claude-sonnet-4-20250514-v1:0

# Specify AWS profile and region
vocabgen lookup "werk" -l nl --aws-profile my-profile --region us-east-1 --model-id us.anthropic.claude-3-5-haiku-20241022-v1:0
```

> **Note:** Bedrock model IDs differ from direct Anthropic API IDs. Use the Bedrock format (e.g., `us.anthropic.claude-sonnet-4-20250514-v1:0`) not the Anthropic format (`claude-sonnet-4-20250514`). For cross-region inference profiles, include the region prefix (`us.`).

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
vocabgen lookup "werk" -l nl --provider openai --base-url http://localhost:11434/v1 --model-id translategemma
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

## Checking for Updates

Check for newer versions from the command line without starting the web server:

```bash
vocabgen update
```

This queries the GitHub Releases API and displays the current version, latest available version, a download link for your OS and architecture, and a delta changelog covering all releases between your version and the latest. If you're already on the latest version, it prints a "You're up to date" message. If the API is unreachable, it prints an error and exits with a non-zero status.

The `vocabgen version` command also performs a quick update check:

```bash
vocabgen version
```

If a newer version is available, it appends a one-line notice:

```
Update available: v1.1.0 — run 'vocabgen update' for details
```

If the current version is the latest or the API is unreachable, no extra output is shown. Update checks are only performed on the `version` and `update` subcommands — no other command makes network calls to GitHub.

## User Data and Updates

All user data is stored in `~/.vocabgen/`, independent of where the vocabgen binary is located:

| File | Purpose |
|------|---------|
| `~/.vocabgen/config.yaml` | Application configuration (provider, languages, model) |
| `~/.vocabgen/vocabgen.db` | SQLite vocabulary database (cached lookups, entries) |
| `~/.vocabgen/vocabgen-dev.db` | Dev database (used by `make dev-serve`, never touched by production) |

The web UI displays the active database path in the navigation bar (next to the profile indicator), so you can always tell at a glance which database you're connected to.

To use a custom database path:

```bash
vocabgen serve --db-path ~/.vocabgen/my-project.db
```

If the file doesn't exist, it's created automatically. The `--db-path` flag overrides the `db_path` setting in `config.yaml`.

Replacing the binary (e.g., downloading a new release) does not affect your configuration or vocabulary data. You can safely update vocabgen by overwriting the binary in place — your settings and database remain untouched.

## E2E Testing

Run end-to-end tests with `scripts/e2e-test.sh`. By default, the script uses the `local` profile (Ollama), so tests are free and don't consume cloud API credits.

```bash
./scripts/e2e-test.sh              # defaults to --profile local
./scripts/e2e-test.sh -p anthropic # use a cloud provider
E2E_PROFILE=local make e2e         # via Makefile
```

Override the profile with the `-p` flag or the `E2E_PROFILE` environment variable. The flag takes precedence over the env var.

Prerequisite: if using the `local` profile, run `./scripts/setup-local-llm.sh` first to install Ollama and pull the model. The script checks Ollama reachability before running tests and prints an actionable error if it's not available.

The `make e2e` target passes `E2E_PROFILE` through to the script automatically.

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

When you look up a word that already exists in the database with a context sentence, vocabgen bypasses the cache and gets a fresh LLM result. You then choose how to handle the conflict:

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
