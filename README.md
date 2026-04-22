# VocabGen — Vocabulary Generator & Flashcard App for Language Learners

<!-- Badges -->
![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Build Status](https://github.com/npozs77/VocabGen/actions/workflows/ci.yml/badge.svg?branch=main)
![License](https://img.shields.io/badge/license-MIT-blue)
![GitHub Release](https://img.shields.io/github/v/release/npozs77/VocabGen)

**VocabGen** is an open-source vocabulary web app and flashcard tool for language learners. Look up words, batch-process CSV word lists, and study with flashcards — all in your browser. Powered by LLM providers (OpenAI, Anthropic, AWS Bedrock, Ollama), it ships as a single binary with zero setup friction.

## Features

- 🖥️ **Web-based vocabulary app** — browser interface for word lookup, batch processing, flashcard study, database management, and configuration
- 🃏 **Flashcard study mode** — flip cards, rate difficulty, filter by language/tags/difficulty
- 📝 **Sentence analysis** — grammar checking, corrections, key vocabulary extraction, and translation
- 🔤 **LLM-powered vocabulary lookup** — get definitions, translations, connotation, and register for any word or expression
- 📄 **CSV batch processing** — upload word lists via the web UI or process them from the command line
- 🌍 **Multi-language support** — Dutch, German, French, Spanish, Italian, Russian, Portuguese, Polish, Turkish, Hungarian, English, and any custom language
- 🤖 **Multiple LLM providers** — OpenAI, Anthropic, AWS Bedrock, Google Vertex AI, Ollama (free local), Azure OpenAI, LM Studio
- 💾 **SQLite cache** — never pay twice for the same lookup
- 📦 **Single binary** — zero dependencies, cross-platform (macOS, Linux, Windows), Docker support
- 🔒 **Privacy-first** — runs locally, no telemetry, API keys stay in env vars
- ⌨️ **CLI for power users** — script lookups and batch jobs from the terminal

## Prerequisites

This app calls LLM APIs to generate vocabulary data. It does not include a built-in language model. You need one of the following:

| Option | What you need | Cost |
|--------|--------------|------|
| AWS Bedrock | AWS account with Bedrock model access enabled | Pay-per-token |
| OpenAI API | API key from [platform.openai.com](https://platform.openai.com) | Pay-per-token |
| Anthropic API | API key from [console.anthropic.com](https://console.anthropic.com) | Pay-per-token |
| Ollama (local) | [Ollama](https://ollama.com) installed with a model pulled | Free (runs on your hardware) |
| LM Studio / vLLM | Any OpenAI-compatible local server | Free (runs on your hardware) |

A free ChatGPT, Claude, or Gemini account does not provide API access. You need a separate API key from the provider's developer platform, which requires a payment method on file.

> **💡 Free to try, better with a paid model.** The cheapest way to get started is Ollama — install it, pull a model (`ollama pull translategemma`), and point vocabgen at it with `--provider openai --base-url http://localhost:11434/v1`.
>
> For best translation quality, use a large model (Claude Sonnet/Opus, GPT-4o). Local models like Llama 3 work but produce noticeably lower quality for nuanced vocabulary tasks — especially for less common languages, connotation/register distinctions, and contrastive notes. Use `--dry-run` to preview results before committing to a provider.

## Quick Start

```bash
# Start the web app (lookup, batch, flashcards — all in your browser)
vocabgen serve --port 8080
```

Open `http://localhost:8080` — configure your LLM provider on the Config page, then start looking up words and studying flashcards.

For scripting and automation, the CLI is also available:

```bash
# Look up a word from the terminal
vocabgen lookup "maison" -l French

# Batch-process a CSV word list
vocabgen batch --input-file ch1.csv --mode words -l nl --tags "chapter-1"
```

## CLI Usage

| Command | Description | Example |
|---------|-------------|---------|
| `lookup` | Look up a word, expression, or sentence | `vocabgen lookup "werk" -l nl --type word` |
| `batch` | Process words/expressions from a CSV file | `vocabgen batch --input-file words.csv --mode words -l nl` |
| `serve` | Start the embedded web UI | `vocabgen serve --port 8080` |
| `backup` | Create a timestamped database backup | `vocabgen backup` |
| `restore` | Restore database from a backup file | `vocabgen restore vocabgen.db.2025-01-15T10-30-00.bak` |
| `version` | Print version, Go version, and build date | `vocabgen version` |

### Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--source-language` | `-l` | (from config) | Source language code or name |
| `--target-language` | | `hu` | Target language for translations |
| `--provider` | | `bedrock` | LLM provider (`bedrock`, `openai`, `anthropic`, `vertexai`) |
| `--model-id` | | | LLM model identifier |
| `--api-key` | | | API key (OpenAI/Anthropic) |
| `--base-url` | | | Custom API base URL (Ollama, Azure, LM Studio) |
| `--region` | `-r` | `us-east-1` | AWS region (Bedrock) |
| `--profile` | | | Config profile name |
| `--aws-profile` | | | AWS credential profile name (Bedrock) |
| `--gcp-project` | | | GCP project ID (Vertex AI) |
| `--gcp-region` | | `us-central1` | GCP region (Vertex AI) |
| `--timeout` | | `60` | Per-request timeout in seconds |
| `--tags` | | | Comma-separated tags for entries |
| `--verbose` | `-v` | `false` | Enable debug logging |
| `--db-path` | | | Override database file path |

### lookup

```bash
vocabgen lookup "uitkomen" -l nl
vocabgen lookup "werk" -l nl --type word --context "Het werk is klaar"
vocabgen lookup "het zit zo" -l nl --type expression
vocabgen lookup "Ik ga morgen naar de markt" -l nl --type sentence  # grammar check + vocabulary
vocabgen lookup "uitkomen" -l nl --on-conflict replace   # auto-resolve conflicts
vocabgen lookup "uitkomen" -l nl --dry-run               # preview without API call
```

### batch

```bash
vocabgen batch --input-file ch1.csv --mode words -l nl
vocabgen batch --input-file idioms.csv --mode expressions -l nl --tags "chapter-3"
vocabgen batch --input-file ch1.csv --mode words -l nl --limit 10
vocabgen batch --input-file ch1.csv --mode words -l nl --dry-run
vocabgen batch --input-file ch1.csv --mode words -l nl --on-conflict replace
```

Input CSV format (no header row, or with header row — columns mapped by name):

```csv
uitkomen
werk,Het werk is klaar
aankomen
```

### serve

```bash
vocabgen serve                # default port 8080
vocabgen serve --port 3000    # custom port
```

For first-time setup, the Web UI config page is the easiest way to configure your provider and languages — see [Quick Setup via Web UI](docs/user-guide.md#quick-setup-via-web-ui). Credentials (API keys, AWS profiles) must be set via environment variables before starting the server.

## Provider Configuration

| Provider | Auth | Example |
|----------|------|---------|
| Bedrock (default) | AWS credential chain | `vocabgen lookup "word" -l nl --aws-profile my-profile --region us-east-1 --model-id us.anthropic.claude-sonnet-4-20250514-v1:0` |
| OpenAI | API key | `vocabgen lookup "word" -l nl --provider openai --api-key sk-...` |
| Anthropic | API key | `vocabgen lookup "word" -l nl --provider anthropic --api-key sk-ant-...` |
| Vertex AI | Google ADC | `vocabgen lookup "word" -l nl --provider vertexai --gcp-project my-proj` |
| Ollama (local) | None | `vocabgen lookup "word" -l nl --provider openai --base-url http://localhost:11434/v1` |
| Azure OpenAI | API key + URL | `vocabgen lookup "word" -l nl --provider openai --base-url https://my-resource.openai.azure.com --api-key ...` |

## Local LLM (Ollama)

Set up a free local LLM with one command:

```bash
./scripts/setup-local-llm.sh
vocabgen lookup "fiets" -l nl --profile local
```

The script installs Ollama (if needed), pulls a model, and creates a `local` config profile. See the [User Guide](docs/user-guide.md#local-llm-setup) for details.

## Environment Variables

| Variable | Provider | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | OpenAI | API key (overridden by `--api-key`) |
| `ANTHROPIC_API_KEY` | Anthropic | API key (overridden by `--api-key`) |
| `GCP_PROJECT` | Vertex AI | GCP project ID (overridden by `--gcp-project`) |
| `AWS_PROFILE` | Bedrock | AWS profile (overridden by `--aws-profile`) |
| `AWS_REGION` | Bedrock | AWS region (overridden by `--region`) |

## Configuration

Settings are stored in `~/.vocabgen/config.yaml`. CLI flags override config values. The config supports named profiles for different LLM setups:

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
```

Switch profiles with `--profile`:

```bash
vocabgen lookup "werk" -l nl --profile local
```

Old flat config files (without `profiles:`) still work — they're treated as a single `default` profile. See the [User Guide](docs/user-guide.md#config-profiles) for details.

API keys are never stored in the config file — use environment variables or `--api-key`.

vocabgen supports named config profiles for switching between providers. See [Config Profiles](docs/user-guide.md#config-profiles) for details.

```yaml
# Multi-profile example
default_profile: default
default_source_language: nl
default_target_language: hu
db_path: ~/.vocabgen/vocabgen.db
profiles:
  default:
    provider: bedrock
    aws_region: us-east-1
  local:
    provider: openai
    base_url: http://localhost:11434/v1
    model_id: mistral
```

Switch profiles with `--profile`:

```bash
vocabgen lookup "werk" -l nl --profile local
```

## Supported Languages

Dutch (nl), Hungarian (hu), Italian (it), Russian (ru), English (en), German (de), French (fr), Spanish (es), Portuguese (pt), Polish (pl), Turkish (tr).

Any language name or code can be passed via `-l` — unregistered languages are used as-is in prompts.

## Documentation

| Document | Description |
|----------|-------------|
| [User Guide](docs/user-guide.md) | Installation, first run, batch processing, web UI, provider setup, AWS IAM, adding languages |
| [Architecture](docs/architecture.md) | System design, package layout, data flows, data models, error handling, API routes |
| [Deployment](docs/deployment.md) | Cross-compilation, GitHub Actions CI/CD, goreleaser, Docker, release process |

Documentation is also available in the web UI via Help → Documentation when running `vocabgen serve`.

## Docker

```bash
docker run -d \
  -p 8080:8080 \
  -v ~/.vocabgen:/home/nonroot/.vocabgen \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  ghcr.io/npozs77/vocabgen:latest
```

The volume mount persists config and database across restarts. Pass API keys via `-e`. All CLI commands work: `docker run ghcr.io/npozs77/vocabgen:latest lookup "werk" -l nl`.

## Build from Source

```bash
git clone https://github.com/npozs77/VocabGen.git
cd vocabgen
make build        # produces ./vocabgen binary
make test         # run all tests with race detection
make vet          # go vet
make lint         # staticcheck
make quality      # build + vet + fmt-check + tests + coverage
```

## License

MIT License. See [LICENSE](LICENSE).

This software is provided as-is. You are responsible for any API costs incurred through LLM providers (AWS Bedrock, OpenAI, Anthropic). Use `--dry-run` to preview operations before making API calls. See the [User Guide](docs/user-guide.md) for AWS IAM least-privilege setup to limit what your credentials can access.
