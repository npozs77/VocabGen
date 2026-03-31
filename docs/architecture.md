# Architecture

## System Architecture

```mermaid
graph TB
    subgraph "Single Binary"
        CLI["cmd/vocabgen<br/>Cobra CLI"]
        WEB["internal/web<br/>net/http + HTMX"]
        SVC["internal/service<br/>Lookup · ProcessBatch"]
        LANG["internal/language<br/>Templates · Validation · Registry"]
        PARSE["internal/parsing<br/>CSV · Normalization"]
        LLM["internal/llm<br/>Provider Interface"]
        DB["internal/db<br/>SQLite · Cache"]
        CFG["internal/config<br/>YAML Config"]
        OUT["internal/output<br/>Field Mapping · Excel"]
    end

    CLI --> SVC
    WEB --> SVC
    SVC --> LANG
    SVC --> LLM
    SVC --> DB
    SVC --> PARSE
    SVC --> OUT
    LLM --> BEDROCK["AWS Bedrock"]
    LLM --> OPENAI["OpenAI API<br/>(+ Azure, Ollama, LM Studio)"]
    LLM --> ANTHROPIC["Anthropic API"]
    LLM --> VERTEXAI["Google Vertex AI"]
    CLI --> CFG
    WEB --> CFG
    DB --> SQLITE["~/.vocabgen/vocabgen.db"]
    CFG --> YAML["~/.vocabgen/config.yaml"]
```

## Package Layout

| Package | Path | Description |
|---------|------|-------------|
| main | `cmd/vocabgen/` | Cobra CLI entry point, subcommands, flag parsing |
| config | `internal/config/` | YAML config manager (`LoadConfig`, `SaveConfig`) |
| db | `internal/db/` | SQLite schema, migrations, CRUD, cache layer |
| language | `internal/language/` | Prompt templates, JSON validation, language registry |
| llm | `internal/llm/` | Provider interface, Bedrock/OpenAI/Anthropic/VertexAI |
| output | `internal/output/` | Field mapping, translation flattening, Excel export |
| parsing | `internal/parsing/` | CSV reading, word/expression normalization |
| service | `internal/service/` | `Lookup`, `ProcessBatch` — shared business logic |
| web | `internal/web/` | HTTP handlers, routes, embedded HTML templates |

## Provider Interface

```go
type Provider interface {
    Invoke(ctx context.Context, prompt string, modelID string) (string, error)
    Name() string
}
```

Providers are registered in a `map[string]NewProviderFunc` registry. Adding a provider requires one file and one registry entry. The service layer depends only on the interface.

| Provider | File | Auth |
|----------|------|------|
| Bedrock | `bedrock.go` | AWS credential chain |
| OpenAI | `openai.go` | API key (or none with custom base URL) |
| Anthropic | `anthropic.go` | API key |
| Vertex AI | `vertexai.go` | Google ADC |

## Data Flow: Single Lookup

```mermaid
sequenceDiagram
    participant C as CLI / Web Handler
    participant S as service.Lookup()
    participant DB as db.Store
    participant L as language.BuildPrompt()
    participant V as language.Validate()
    participant P as llm.Provider
    participant O as output.MapFields()

    C->>S: Lookup(ctx, store, params)
    S->>S: Normalize token
    S->>S: Apply timeout to ctx via context.WithTimeout
    S->>DB: FindWords/FindExpressions(lang, text)
    alt No existing entries (cache miss)
        S->>L: BuildPrompt(lang, mode, token, context, targetLang)
        L-->>S: prompt string
        S->>P: Invoke(ctx, prompt, modelID)
        P-->>S: raw JSON string
        S->>V: ValidateResponse(mode, rawJSON)
        V-->>S: validated struct
        S->>O: MapFields(validated, mode)
        O-->>S: output struct
        S->>DB: InsertEntry(output, lang, targetLang)
        S-->>C: result
    else Existing entries found, no context sentence
        DB-->>S: []existing entries
        S-->>C: first cached entry
    else Existing entries found, context sentence provided (cache bypass)
        DB-->>S: []existing entries
        S->>L: BuildPrompt(lang, mode, token, context, targetLang)
        L-->>S: prompt string
        S->>P: Invoke(ctx, prompt, modelID)
        P-->>S: raw JSON string
        S->>V: ValidateResponse(mode, rawJSON)
        V-->>S: validated struct
        S->>O: MapFields(validated, mode)
        O-->>S: new entry
        S-->>C: LookupResult{New, Existing[], NeedsResolution: true}
        Note over C,S: Caller applies conflict resolution (replace/add/skip)
    end
```

## Data Flow: Batch Processing

```mermaid
sequenceDiagram
    participant C as CLI / Web Handler
    participant S as service.ProcessBatch()
    participant DB as db.Store
    participant P as llm.Provider

    C->>S: ProcessBatch(ctx, store, params)
    Note over S: params.OnConflict = "replace" | "add" | "skip"
    loop For each (token, context) in tokens
        S->>S: Normalize token
        S->>DB: FindWords/FindExpressions(lang, normalizedToken)
        alt No existing entries (cache miss)
            alt limit reached
                S->>S: break
            else within limit
                S->>S: BuildPrompt → Invoke → Validate → MapFields
                S->>DB: InsertEntry(result)
                S->>S: count as processed
            end
        else Existing entries found, no context sentence
            S->>S: count as cached, skip
        else Existing entries found, context sentence provided
            alt OnConflict = replace
                S->>DB: UpdateWord/UpdateExpression(firstExistingID, newResult)
            else OnConflict = add
                S->>DB: InsertEntry(newResult)
            else OnConflict = skip
                S->>S: count as skipped
            end
        end
    end
    S-->>C: BatchResult{Results, Errors, Processed, Cached, Failed, Skipped, Replaced, Added}
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| CLI framework | Cobra | Subcommands, auto-generated help, flag parsing |
| Web framework | stdlib `net/http` + HTMX | No JS build step, embedded in binary |
| Database | SQLite via `modernc.org/sqlite` | Pure-Go, cross-compiles, zero-config |
| LLM abstraction | Go interface + registry | Testable with mocks, easy to extend |
| Config format | YAML | Human-readable, `gopkg.in/yaml.v3` |
| PBT library | `pgregory.net/rapid` | Go-native, integrates with `testing` |
| Excel export | `excelize/v2` | Pure-Go xlsx writer |
| Logging | `log/slog` | Stdlib, structured, leveled |
