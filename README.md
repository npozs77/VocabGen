# vocabgen

A single-binary CLI and embedded web app that generates structured B2→C1 vocabulary lists for language learners. It processes words and expressions through LLM providers (AWS Bedrock, OpenAI, Anthropic), validates JSON responses against English schemas, caches results in SQLite, and serves a browser-based HTMX interface — all compiled into one executable with zero runtime dependencies.

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

The cheapest way to get started is Ollama — install it, pull a model (`ollama pull llama3`), and point vocabgen at it with `--provider openai --base-url http://localhost:11434/v1`.

For best translation quality, use a large model (Claude Sonnet/Opus, GPT-4o). Local models like Llama 3 work but produce noticeably lower quality for nuanced vocabulary tasks — especially for less common languages, connotation/register distinctions, and contrastive notes. Use `--dry-run` to preview results before committing to a provider.

## Quick Start

```bash
# Look up a Dutch word (default provider: Bedrock)
vocabgen lookup "uitkomen" -l nl

# Process a batch of words from CSV
vocabgen batch --input-file ch1.csv --mode words -l nl --tags "chapter-1"

# Start the web UI
vocabgen serve --port 8080
```

## CLI Usage

| Command | Description | Example |
|---------|-------------|---------|
| `lookup` | Look up a single word or expression | `vocabgen lookup "werk" -l nl --type word` |
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
| `--profile` | | | AWS profile name (Bedrock) |
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

Input CSV format (no header row):

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

## Provider Configuration

| Provider | Auth | Example |
|----------|------|---------|
| Bedrock (default) | AWS credential chain | `vocabgen lookup "word" -l nl --profile my-profile --region eu-west-1` |
| OpenAI | API key | `vocabgen lookup "word" -l nl --provider openai --api-key sk-...` |
| Anthropic | API key | `vocabgen lookup "word" -l nl --provider anthropic --api-key sk-ant-...` |
| Vertex AI | Google ADC | `vocabgen lookup "word" -l nl --provider vertexai --gcp-project my-proj` |
| Ollama (local) | None | `vocabgen lookup "word" -l nl --provider openai --base-url http://localhost:11434/v1` |
| Azure OpenAI | API key + URL | `vocabgen lookup "word" -l nl --provider openai --base-url https://my-resource.openai.azure.com --api-key ...` |

## Environment Variables

| Variable | Provider | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | OpenAI | API key (overridden by `--api-key`) |
| `ANTHROPIC_API_KEY` | Anthropic | API key (overridden by `--api-key`) |
| `GCP_PROJECT` | Vertex AI | GCP project ID (overridden by `--gcp-project`) |
| `AWS_PROFILE` | Bedrock | AWS profile (overridden by `--profile`) |
| `AWS_REGION` | Bedrock | AWS region (overridden by `--region`) |

## Configuration

Settings are stored in `~/.vocabgen/config.yaml`. CLI flags override config values.

```yaml
provider: bedrock
aws_region: us-east-1
default_source_language: nl
default_target_language: hu
db_path: ~/.vocabgen/vocabgen.db
# model_id: claude-sonnet-4-20250514
# aws_profile: my-profile
# base_url: http://localhost:11434/v1
# gcp_project: my-project
# gcp_region: us-central1
```

API keys are never stored in the config file — use environment variables or `--api-key`.

## Supported Languages

Dutch (nl), Hungarian (hu), Italian (it), Russian (ru), English (en), German (de), French (fr), Spanish (es), Portuguese (pt), Polish (pl), Turkish (tr).

Any language name or code can be passed via `-l` — unregistered languages are used as-is in prompts.

## Build from Source

```bash
git clone https://github.com/user/vocabgen.git
cd vocabgen
make build        # produces ./vocabgen binary
make test         # run all tests with race detection
make vet          # go vet
make lint         # staticcheck
make quality      # build + vet + fmt-check + tests + coverage
```

## License

See [LICENSE](LICENSE) for details.
