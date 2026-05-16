# Requirements Document: Go Vocabulary Generator

## Introduction

The Go Vocabulary Generator is a single-binary, multi-platform CLI and web application that automates the creation of structured, nuanced vocabulary lists (B2→C1 level) for language learners. It consolidates vocabulary generation, modular LLM providers, SQLite database storage, and an embedded web UI into one cohesive Go application.

The application uses two unified, language-agnostic prompt templates (words and expressions) parameterized by `{source_language}`, plus a dedicated sentence template for grammar checking and vocabulary extraction, with English JSON field names throughout. The LLM provides native POS labels, register labels, and grammatical categories for each language. Translations are connotation-aware, preserving register and tone via a Core Rule Block and Decision Rubric embedded in each prompt.

Key technical direction: Go with Cobra CLI, embedded web UI via `go:embed` + `net/http`, LLM provider interface (starting with Bedrock, designed for OpenAI/Anthropic/Ollama), SQLite for persistent storage, single binary distribution via cross-compilation, and table-driven tests with `rapid` (Go PBT library) for correctness properties.

## Glossary

- **App**: The Go Vocabulary Generator single-binary application
- **CLI**: The Cobra-based command-line interface
- **Web_UI**: The embedded web interface served via `go:embed` + `net/http`, using Go `html/template` and HTMX
- **Provider**: An LLM API backend that receives prompts and returns text responses (e.g., AWS Bedrock, OpenAI, Anthropic, Ollama)
- **Provider_Interface**: A Go interface defining the contract all provider implementations must satisfy (client creation, prompt invocation, provider name)
- **Bedrock_Provider**: Provider implementation for AWS Bedrock
- **OpenAI_Provider**: Provider implementation for the OpenAI API, also compatible with OpenAI-compatible local servers (Ollama, LM Studio) via custom base URL
- **Anthropic_Provider**: Provider implementation for the Anthropic Claude direct API
- **Vertex_AI_Provider**: Provider implementation for Google Vertex AI (Gemini, PaLM, Claude on Vertex)
- **Provider_Registry**: A map from provider name strings to provider constructor functions
- **Database**: SQLite file-based database stored at a configurable path (default: `~/.vocabgen/vocabgen.db`)
- **Word_Entry**: A row in the words table containing vocabulary data plus metadata (source_language, target_language, created_at, updated_at)
- **Expression_Entry**: A row in the expressions table containing vocabulary data plus metadata
- **Cache_Layer**: Logic that checks the Database for an existing entry before invoking the LLM provider, returning the cached result when found; bypasses the cache when a context sentence is provided for an existing entry
- **Conflict_Resolution_Strategy**: The user-selected action when a new LLM result conflicts with an existing database entry: "replace" (update existing), "add" (insert alongside), or "skip" (discard new result)
- **Input_File**: CSV file containing raw words or expressions in the source language
- **Prompt_Template**: A parameterized Go string constant sent to the LLM containing instructions for vocabulary generation, with `{source_language}`, `{word}`/`{expression}`, `{context}`, and `{target_language_name}` placeholders
- **Words_Template**: The unified prompt template for individual word lookups
- **Expressions_Template**: The unified prompt template for fixed expressions and idioms
- **Sentence_Template**: The dedicated prompt template for sentence lookups, focused on grammar checking, vocabulary extraction, and contextual analysis (not treating the input as a fixed expression)
- **English_Schema**: The set of English JSON field names used in LLM responses for words (`word`, `type`, `article`, `definition`, `english_definition`, `example`, `english`, `target_translation`, `notes`, `connotation`, `register`, `collocations`, `contrastive_notes`, `secondary_meanings`), expressions (`expression`, `definition`, `english_definition`, `example`, `english`, `target_translation`, `notes`, `connotation`, `register`, `contrastive_notes`), and sentences (`sentence`, `translation`, `grammar_check`, `vocabulary`)
- **English_Definition**: An optional field containing an explanation of the word or expression's meaning in English prose (not a translation, but a definition/explanation), supplementing the source-language `definition` field for learners who are not yet advanced enough to fully understand the source-language definition
- **Validator**: The function that validates LLM JSON responses against English schemas
- **Field_Mapper**: The function that maps validated JSON to output structs, flattening nested translations
- **Source_Language**: The language being learned, resolved from the supported languages registry or passed as free-text via CLI
- **Target_Language**: The user's native language for the second translation column
- **Supported_Languages**: Single shared registry (Go map) mapping language codes to full names, used for both source and target language selection
- **Core_Rule_Block**: Instruction section within prompts stating connotation/register/tone preservation principles
- **Decision_Rubric**: Prioritized criteria for choosing between candidate translations (connotation → register → comprehensibility)
- **Config_Manager**: Module that reads and writes a local YAML configuration file storing default settings
- **Config_File**: A YAML file (`~/.vocabgen/config.yaml`) that persists user preferences between sessions
- **CSV_Importer**: Component that reads CSV files and inserts rows into the Database
- **Excel_Exporter**: Component that writes Database query results to an .xlsx file for download
- **Database_Page**: A page in the Web_UI at `/database` for browsing, searching, editing, importing, and exporting vocabulary entries
- **Store**: Go interface defining all database operations, enabling test doubles without mocking frameworks

## Requirements
### Requirement 1: Unified Words Prompt Template

**User Story:** As a developer, I want a single words prompt template as a Go string constant parameterized by source language, so that adding new source languages requires zero template changes.

#### Acceptance Criteria

1. WHEN the Words_Template is defined, THE Words_Template SHALL be a Go string constant containing a `{source_language}` placeholder for the source language name (e.g., "Dutch", "Hungarian")
2. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to return JSON with English field names: `word`, `type`, `article`, `definition`, `english_definition`, `example`, `english`, `target_translation`, `notes`, `connotation`, `register`, `collocations`, `contrastive_notes`, `secondary_meanings`
3. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to use the source language's standard abbreviated POS label in the `type` field (e.g., "znw", "ww", "bn" for Dutch; "főnév", "ige" for Hungarian) without hardcoding them in the template; when no standard abbreviation exists for a language, the LLM SHALL use the shortest conventional form
4. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to use the source language's native register labels in the `register` field without hardcoding them in the template
5. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to provide the `definition` and `example` fields in the source language
6. WHEN the Words_Template is defined, THE Words_Template SHALL contain `{word}` and `{context}` placeholders for the input token and optional context sentence
7. WHEN the Words_Template is defined, THE Words_Template SHALL contain `{target_language_name}` placeholder for the target translation language name
8. WHEN the Words_Template is defined, THE Words_Template SHALL include the Core_Rule_Block and Decision_Rubric
9. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to return `english` and `target_translation` as objects with `primary` and `alternatives` keys (semicolon-separated alternatives)
10. WHEN the Words_Template is formatted with any source language name, THE Words_Template SHALL produce a valid prompt that generates correct vocabulary data for that language
11. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to populate the `connotation` field with a short description of the emotional or evaluative association of the source word
12. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to list two to four common collocations separated by semicolons in the `collocations` field
13. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to name one or two near-synonyms and briefly explain how they differ in the `contrastive_notes` field
14. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to list additional distinct meanings separated by semicolons in the `secondary_meanings` field, or leave empty when the word has only one meaning
15. WHEN the Words_Template is defined, THE Words_Template SHALL instruct the LLM to provide an `english_definition` field containing a concise English-language explanation of the word's meaning, distinct from the `english` translation field

#### Properties

- P1.1: Template formatting produces valid prompts for any source language name (Req 1, AC 10)
- P1.2: Formatted template contains no unresolved `{...}` placeholders (Req 1, AC 10)
- P1.3: Formatted template contains Core Rule Block and Decision Rubric text (Req 1, AC 8)

### Requirement 2: Unified Expressions Prompt Template

**User Story:** As a developer, I want a single expressions prompt template as a Go string constant parameterized by source language, so that expression processing works identically across all languages.

#### Acceptance Criteria

1. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL be a Go string constant containing a `{source_language}` placeholder for the source language name
2. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to return JSON with English field names: `expression`, `definition`, `english_definition`, `example`, `english`, `target_translation`, `notes`, `connotation`, `register`, `contrastive_notes`
3. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to use the source language's native register labels in the `register` field without hardcoding them in the template
4. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to provide the `definition` and `example` fields in the source language
5. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL contain `{expression}` and `{context}` placeholders for the input token and optional context sentence
6. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL contain `{target_language_name}` placeholder for the target translation language name
7. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL include the Core_Rule_Block and Decision_Rubric
8. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to return `english` and `target_translation` as objects with `primary` and `alternatives` keys (semicolon-separated alternatives)
9. WHEN the Expressions_Template is formatted with any source language name, THE Expressions_Template SHALL produce a valid prompt that generates correct vocabulary data for that language
10. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to populate the `connotation` field with a short description of the emotional or evaluative association
11. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to name one or two near-synonyms and briefly explain how they differ in the `contrastive_notes` field
12. WHEN the Expressions_Template is defined, THE Expressions_Template SHALL instruct the LLM to provide an `english_definition` field containing a concise English-language explanation of the expression's meaning, distinct from the `english` translation field

#### Properties

- P2.1: Template formatting produces valid prompts for any source language name (Req 2, AC 9)
- P2.2: Formatted template contains no unresolved `{...}` placeholders (Req 2, AC 9)
### Requirement 3: JSON Validation with English Schemas

**User Story:** As a developer, I want a single JSON validation function per mode using English field names, so that validation logic is language-agnostic.

#### Acceptance Criteria

1. WHEN the Validator receives a words JSON response, THE Validator SHALL validate against a single words schema with English field names: `word` (string, required), `type` (string, required), `article` (string, required), `definition` (string, required), `english_definition` (string, optional), `example` (string, required), `english` (string or object, required), `target_translation` (string or object, required), `notes` (string, optional), `connotation` (string, optional), `register` (string, optional), `collocations` (string, optional), `contrastive_notes` (string, optional), `secondary_meanings` (string, optional)
2. WHEN the Validator receives an expressions JSON response, THE Validator SHALL validate against a single expressions schema with English field names: `expression` (string, required), `definition` (string, required), `english_definition` (string, optional), `example` (string, required), `english` (string or object, required), `target_translation` (string or object, required), `notes` (string, optional), `connotation` (string, optional), `register` (string, optional), `contrastive_notes` (string, optional)
3. WHEN the Validator receives `english` or `target_translation` fields, THE Validator SHALL accept either a plain string or a JSON object with `primary` and `alternatives` keys
4. WHEN a plain string is provided for `english` or `target_translation`, THE Validator SHALL normalize it into an object with `primary` set to the string and `alternatives` set to empty string
5. WHEN an optional field is absent from the response, THE Validator SHALL default it to empty string
6. WHEN a required field is missing, THE Validator SHALL return an error listing the missing fields
7. WHEN an optional field is present but has a non-string value, THE Validator SHALL return an error
8. WHEN a translation field contains a nested object, THE Validator SHALL verify that `primary` is a string and `alternatives` is a string when present
9. IF a translation field contains neither a string nor a valid object with a string `primary` key, THEN THE Validator SHALL return an error identifying the malformed field
10. WHEN the Validator receives a sentence JSON response, THE Validator SHALL validate against the sentence schema with English field names: `sentence` (string, required), `translation` (string, required), `grammar_check` (object, required — containing `has_errors` boolean, `corrected_sentence` string, `errors` array of objects with `original`, `corrected`, `explanation` strings), `vocabulary` (array, required — each item with `word`, `type`, `definition`, `english` strings)

#### Properties

- P3.1: Translation field normalization produces consistent object format (Req 3, AC 3–4)
- P3.2: Optional fields default to empty string when absent (Req 3, AC 5)
- P3.3: Missing required fields return validation error listing all missing fields (Req 3, AC 6)
- P3.4: Any valid English-schema JSON passes validation without error (Req 3, AC 1–2)

### Requirement 4: Language Configuration

**User Story:** As a developer, I want a minimal language configuration using Go maps and constants, so that the codebase has no per-language duplication and new languages need only a registry entry.

#### Acceptance Criteria

1. WHEN the App is compiled, THE App SHALL contain exactly 3 prompt template constants: one for words, one for expressions, and one for sentences
2. WHEN the App is compiled, THE App SHALL contain exactly 3 JSON schema definitions (as Go structs or maps): one for words, one for expressions, and one for sentences
3. WHEN the App is compiled, THE App SHALL NOT contain per-language prompt template constants
4. WHEN a source language has no entry in Supported_Languages, THE prompt builder function SHALL still generate a valid prompt using the language name directly
5. WHEN the App is compiled, THE App SHALL contain a single Supported_Languages map (Go `map[string]string`) mapping language codes to full names, used for both source and target language resolution

### Requirement 5: Field Mapping and Translation Flattening

**User Story:** As a developer, I want a language-agnostic field mapper that passes through English fields and flattens nested translations, so that the service layer has no per-language branches.

#### Acceptance Criteria

1. WHEN the LLM returns JSON with English field names, THE Field_Mapper SHALL pass through the fields directly without language-specific mapping
2. WHEN the Field_Mapper receives nested translation objects (`english` and `target_translation`), THE Field_Mapper SHALL flatten them from `{primary} (alternatives)` format to a single string for output
3. WHEN the Field_Mapper processes any input, THE Field_Mapper SHALL NOT contain separate code branches for individual languages
4. WHEN the Field_Mapper processes any source language, THE Field_Mapper SHALL work identically without code changes
5. WHEN the Field_Mapper receives both flat string and nested object translation fields, THE Field_Mapper SHALL handle both without errors

#### Properties

- P5.1: Field mapper pass-through preserves non-translation fields (Req 5, AC 1)
- P5.2: Translation flattening produces correct format for objects and strings (Req 5, AC 2, 5)
### Requirement 6: Prompt Builder Function

**User Story:** As a developer, I want a `BuildPrompt` function that accepts a source language name or code and injects all parameters into the template, so that any language can be used without pre-registration.

#### Acceptance Criteria

1. WHEN a source language name or code is provided, THE `BuildPrompt` function SHALL accept it as a parameter
2. WHEN a known language code is provided (nl, hu, it, ru, etc.), THE `BuildPrompt` function SHALL resolve the code to the full language name
3. WHEN an unknown language code or name is provided, THE `BuildPrompt` function SHALL use the provided value directly as the source language name in the prompt
4. WHEN a mode parameter is provided, THE `BuildPrompt` function SHALL select the words, expressions, or sentence template based on the mode parameter
5. WHEN all parameters are provided, THE `BuildPrompt` function SHALL inject `source_language`, `word`/`expression`, `context`, and `target_language_name` into the template
6. WHEN an optional `context` parameter is provided, THE `BuildPrompt` function SHALL inject it into the prompt template
7. WHEN an invalid mode value is provided, THE `BuildPrompt` function SHALL return an error

#### Properties

- P6.1: BuildPrompt injects all parameters into output for any source language, mode, token, context, and target language (Req 6, AC 1–6)
- P6.2: BuildPrompt returns error for invalid mode values (Req 6, AC 7)

### Requirement 7: LLM Provider Interface

**User Story:** As a developer, I want a common Go interface for LLM providers, so that I can add new providers without modifying calling code.

#### Acceptance Criteria

1. WHEN a provider is invoked, THE Provider_Interface SHALL define a method to invoke the provider with a `context.Context`, a prompt string, and a model ID, returning a response string and an error
2. WHEN a provider name is requested, THE Provider_Interface SHALL define a method to return the provider name as a string identifier
3. WHEN a provider receives a successful response, THE provider SHALL extract the text content and return it as a string, stripping any provider-specific envelope
4. IF a provider returns an empty or missing text response, THEN THE provider SHALL return a descriptive error
5. WHEN a new provider struct implements the Provider_Interface, THE Provider_Interface SHALL require no changes to service layer call sites
6. WHEN a provider is constructed, THE provider SHALL be constructed via a dedicated constructor function (e.g., `NewBedrockProvider(...)`) that returns a concrete struct satisfying the Provider_Interface, following the Go convention of "accept interfaces, return structs"

#### Properties

- P7.1: Provider interface consistency — invoking with same prompt and model returns string or error, never both nil; name returns non-empty string (Req 7, AC 1–2, 4)

### Requirement 8: Bedrock Provider Implementation

**User Story:** As a user, I want the existing Bedrock functionality as a provider implementation, so that current workflows work from day one.

#### Acceptance Criteria

1. WHEN the Bedrock_Provider is used, THE Bedrock_Provider SHALL implement the Provider_Interface
2. WHEN the Bedrock_Provider is configured, THE Bedrock_Provider SHALL accept AWS credentials and region for client creation
3. WHEN the Bedrock_Provider is invoked, THE Bedrock_Provider SHALL support Claude and Cohere model families via the Bedrock Runtime API
4. WHEN the Bedrock_Provider encounters throttling or timeout errors, THE Bedrock_Provider SHALL retry up to a configurable maximum
5. WHEN the Bedrock_Provider is selected, THE App SHALL require AWS authentication (profile or default credential chain)
6. WHEN the Bedrock_Provider creates a client, THE Bedrock_Provider SHALL validate that the specified region supports Bedrock before creating a client

### Requirement 9: OpenAI Provider Implementation

**User Story:** As a user, I want to use OpenAI/ChatGPT models or OpenAI-compatible local servers (Ollama, LM Studio), so that I can choose a different LLM backend including self-hosted models.

#### Acceptance Criteria

1. WHEN the OpenAI_Provider is used, THE OpenAI_Provider SHALL implement the Provider_Interface
2. WHEN the OpenAI_Provider authenticates, THE OpenAI_Provider SHALL use an API key sourced from an environment variable or CLI flag
3. WHEN no API key is available and no custom base URL is set, THE OpenAI_Provider SHALL return a descriptive authentication error before attempting invocation
4. WHEN a model is specified, THE OpenAI_Provider SHALL support specifying an OpenAI model name (e.g., "gpt-4o", "gpt-4o-mini") via the `--model-id` flag
5. WHEN the OpenAI_Provider encounters rate-limit errors, THE OpenAI_Provider SHALL retry up to a configurable maximum
6. WHEN a base URL is configured, THE OpenAI_Provider SHALL accept an optional base URL via the `--base-url` CLI flag or the `OPENAI_BASE_URL` environment variable to target OpenAI-compatible servers
7. WHEN a custom base URL is provided, THE OpenAI_Provider SHALL use that URL instead of the default OpenAI API endpoint
8. WHEN a custom base URL is provided and no API key is set, THE OpenAI_Provider SHALL allow invocation without an API key
9. WHEN configured with a compatible endpoint, THE OpenAI_Provider SHALL be compatible with Azure OpenAI Service, Ollama (`http://localhost:11434/v1`), LM Studio, vLLM, and any other server implementing the OpenAI chat completions API

### Requirement 10: Anthropic Provider Implementation

**User Story:** As a user, I want to use the Anthropic Claude API directly, so that I can access Claude models without going through AWS Bedrock.

#### Acceptance Criteria

1. WHEN the Anthropic_Provider is used, THE Anthropic_Provider SHALL implement the Provider_Interface
2. WHEN the Anthropic_Provider authenticates, THE Anthropic_Provider SHALL use an API key sourced from an environment variable or CLI flag
3. WHEN no API key is available, THE Anthropic_Provider SHALL return a descriptive authentication error before attempting invocation
4. WHEN a model is specified, THE Anthropic_Provider SHALL support specifying an Anthropic model name (e.g., "claude-sonnet-4-20250514") via the `--model-id` flag
5. WHEN the Anthropic_Provider encounters rate-limit errors, THE Anthropic_Provider SHALL retry up to a configurable maximum
### Requirement 11: Provider Selection and Registry

**User Story:** As a user, I want to select the LLM provider at runtime via a CLI flag, so that I can switch providers without changing code.

#### Acceptance Criteria

1. WHEN the CLI is invoked, THE CLI SHALL accept a `--provider` flag with valid values including "bedrock", "openai", "anthropic", and "vertexai"
2. WHEN no `--provider` flag is specified, THE CLI SHALL default to "bedrock"
3. WHEN an unsupported provider name is specified, THE CLI SHALL display an error listing valid provider names and exit with a non-zero code
4. WHEN the Provider_Registry is queried, THE Provider_Registry SHALL map string identifiers to their corresponding provider constructor functions
5. WHEN a new provider is added, THE Provider_Registry SHALL require only adding a new entry to the map and the provider implementation file

### Requirement 12: Provider-Specific Authentication

**User Story:** As a user, I want each provider to handle its own authentication method, so that I only need to configure credentials for the provider I am using.

#### Acceptance Criteria

1. WHEN the "bedrock" provider is selected, THE App SHALL use the AWS authentication flow (profile or default credential chain)
2. WHEN the "openai" provider is selected, THE App SHALL accept an API key via the `--api-key` flag or the `OPENAI_API_KEY` environment variable
3. WHEN the "anthropic" provider is selected, THE App SHALL accept an API key via the `--api-key` flag or the `ANTHROPIC_API_KEY` environment variable
4. IF both `--api-key` and the corresponding environment variable are set, THEN THE App SHALL prefer the `--api-key` flag value
5. WHEN the "bedrock" provider is selected, THE App SHALL ignore the `--api-key` flag
6. WHEN the "openai" or "anthropic" provider is selected, THE App SHALL ignore the `--profile` flag
7. WHEN the "vertexai" provider is selected, THE App SHALL use Google Application Default Credentials and accept a project ID via `--gcp-project` or `GCP_PROJECT` environment variable

### Requirement 13: Error Handling Across Providers

**User Story:** As a user, I want clear error messages regardless of which provider fails, so that I can diagnose and fix issues quickly.

#### Acceptance Criteria

1. WHEN a provider invocation fails, THE provider SHALL return an error that includes the provider name and a descriptive message
2. WHEN authentication fails for a provider, THE provider SHALL return an error that specifies which credential is missing or invalid
3. IF a provider encounters a rate-limit error and retries are exhausted, THEN THE provider SHALL return an error indicating throttling and the number of retries attempted
4. WHEN errors are returned by any provider, THE error types SHALL be wrappable by a single sentinel error type in calling code

### Requirement 14: Parse Input CSV Files

**User Story:** As a language learner, I want the app to parse raw word lists from CSV files, so that I can process vocabulary from my textbook chapters.

#### Acceptance Criteria

1. WHEN an input file is provided, THE App SHALL read the file using UTF-8 encoding
2. WHEN an input line is empty or whitespace-only, THE App SHALL skip the line
3. WHEN an input file does not exist, THE App SHALL exit with a non-zero exit code and error message
4. WHEN an input file is empty after skipping blank lines, THE App SHALL exit with an error message
5. WHEN an input CSV has two columns, THE App SHALL treat the second column as the context sentence for each word or expression
6. WHEN an input CSV has a single column, THE App SHALL work without errors
7. WHEN an input CSV is parsed, THE App SHALL treat all non-empty lines as data rows (no header detection or skipping)

#### Properties

- P14.1: CSV parsing returns exactly the non-empty lines as (token, context) pairs; two-column lines return both, single-column lines return token with empty context (Req 14, AC 2, 5–7)

### Requirement 15: Normalize Word Tokens

**User Story:** As a language learner, I want the app to extract canonical word forms, so that my vocabulary list contains clean lemmas.

#### Acceptance Criteria

1. WHEN a word token contains quotes, THE App SHALL remove the quotes
2. WHEN a word token contains multiple spaces, THE App SHALL normalize to single spaces
3. WHEN a word token contains parentheses with inflection info (no commas), THE App SHALL preserve the parenthetical content (e.g., `(ergens)`, `(zich)`)
4. WHEN a word token is processed, THE App SHALL strip leading and trailing whitespace
5. WHEN a word token contains digits or special characters (anything other than letters, spaces, hyphens, apostrophes, or parentheses), THE App SHALL reject the token with an error before invoking the LLM provider
6. WHEN a word token contains vocabulary-list markers (`*` prefix/suffix, `>` prefix, `(sep.)` annotation), THE App SHALL strip these markers before processing
7. WHEN a word token contains conjugation annotations (parenthetical groups with commas, e.g., `(kwam uit, is uitgekomen)`), THE App SHALL strip these annotations before processing
8. WHEN a word token starts with a Dutch article (`de`, `het`, `een`), THE App SHALL strip the article since it is stored in a separate database field

#### Properties

- P15.1: Token normalization is idempotent — normalizing an already-normalized token produces the same result (Req 15, AC 1–4, 6–8)
- P15.2: Words containing digits or special characters are rejected before LLM invocation (Req 15, AC 5)
- P15.3: Vocabulary-list markers and leading articles are stripped from word tokens (Req 15, AC 6–8)

### Requirement 16: Normalize Expression Tokens

**User Story:** As a language learner, I want the app to clean expression text, so that my vocabulary list contains properly formatted expressions.

#### Acceptance Criteria

1. WHEN an expression token contains quotes, THE App SHALL remove the quotes
2. WHEN an expression token contains multiple spaces, THE App SHALL normalize to single spaces
3. WHEN an expression token is empty after normalization, THE App SHALL skip the token
4. WHEN an expression token contains vocabulary-list markers (`*` prefix/suffix, `>` prefix, `(sep.)` annotation), THE App SHALL strip these markers before processing
5. WHEN an expression token contains conjugation annotations (parenthetical groups with commas), THE App SHALL strip these annotations before processing

#### Properties

- P16.1: Token normalization is idempotent — normalizing an already-normalized expression produces the same result (Req 16, AC 1–2, 4–5)
- P16.2: Vocabulary-list markers are stripped from expression tokens (Req 16, AC 4–5)
### Requirement 17: SQLite Database Schema and Initialization

**User Story:** As a developer, I want a structured SQLite database with separate tables for words and expressions, so that I can store and query vocabulary data efficiently.

#### Acceptance Criteria

1. WHEN the application starts, THE Database SHALL create the SQLite file and tables if they do not already exist
2. WHEN storing word entries, THE Database SHALL store Word_Entry rows in a `words` table with columns: id (INTEGER PRIMARY KEY AUTOINCREMENT), word (TEXT NOT NULL), part_of_speech (TEXT), article (TEXT), definition (TEXT), english_definition (TEXT), example (TEXT), english (TEXT), target_translation (TEXT), notes (TEXT), connotation (TEXT), register (TEXT), collocations (TEXT), contrastive_notes (TEXT), secondary_meanings (TEXT), tags (TEXT), source_language (TEXT NOT NULL), target_language (TEXT NOT NULL), created_at (TEXT NOT NULL), updated_at (TEXT NOT NULL)
3. WHEN storing expression entries, THE Database SHALL store Expression_Entry rows in an `expressions` table with columns: id (INTEGER PRIMARY KEY AUTOINCREMENT), expression (TEXT NOT NULL), definition (TEXT), english_definition (TEXT), example (TEXT), english (TEXT), target_translation (TEXT), notes (TEXT), connotation (TEXT), register (TEXT), contrastive_notes (TEXT), tags (TEXT), source_language (TEXT NOT NULL), target_language (TEXT NOT NULL), created_at (TEXT NOT NULL), updated_at (TEXT NOT NULL)
4. WHEN the Database is initialized, THE Database SHALL create an index on (source_language, word) in the words table and (source_language, expression) in the expressions table
5. WHEN the Database path is resolved, THE Database SHALL use the path configured in Config_Manager, defaulting to `~/.vocabgen/vocabgen.db`

### Requirement 18: Cache Layer for LLM Lookups

**User Story:** As a user, I want the system to check the database before calling the LLM provider, so that I save time and API costs on words already processed.

#### Acceptance Criteria

1. WHEN a lookup request is received and no context sentence is provided, THE Cache_Layer SHALL query the Database for a matching entry (by word/expression text and source_language) before invoking the LLM provider
2. WHEN a matching entry exists in the Database and no context sentence is provided, THE Cache_Layer SHALL return the cached result without calling the LLM provider
3. WHEN no matching entry exists in the Database, THE Cache_Layer SHALL invoke the LLM provider, store the result in the Database, and return the result
4. WHEN a batch process runs, THE Cache_Layer SHALL check each token against the Database before invoking the LLM provider for that token
5. WHEN a lookup is served, THE Cache_Layer SHALL log whether each lookup was served from cache or from the LLM provider
6. WHEN a lookup request is received with a context sentence and a matching entry already exists in the Database, THE Cache_Layer SHALL invoke the LLM provider (bypassing the cache) and return the new result alongside the existing entry metadata, so the user can decide how to handle the conflict
7. WHEN multiple entries exist for the same word/expression and source_language, THE Cache_Layer `FindWords`/`FindExpressions` functions SHALL return all matching entries as a slice

#### Properties

- P18.1: Database cache idempotency — looking up a token twice results in one LLM invocation and one cache hit with identical data (Req 18, AC 1–4)

### Requirement 19: LLM Results Integration with Database

**User Story:** As a user, I want LLM lookup results to be automatically saved to the database, so that my vocabulary collection grows with each query.

#### Acceptance Criteria

1. WHEN a single lookup returns a result from the LLM provider and no existing entry conflicts, THE App SHALL insert the result into the Database
2. WHEN a batch process completes, THE App SHALL insert all successful results into the Database
3. WHEN an entry is inserted, THE App SHALL record the source_language and target_language for each inserted entry
4. WHEN an entry is inserted, THE App SHALL set created_at and updated_at timestamps on each inserted entry
5. WHEN a lookup returns a result and an existing entry for the same word/expression and source_language exists, THE App SHALL apply the user-selected conflict resolution strategy (replace, add, or skip) before persisting
6. WHEN the "replace" strategy is selected, THE App SHALL update the existing entry in-place using `UpdateWord`/`UpdateExpression` by ID and set the updated_at timestamp
7. WHEN the "add" strategy is selected, THE App SHALL insert the new result as a separate entry alongside the existing one, allowing multiple versions of the same word/expression

### Requirement 20: Service Layer

**User Story:** As a developer, I want business logic separated from the CLI and HTTP layers, so that the same logic can be called from the CLI, the web UI, and tests.

#### Acceptance Criteria

1. WHEN a lookup is requested, THE App SHALL expose a `Lookup` function that accepts a `context.Context`, a source language code, a lookup type (word, expression, or sentence), the input text, a provider instance, a model ID, an optional context sentence, a target language, and optional tags, and returns a validated and mapped vocabulary struct
2. WHEN a batch is requested, THE App SHALL expose a `ProcessBatch` function that accepts a `context.Context`, a source language code, a mode (words or expressions), a list of raw tokens with contexts, a provider instance, a model ID, a target language, and optional tags, and returns a list of result structs and a list of error tuples
3. WHEN supported languages are requested, THE App SHALL expose a `GetSupportedLanguages` function that returns the list of supported language codes and names
4. WHEN the `Lookup` function is called, THE App SHALL perform language resolution, prompt building, LLM invocation, JSON validation, and field mapping without depending on Cobra or net/http
5. IF the LLM invocation fails during a service call, THEN THE App SHALL return a typed error with the failure details
6. IF JSON validation fails during a service call, THEN THE App SHALL return a typed validation error with the details
7. WHEN the LLM returns a response for a word lookup, THE App SHALL check if the LLM recognized the input as a non-word (type="—", definition contains "not a valid"/"geen geldig", or example="—") and if so, return a warning and skip the database insert
8. WHEN the LLM returns a response for a word lookup, THE App SHALL check if the input token appears in the example sentence and if not, return a hallucination warning (the entry is still saved but flagged)
9. WHEN the LLM returns a response for an expression lookup, THE App SHALL skip the hallucination check (expressions are naturally conjugated/modified in example sentences) but still perform the non-word check
10. WHEN the `Lookup` function is called with `LookupType` "sentence", THE App SHALL skip the cache check, invoke the LLM with the sentence template, validate with `ValidateSentenceResponse`, and return the result without writing to the Database

### Requirement 21: Cobra CLI Interface

**User Story:** As a language learner, I want to control app behavior via command-line arguments, so that I can process different chapters and modes.

#### Acceptance Criteria

1. WHEN the App is built, THE App SHALL use Cobra for CLI argument parsing
2. WHEN the CLI is invoked, THE CLI SHALL accept a required `--source-language` / `-l` flag accepting any string (known codes resolved to full names)
3. WHEN the CLI is invoked, THE CLI SHALL accept a `--mode` flag with values "words" or "expressions" (not required in lookup subcommand)
4. WHEN the CLI is invoked, THE CLI SHALL accept a `--input-file` flag specifying the input CSV path (not required in lookup subcommand)
5. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--model-id` flag specifying the LLM model ID
6. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--limit` flag to process only the first N items
7. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--verbose` / `-v` flag to enable debug logging
8. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--dry-run` flag to preview processing without LLM calls
9. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--provider` flag (default: "bedrock")
10. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--api-key` flag for OpenAI/Anthropic authentication
11. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--base-url` flag for OpenAI-compatible server endpoints
12. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--profile` flag for AWS profile authentication
13. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--region` / `-r` flag (default: "us-east-1")
14. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--target-language` flag (default: "hu")
15. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--context` flag for context sentence in lookup mode
16. WHEN the CLI is invoked, THE CLI SHALL provide a `lookup` subcommand for quick single-item lookups (word, expression, or sentence)
17. WHEN the CLI is invoked, THE CLI SHALL provide a `batch` subcommand for batch CSV processing
18. WHEN the CLI is invoked, THE CLI SHALL provide a `serve` subcommand to start the embedded web server
19. WHEN the `serve` subcommand is invoked, THE CLI SHALL accept an optional `--port` flag (default: 8080)
20. WHEN required arguments are missing, THE CLI SHALL print usage help and exit with non-zero code
21. WHEN `--help` is provided, THE CLI SHALL show all available arguments
22. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--tags` flag containing a comma-separated list of tags to apply to entries created during the session
23. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--timeout` flag specifying the LLM request timeout in seconds (default: 60)
24. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--gcp-project` flag for Vertex AI project ID
25. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--gcp-region` flag for Vertex AI region (default: "us-central1")
26. WHEN the CLI is invoked, THE CLI SHALL provide a `backup` subcommand for database backup
27. WHEN the CLI is invoked, THE CLI SHALL provide a `restore` subcommand that accepts a backup file path for database restore
28. WHEN the CLI is invoked, THE CLI SHALL provide a `version` subcommand that prints version information
29. WHEN `--version` is provided on the root command, THE CLI SHALL print the version and exit

### Requirement 22: Quick Lookup Mode

**User Story:** As a language learner, I want to quickly look up a single word, expression, or sentence without writing to CSV, so that I can test the LLM output or check vocabulary on the fly.

#### Acceptance Criteria

1. WHEN the `lookup` subcommand is invoked, THE `lookup` subcommand SHALL accept a positional argument for the text to look up
2. WHEN the `lookup` subcommand is invoked, THE `lookup` subcommand SHALL accept a `--type` flag with values "word", "expression", or "sentence" (default: "word")
3. WHEN the `lookup` subcommand is invoked, THE App SHALL invoke the LLM provider with the provided text
4. WHEN the `lookup` subcommand completes, THE App SHALL print the JSON response to console in formatted output
5. WHEN the `lookup` subcommand completes and no existing entry exists, THE App SHALL save the result to the Database
6. WHEN the `lookup` subcommand is invoked, THE `lookup` subcommand SHALL accept an optional `--context` flag for disambiguation
7. WHEN the `lookup` subcommand is invoked with a `--context` flag and an existing entry is found, THE App SHALL invoke the LLM provider with the context, display both the existing and new results, and prompt the user to choose a conflict resolution strategy (replace, add, or skip)
8. WHEN the `lookup` subcommand is invoked with an `--on-conflict` flag, THE App SHALL apply the specified strategy without interactive prompting

### Requirement 23: Batch Processing Mode

**User Story:** As a language learner, I want to process entire CSV files of vocabulary, so that I can build comprehensive vocabulary lists from textbook chapters.

#### Acceptance Criteria

1. WHEN the `batch` subcommand is invoked, THE `batch` subcommand SHALL accept `--input-file` and `--mode` as required flags
2. WHEN processing a batch, THE App SHALL check the Database cache for each token before invoking the LLM provider
3. WHEN a token exists in the Database cache, THE App SHALL skip LLM invocation for that token
4. WHEN skipping a token, THE App SHALL log a skip message
5. WHEN processing completes, THE App SHALL print a summary with counts of processed, cached, and failed items
6. WHEN `--limit N` is provided, THE App SHALL process at most N new items (excluding cached items)

#### Properties

- P23.1: Limit enforcement — for limit N and M tokens (M > N), processing invokes the LLM for at most N tokens excluding cached items (Req 23, AC 6)
### Requirement 24: Embedded Web UI Server

**User Story:** As a language learner, I want to start a web server from the binary and use a browser-based interface, so that I can do lookups and batch processing without the terminal.

#### Acceptance Criteria

1. WHEN the `serve` subcommand is invoked, THE App SHALL start an HTTP server on a configurable port (default: 8080)
2. WHEN the App is built, THE App SHALL embed HTML templates and static assets using Go's `go:embed` directive
3. WHEN the Web_UI renders pages, THE Web_UI SHALL use Go `html/template` for server-side rendering and HTMX for dynamic interactions
4. WHEN the Web_UI renders pages, THE Web_UI SHALL use Tailwind CSS (via CDN or embedded) for styling
5. WHEN the App is run, THE App SHALL serve all web UI assets from the single binary with no external file dependencies
6. WHEN the `serve` subcommand receives OS interrupt signals (SIGINT, SIGTERM), THE App SHALL perform graceful HTTP server shutdown using `context.Context`, allowing in-flight requests to complete before exiting

### Requirement 25: Web UI — Lookup Page

**User Story:** As a language learner, I want a web page where I can type a word, expression, or sentence and see the vocabulary entry rendered in a clean layout.

#### Acceptance Criteria

1. WHEN the Web_UI serves the lookup page, THE Web_UI SHALL serve it at `/` with a form containing a text input, source language selector, target language selector, lookup type selector (word, expression, sentence), and optional tags input
2. WHEN the user submits the lookup form, THE Web_UI SHALL use HTMX to POST to the API endpoint and swap the result into the page without a full reload
3. WHEN the lookup request is in progress, THE Web_UI SHALL display a loading spinner indicator
4. WHEN the API returns an error, THE Web_UI SHALL display the error message inline
5. WHEN the language selectors are rendered, THE Web_UI SHALL populate them from the Supported_Languages registry

### Requirement 26: Web UI — Batch Page

**User Story:** As a language learner, I want to upload a CSV file through the web interface and see the processing results.

#### Acceptance Criteria

1. WHEN the Web_UI serves the batch page, THE Web_UI SHALL serve it at `/batch` with a CSV file upload form, source language selector, target language selector, mode selector (words or expressions), and optional tags input
2. WHEN the user submits the batch form, THE Web_UI SHALL POST the file to the batch API endpoint
3. WHEN the batch API processes items, THE batch API endpoint SHALL use Server-Sent Events (SSE) to stream progress updates to the Web_UI as each item is processed
4. WHEN batch processing is in progress, THE Web_UI SHALL display a progress bar showing the number of processed items out of the total, and the current token being processed
5. WHEN batch processing completes, THE Web_UI SHALL display counts of processed, cached, and failed items
6. WHEN errors occur for specific items, THE Web_UI SHALL display the failed items list
7. WHEN batch processing is in progress, THE Web_UI SHALL display a cancel button that stops processing remaining items

### Requirement 27: Web UI — Config Page

**User Story:** As a language learner, I want a settings page in the web interface to configure my LLM provider, credentials, and preferences.

#### Acceptance Criteria

1. WHEN the Web_UI serves the config page, THE Web_UI SHALL serve it at `/config` with form fields for provider (dropdown: bedrock, openai, anthropic, vertexai), aws_profile (text input, shown for bedrock), aws_region (text input, shown for bedrock), gcp_project (text input, shown for vertexai), gcp_region (text input, shown for vertexai), model_id (text input), base_url (text input, shown for openai), default_source_language (dropdown), and default_target_language (dropdown); each provider section SHALL display an informational note about which environment variable or credential source is required
2. WHEN the config page loads, THE Web_UI SHALL populate the form fields from the current configuration
3. WHEN the user saves settings, THE Web_UI SHALL validate that the required environment variables or credentials are available for the selected provider and display a clear error message if missing; IF validation passes, THE Web_UI SHALL update the Config_File and display a success message inline
4. WHEN the user clicks "Test Connection", THE Config page SHALL test the current provider configuration using API keys resolved from environment variables and display the result inline
5. WHEN the config page renders, THE Config page SHALL display the current database path as a read-only field

### Requirement 28: Web UI — Database Page

**User Story:** As a user, I want a dedicated database page in the web UI to browse, search, and manage my vocabulary entries.

#### Acceptance Criteria

1. WHEN the Web_UI serves the database page, THE Database_Page SHALL be accessible at the `/database` URL
2. WHEN the Web_UI renders navigation, THE Database_Page SHALL display a navigation link in the shared nav bar alongside Lookup, Batch, and Config
3. WHEN the Database_Page renders entries, THE Database_Page SHALL display vocabulary entries in a paginated table with 50 rows per page
4. WHEN the Database_Page renders filters, THE Database_Page SHALL provide filter controls for source_language, target_language, entry type (words or expressions), and tags
5. WHEN a user types in the search field, THE Database_Page SHALL filter entries by matching against the word/expression, definition, english, and tags columns with a debounce delay of 300 milliseconds
6. WHEN entries are displayed, THE Database_Page SHALL display the total count of matching entries
7. WHEN a user clicks an entry row, THE Database_Page SHALL display an edit form with all fields pre-populated
8. WHEN a user submits the edit form, THE App SHALL update the corresponding row in the Database and set the updated_at timestamp
9. WHEN a user clicks a delete button on an entry, THE Database_Page SHALL ask for confirmation before deleting
10. WHEN the user confirms deletion, THE App SHALL remove the entry from the Database
### Requirement 29: Import into Database

**User Story:** As a user, I want to import my existing vocabulary files into the database, so that I can consolidate scattered vocabulary data.

#### Acceptance Criteria

1. WHEN the Database_Page renders import controls, THE Database_Page SHALL provide an import form with file upload (accepting .csv and .xlsx), source_language selector, target_language selector, and entry type selector (words or expressions)
2. WHEN a CSV file is uploaded, THE Importer SHALL validate UTF-8 encoding and reject non-UTF-8 files with a descriptive error message suggesting XLSX import instead
3. WHEN an XLSX file is uploaded, THE Importer SHALL read the "Words" or "Expressions" sheet by name (falling back to the first sheet) and parse rows with header detection
4. WHEN the import file contains a header row with recognized column names (word, expression, definition, english, type), THE Importer SHALL map columns by header name
5. WHEN a row matches an existing entry (same word/expression and source_language), THE Importer SHALL skip that row and count it as a duplicate
6. WHEN the import completes, THE Database_Page SHALL display a summary showing the count of imported, skipped, and failed rows
7. IF a row has missing required fields (word/expression), THEN THE Importer SHALL skip that row and include it in the failed count
8. WHEN an import file contains a tags column, THE Importer SHALL support the optional tags column and apply the tags to imported entries
9. WHEN a row contains encoding garbage (U+FFFD replacement characters), THE Importer SHALL skip that row and report the count of skipped encoding errors

### Requirement 30: Excel Export from Database

**User Story:** As a user, I want to export database entries to an Excel file, so that I can use the data in spreadsheet tools or Anki.

#### Acceptance Criteria

1. WHEN the Database_Page renders export controls, THE Database_Page SHALL provide an export button that downloads an .xlsx file
2. WHEN the export is triggered, THE Excel_Exporter SHALL apply the current filter and search criteria to determine which entries to export
3. WHEN the export generates a file, THE Excel_Exporter SHALL include column headers matching the database field names
4. WHEN the export generates a file, THE Excel_Exporter SHALL name the downloaded file using the pattern `vocabgen-{source_language}-{date}.xlsx`
5. WHEN the export generates a file, THE Excel_Exporter SHALL write words to a "Words" sheet and expressions to an "Expressions" sheet in the same file
6. WHEN no entries match the current filters, THE Database_Page SHALL disable the export button

### Requirement 31: REST API Endpoints

**User Story:** As a developer, I want HTTP endpoints backing the web UI, so that the HTMX frontend can interact with the service layer.

#### Acceptance Criteria

1. WHEN a POST request is sent to `/api/lookup` with a JSON body containing `source_language`, `lookup_type`, `text`, and optional `context`, `target_language`, and `tags`, THE App SHALL return a JSON response with the vocabulary entry
2. WHEN a POST request is sent to `/api/batch` with a multipart form containing a CSV file, source_language, target_language, mode, and optional tags, THE App SHALL parse the CSV, process the batch, and return a JSON response with results and errors
3. WHEN a GET request is sent to `/api/config`, THE App SHALL return the current configuration as JSON
4. WHEN a PUT request is sent to `/api/config` with a JSON body, THE App SHALL update the Config_File and return the updated configuration
5. WHEN a GET request is sent to `/api/languages`, THE App SHALL return the list of supported language codes and names
6. WHEN a GET request is sent to `/api/health`, THE App SHALL return HTTP 200 with `{"status": "ok"}`
7. WHEN a POST request is sent to `/api/test-connection`, THE App SHALL attempt to create an authenticated provider client using API keys resolved from environment variables and return the result
8. WHEN a POST request is sent to `/api/batch` with a file exceeding 10 MB, THE App SHALL reject the upload with HTTP 413 and a descriptive error message

### Requirement 32: Actionable API Error Messages

**User Story:** As a user of the web UI, I want error messages that tell me what went wrong and what to do next, so that I can resolve issues without checking server logs.

#### Acceptance Criteria

1. WHEN the API returns an error response (HTTP 400, 502, or 500), THE App SHALL include a JSON body with a `detail` field containing a human-readable, actionable error message
2. IF authentication fails during an API request, THEN THE App SHALL return an error message that identifies the failure reason and suggests a corrective action
3. IF an LLM invocation fails due to throttling, THEN THE App SHALL return an error message suggesting the user retry after a short wait
4. IF JSON validation of an LLM response fails, THEN THE App SHALL return an error message identifying the token that failed

### Requirement 33: Config Manager

**User Story:** As a user, I want my provider, credentials, region, and model preferences saved locally, so that I do not need to specify them as CLI flags every session.

#### Acceptance Criteria

1. WHEN the Config_Manager reads configuration, THE Config_Manager SHALL read from a YAML file at `~/.vocabgen/config.yaml`
2. WHEN the Config_Manager loads configuration, THE Config_Manager SHALL expose a `LoadConfig` function that returns a configuration struct with fields: provider, aws_profile, aws_region, model_id, base_url, gcp_project, gcp_region, default_source_language, default_target_language, db_path
3. WHEN the Config_Manager saves configuration, THE Config_Manager SHALL expose a `SaveConfig` function that accepts a configuration struct and writes it to the Config_File
4. WHEN the Config_File does not exist, THE `LoadConfig` function SHALL return a struct with default values (provider: "bedrock", aws_region: "us-east-1", default_source_language: "nl", default_target_language: "hu", db_path: "~/.vocabgen/vocabgen.db")
5. WHEN the `SaveConfig` function is called, THE Config_Manager SHALL create the `~/.vocabgen/` directory if it does not exist
6. WHEN the Config_File is written, THE Config_File SHALL contain only non-sensitive settings; THE Config_Manager SHALL NOT store API keys or AWS secrets in the YAML file
7. WHEN the Config_Manager serializes configuration, THE Config_Manager SHALL NOT serialize the `api_key` field to the YAML file; API keys are runtime-only values sourced from environment variables or CLI flags
8. WHEN the CLI is invoked without explicit flags, THE CLI SHALL load defaults from the Config_Manager
9. WHEN the CLI is invoked with explicit flags, THE CLI SHALL use the provided flag values and ignore the Config_File values for those flags

### Requirement 34: Config File Round-Trip Integrity

**User Story:** As a developer, I want to be confident that saving and loading configuration produces identical data, so that user settings are never silently corrupted.

#### Acceptance Criteria

1. FOR ALL valid configuration structs, saving via `SaveConfig` then loading via `LoadConfig` SHALL produce a struct equal to the original
2. WHEN `SaveConfig` writes a file, THE `SaveConfig` function SHALL produce a valid YAML file that the `LoadConfig` function can parse without error

#### Properties

- P34.1: Config file round-trip — for any valid config struct, SaveConfig then LoadConfig produces an equal struct (Req 34, AC 1–2)
### Requirement 35: Single Binary Distribution

**User Story:** As a language learner, I want to download a single binary for my platform and run it immediately, so that I do not need to install Go, Python, or any dependencies.

#### Acceptance Criteria

1. WHEN the App is compiled, THE App SHALL compile to a single binary with all templates, static assets, and SQLite driver embedded
2. WHEN the App is released, THE App SHALL cross-compile for macOS (amd64, arm64), Linux (amd64, arm64), and Windows (amd64)
3. WHEN the App is compiled, THE App SHALL use `go:embed` to embed HTML templates and static assets into the binary
4. WHEN the App is compiled, THE App SHALL use a pure-Go or CGo-compatible SQLite driver that compiles into the binary
5. WHEN the binary is run on a supported platform, THE App SHALL require no external runtime dependencies

### Requirement 36: Handle Errors Gracefully

**User Story:** As a language learner, I want the app to continue processing after errors, so that I can manually fix failed items later.

#### Acceptance Criteria

1. WHEN an LLM invocation fails, THE App SHALL log the error
2. WHEN a JSON parsing error occurs, THE App SHALL log the raw response at debug level
3. WHEN an error occurs during batch processing, THE App SHALL continue processing remaining items
4. WHEN processing completes, THE App SHALL print a summary report listing all failed items
5. WHEN authentication fails, THE App SHALL exit with a non-zero exit code and a clear error message suggesting corrective action

#### Properties

- P36.1: Error resilience — for any list of tokens containing both valid and invalid items, processing completes all valid items even when errors occur for some (Req 36, AC 3)

### Requirement 37: Dry-Run Mode

**User Story:** As a language learner, I want to preview what will be processed without spending API credits, so that I can verify parsing logic before running.

#### Acceptance Criteria

1. WHEN `--dry-run` flag is provided, THE App SHALL parse and normalize all input tokens
2. WHEN `--dry-run` flag is provided, THE App SHALL print normalized tokens to console
3. WHEN `--dry-run` flag is provided, THE App SHALL not invoke the LLM provider
4. WHEN `--dry-run` flag is provided, THE App SHALL not write to the Database
5. WHEN `--dry-run` flag is provided, THE App SHALL print a summary of what would be processed

#### Properties

- P37.1: Dry-run no side effects — for any input file, dry-run mode does not write to the Database, does not invoke the LLM provider, and does not modify any persistent state (Req 37, AC 3–4)

### Requirement 38: Progress Feedback

**User Story:** As a language learner, I want to see processing progress, so that I know the app is working and how much is left.

#### Acceptance Criteria

1. WHEN processing items, THE App SHALL log each processed item
2. WHEN skipping cached items, THE App SHALL log skip messages
3. WHEN `--verbose` flag is provided, THE App SHALL log prompts at debug level
4. WHEN `--verbose` flag is provided, THE App SHALL log raw LLM responses at debug level
5. WHEN processing completes, THE App SHALL print a summary with counts of processed, cached, and failed items

### Requirement 39: Handle UTF-8 Encoding

**User Story:** As a language learner, I want proper handling of special characters (ë, ï, ü, ő, ű, à, è, ì, ò, ù, я, ё, etc.), so that my vocabulary data displays correctly.

#### Acceptance Criteria

1. WHEN reading input files, THE App SHALL use UTF-8 encoding
2. WHEN writing to the Database, THE App SHALL preserve UTF-8 characters
3. WHEN serving web UI responses, THE App SHALL set Content-Type with charset=utf-8
4. WHEN logging to console, THE App SHALL handle UTF-8 characters without errors

#### Properties

- P39.1: UTF-8 round-trip consistency — for any text containing special characters, reading from UTF-8 source then writing to Database then reading back preserves the exact character sequence (Req 39, AC 1–2)

### Requirement 40: Configurable Target Translation Language

**User Story:** As a language learner sharing this tool with classmates, I want to configure which language appears as the second translation column, so that each user can choose their own native language for translations.

#### Acceptance Criteria

1. WHEN the CLI is invoked, THE App SHALL accept an optional `--target-language` CLI flag specifying the second translation language code (default: "hu" for Hungarian)
2. WHEN `--target-language` is provided, THE App SHALL use the specified language for the second translation field in prompts
3. WHEN a target language is specified, THE App SHALL support any language in Supported_Languages as target language
4. WHEN `--target-language` is not provided, THE App SHALL default to Hungarian ("hu")

### Requirement 41: Maintain Output Quality Parity

**User Story:** As a language learner, I want the prompts to produce vocabulary data of high quality for any source language, so that my vocabulary lists are useful.

#### Acceptance Criteria

1. WHEN processing a word in any supported language, THE LLM response SHALL contain native POS labels, native definitions, native example sentences, and native register labels for that language
2. WHEN the prompt templates are defined, THE prompt templates SHALL include the Core_Rule_Block, Decision_Rubric, and all field-level quality instructions
3. WHEN the prompt templates produce output, THE prompt templates SHALL produce output compatible with the English_Schema

### Requirement 42: Context Sentence Support in Prompts

**User Story:** As a language learner, I want to provide an optional context sentence alongside each word or expression, so that the LLM can disambiguate polysemous words and produce connotation-accurate translations.

#### Acceptance Criteria

1. WHEN the Prompt_Template is defined, THE Prompt_Template SHALL include a `{context}` placeholder that, when populated, provides the LLM with the sentence in which the word or expression appears
2. WHEN a context sentence is provided, THE Prompt_Template SHALL instruct the LLM to use the context to determine the correct sense, connotation, and register
3. WHEN no context sentence is provided, THE Prompt_Template SHALL instruct the LLM to infer the most typical B2–C1 textbook sense and note polysemy if the word is highly polysemous
4. WHEN the `BuildPrompt` function is called, THE `BuildPrompt` function SHALL accept an optional context parameter and inject it into the prompt template
### Requirement 43: Testing Strategy

**User Story:** As a developer, I want a comprehensive test suite using table-driven tests and property-based tests with the `rapid` library, so that correctness properties from the Python prototype are carried forward.

#### Acceptance Criteria

1. WHEN tests are written, THE test suite SHALL use Go table-driven tests for unit tests
2. WHEN correctness properties are tested, THE test suite SHALL use the `rapid` library (pgregory.net/rapid) for property-based tests verifying formal correctness properties
3. WHEN edge cases are discovered, THE test suite SHALL use Go's built-in fuzz testing (`func FuzzXxx(f *testing.F)`) for edge case discovery on parsers and validators
4. WHEN the test suite is designed, THE test suite SHALL carry forward all correctness properties from the Python prototype design document
5. WHEN templates are tested, THE test suite SHALL test the unified templates with language parameters
6. WHEN validation is tested, THE test suite SHALL use English field names in validation tests
7. WHEN BuildPrompt is tested, THE test suite SHALL include at least one test per supported language verifying that `BuildPrompt` produces a valid prompt containing the correct source language name
8. WHEN the Validator is tested, THE test suite SHALL include tests verifying that the Validator accepts both flat string and nested object translation fields
9. WHEN the Field_Mapper is tested, THE test suite SHALL include tests verifying that the Field_Mapper correctly flattens nested translation objects
10. WHEN round-trip validation is tested, THE test suite SHALL include a round-trip property test verifying that a valid JSON response passes validation and field mapping without error for all supported languages and both modes

### Requirement 44: Structured Logging

**User Story:** As a developer, I want all application logging to use `log/slog` with structured fields, so that log output is consistent, parseable, and free of `fmt.Println` in production code.

#### Acceptance Criteria

1. WHEN the App logs messages, THE App SHALL use `log/slog` for all production logging (no `fmt.Println` or `fmt.Printf` for log output)
2. WHEN the App starts, THE App SHALL configure the slog handler at startup based on the `--verbose` flag: text handler with INFO level by default, DEBUG level when verbose
3. WHEN the App logs messages, THE App SHALL include structured fields where applicable (e.g., `slog.String("word", token)`, `slog.String("provider", name)`)
4. WHEN the App processes items normally, THE App SHALL log at INFO level for: processed items, skip/cache messages, progress, summaries
5. WHEN the App processes LLM interactions in verbose mode, THE App SHALL log at DEBUG level for: prompts sent to LLM, raw LLM responses
6. WHEN the App encounters failures, THE App SHALL log at ERROR level for: authentication failures, LLM invocation errors, validation errors

### Requirement 45: Build and CI Tooling

**User Story:** As a developer, I want a Makefile and CI configuration that enforces code quality gates, so that every commit is vetted before merge.

#### Acceptance Criteria

1. WHEN the repository is set up, THE repository SHALL include a Makefile with targets: `build`, `test`, `lint`, `vet`, `fmt-check`
2. WHEN the `test` target is run, THE `test` target SHALL run `go test -race ./...`
3. WHEN the `lint` target is run, THE `lint` target SHALL run `staticcheck ./...`
4. WHEN the `vet` target is run, THE `vet` target SHALL run `go vet ./...`
5. WHEN the `fmt-check` target is run, THE `fmt-check` target SHALL verify all Go files are formatted with `gofmt` and fail if any are not
6. WHEN the `build` target is run, THE `build` target SHALL compile the binary with `go build -o vocabgen ./cmd/vocabgen`
7. WHEN a release is prepared, THE repository SHALL maintain a `CHANGELOG.md` file following Keep a Changelog format, with entries grouped per release under Added/Changed/Fixed sections

### Requirement 46: Database Schema Migration

**User Story:** As a developer, I want a lightweight schema migration mechanism, so that I can evolve the database schema without losing user data.

#### Acceptance Criteria

1. WHEN the Database stores schema information, THE Database SHALL store a `schema_version` integer in a `metadata` table
2. WHEN the Database is first created, THE Database SHALL set `schema_version` to 1 for the initial schema creation
3. WHEN the application starts and the existing `schema_version` is less than the current version, THE Database SHALL apply migration steps sequentially
4. WHEN a migration step fails, THE Database SHALL return an error and not apply subsequent steps
5. WHEN a migration step runs, THE migration step SHALL run inside a transaction so that a failed step leaves the database unchanged

### Requirement 47: Entry Tagging

**User Story:** As a language learner, I want to tag vocabulary entries with labels like "chapter-3" or "business-dutch", so that I can organize and filter my vocabulary by project, topic, or source.

#### Acceptance Criteria

1. WHEN the Database stores entries, THE Database SHALL store a `tags` TEXT column on both the words and expressions tables, containing comma-separated tag strings
2. WHEN the `--tags` CLI flag is provided, THE App SHALL apply the specified tags to all entries created during that session
3. WHEN a lookup or batch result is saved to the Database, THE App SHALL store the tags value (or empty string if no tags provided)
4. WHEN the Web_UI renders lookup and batch forms, THE Web_UI SHALL include an optional tags text input
5. WHEN the Database_Page renders filters, THE Database_Page SHALL include a tags filter that matches entries containing the specified tag
6. WHEN the Database_Page renders the edit form, THE Database_Page SHALL allow editing the tags field
7. WHEN the Excel_Exporter generates a file, THE Excel_Exporter SHALL include the tags column in exported files
8. WHEN the CSV_Importer parses a file, THE CSV_Importer SHALL support an optional tags column in imported files
### Requirement 48: Database Backup and Restore

**User Story:** As a user, I want to back up and restore my vocabulary database, so that I can recover from corruption or accidental deletion.

#### Acceptance Criteria

1. WHEN the `backup` subcommand is invoked, THE App SHALL copy the current SQLite database file to a timestamped backup file in the same directory (e.g., `vocabgen.db.2026-03-30T14-00-00.bak`)
2. WHEN the `restore` subcommand is invoked, THE App SHALL accept a backup file path and replace the current database with the backup
3. WHEN the `restore` subcommand is invoked, THE App SHALL verify the backup file is a valid SQLite database before replacing the current one
4. WHEN the `restore` subcommand is invoked, THE App SHALL create a backup of the current database before overwriting it
5. WHEN the `backup` subcommand completes, THE App SHALL print the path of the created backup file
6. WHEN the `restore` subcommand completes, THE App SHALL print a confirmation message after successful restore

### Requirement 49: Version Command

**User Story:** As a user, I want to check which version of the tool I'm running, so that I can report issues or verify I have the latest release.

#### Acceptance Criteria

1. WHEN the `version` subcommand is invoked, THE CLI SHALL print the application version, Go version, and build date
2. WHEN `--version` is provided on the root command, THE CLI SHALL print the version and exit
3. WHEN the binary is built, THE version string SHALL be injected at build time via Go linker flags (`-ldflags`)

### Requirement 50: LLM Request Timeout

**User Story:** As a user, I want LLM requests to time out after a reasonable period, so that the app doesn't hang indefinitely on unresponsive providers.

#### Acceptance Criteria

1. WHEN an LLM provider is invoked, THE App SHALL apply a configurable timeout to each invocation (default: 60 seconds)
2. WHEN the CLI is invoked, THE CLI SHALL accept an optional `--timeout` flag specifying the timeout in seconds
3. WHEN a provider invocation exceeds the timeout, THE App SHALL cancel the request and return a timeout error
4. WHEN the timeout is applied, THE timeout SHALL be implemented via `context.WithTimeout` on the context passed to the Provider's Invoke method

### Requirement 51: Vertex AI Provider Implementation

**User Story:** As a user with Google Cloud access, I want to use Vertex AI models (Gemini, PaLM, Claude on Vertex), so that I can leverage my existing GCP infrastructure.

#### Acceptance Criteria

1. WHEN the Vertex_AI_Provider is used, THE Vertex_AI_Provider SHALL implement the Provider_Interface
2. WHEN the Vertex_AI_Provider authenticates, THE Vertex_AI_Provider SHALL use Google Application Default Credentials (ADC) or a service account key file
3. WHEN no valid GCP credentials are available, THE Vertex_AI_Provider SHALL return a descriptive authentication error before attempting invocation
4. WHEN a GCP project is specified, THE Vertex_AI_Provider SHALL accept a GCP project ID via the `--gcp-project` CLI flag or the `GCP_PROJECT` environment variable
5. WHEN a GCP region is specified, THE Vertex_AI_Provider SHALL accept a GCP region via the `--gcp-region` CLI flag (default: "us-central1")
6. WHEN a model is specified, THE Vertex_AI_Provider SHALL support specifying a Vertex AI model name (e.g., "gemini-2.0-flash", "gemini-2.5-pro") via the `--model-id` flag
7. WHEN the Vertex_AI_Provider encounters rate-limit errors, THE Vertex_AI_Provider SHALL retry up to a configurable maximum

### Requirement 52: English Definition Field

**User Story:** As a language learner who is not yet advanced enough to fully understand source-language definitions, I want an English-language explanation of each word or expression's meaning, so that I can comprehend the definition even when the source-language version is too difficult.

#### Acceptance Criteria

1. WHEN the English_Schema is defined, THE English_Schema SHALL include an optional `english_definition` field (string) for both words and expressions
2. WHEN the `english_definition` field is populated, THE field SHALL contain a concise English-language explanation of the word or expression's meaning, distinct from the `english` translation field (which provides a direct translation) and the `definition` field (which provides the explanation in the source language)
3. WHEN the LLM returns a response with an `english_definition` field, THE Validator SHALL accept the field as a string and pass it through
4. WHEN the LLM returns a response without an `english_definition` field, THE Validator SHALL default the field to an empty string
5. WHEN an entry is stored, THE Database SHALL store the `english_definition` value in the `english_definition` column of both the words and expressions tables
6. WHEN a lookup result is displayed, THE Web_UI lookup result partial SHALL display the `english_definition` field when it is non-empty
7. WHEN the Database_Page renders an entry, THE Database_Page entry view and edit form SHALL include the `english_definition` field
8. WHEN the Excel_Exporter generates a file, THE Excel_Exporter SHALL include the `english_definition` column in exported files
9. WHEN the CSV_Importer parses a file, THE CSV_Importer SHALL support an optional `english_definition` column in imported files
10. WHEN the Field_Mapper processes an entry, THE Field_Mapper SHALL pass through the `english_definition` field from the validated entry to the output Entry struct without transformation
### Requirement 53: Multi-Version Vocabulary Entries

**User Story:** As a language learner, I want to store multiple versions of the same word when it has distinct meanings (e.g., "werk" as noun "work" and "werk" as verb "to work"), so that my vocabulary database captures all relevant senses of polysemous words.

#### Acceptance Criteria

1. WHEN entries are stored, THE Database SHALL allow multiple rows with the same word/expression text and source_language in the words and expressions tables (no unique constraint on the (source_language, word) or (source_language, expression) pair)
2. WHEN multiple entries exist for the same word/expression and source_language, THE Database_Page SHALL display all versions as separate rows in the table
3. WHEN multiple entries exist for the same word/expression, THE Excel_Exporter SHALL include all versions as separate rows in the exported file
4. WHEN the user selects the "add" conflict resolution strategy, THE App SHALL insert the new LLM result as a new row without modifying or removing the existing entry
5. WHEN the Database_Page displays entries that share the same word/expression text, THE Database_Page SHALL visually distinguish them (e.g., by displaying the part_of_speech or a version indicator) so the user can tell them apart

#### Properties

- P53.1: Multi-version entry integrity — "add" results in N+1 entries, "replace" targeting ID K updates K and leaves others unchanged, "skip" results in no modifications; FindWords/FindExpressions returns all N entries (Req 53, AC 1, 4)

### Requirement 54: Conflict Resolution Strategy

**User Story:** As a language learner, I want to choose whether to replace an existing entry, add a new version alongside it, or skip saving when I re-query a word with different context, so that I control how my vocabulary database evolves.

#### Acceptance Criteria

1. WHEN a conflict is detected, THE App SHALL support three conflict resolution strategies: "replace" (update the existing entry with the new LLM result), "add" (insert the new result as a separate entry), and "skip" (discard the new result and keep the existing entry unchanged)
2. WHEN the CLI `lookup` subcommand detects an existing entry and no `--on-conflict` flag is provided, THE CLI SHALL prompt the user interactively to choose replace, add, or skip, displaying a summary of the existing entry and the new result
3. WHEN the `--on-conflict` flag is provided, THE CLI SHALL apply the specified strategy without interactive prompting
4. WHEN the Web_UI lookup detects an existing entry and a context sentence was provided, THE Web_UI SHALL display both the existing entry and the new result side by side, with buttons for "Replace", "Add as New Version", and "Skip"
5. WHEN the Web_UI lookup detects an existing entry and no context sentence was provided, THE Web_UI SHALL return the cached result without invoking the LLM (existing behavior)
6. WHEN multiple existing entries match the word/expression and source_language, THE App SHALL display all existing entries so the user can choose which one to replace, or add a new version alongside all of them

### Requirement 55: Context-Aware Cache Bypass

**User Story:** As a language learner, I want the cache to be bypassed when I provide a context sentence for a word that already exists in the database, so that the LLM can produce a meaning specific to the context I provide.

#### Acceptance Criteria

1. WHEN a lookup request includes a non-empty context sentence and at least one matching entry exists in the Database, THE App SHALL invoke the LLM provider with the context sentence regardless of the cache hit
2. WHEN a lookup request includes no context sentence and a matching entry exists in the Database, THE App SHALL return the cached entry without invoking the LLM provider (existing behavior)
3. WHEN a lookup request includes a non-empty context sentence and no matching entry exists in the Database, THE App SHALL invoke the LLM provider and insert the result directly (no conflict resolution needed)
4. WHEN a cache bypass occurs due to a context sentence, THE App SHALL log the bypass including the word/expression text and the context sentence at DEBUG level

#### Properties

- P55.1: Context-aware cache bypass — lookup with empty context returns cached entry without LLM invocation; lookup with non-empty context invokes LLM regardless of cache state; final database state is consistent with selected conflict resolution strategy (Req 55, AC 1–3)

### Requirement 56: Batch Conflict Resolution

**User Story:** As a language learner running batch processing with context sentences, I want a default conflict resolution strategy for the batch, so that I do not have to answer prompts for every word that already exists.

#### Acceptance Criteria

1. WHEN the `batch` subcommand is invoked, THE `batch` subcommand SHALL accept an optional `--on-conflict` flag with values "replace", "add", or "skip" (default: "skip")
2. WHEN a batch token has a context sentence (from the CSV second column) and an existing entry is found, THE App SHALL apply the batch-level `--on-conflict` strategy automatically
3. WHEN a batch token has no context sentence and an existing entry is found, THE App SHALL skip the token (existing cache behavior, regardless of `--on-conflict` setting)
4. WHEN the Web_UI renders the batch form, THE Web_UI SHALL include a conflict resolution dropdown (replace, add, skip) that applies to all tokens in the batch
5. WHEN batch processing completes, THE batch summary SHALL include a count of replaced entries and added entries in addition to the existing processed, cached, and failed counts

### Requirement 57: Database Store Interface Updates for Multi-Version Support

**User Story:** As a developer, I want the database Store interface to support querying and managing multiple entries per word, so that the service layer can implement conflict resolution logic.

#### Acceptance Criteria

1. WHEN multiple entries are queried, THE Store interface SHALL expose a `FindWords(ctx, word, sourceLang)` function that returns a slice of all matching WordRow entries
2. WHEN multiple entries are queried, THE Store interface SHALL expose a `FindExpressions(ctx, expr, sourceLang)` function that returns a slice of all matching ExpressionRow entries
3. WHEN no matching entries exist, THE `FindWords`/`FindExpressions` functions SHALL return an empty slice and no error
4. WHEN backward compatibility is needed, THE existing `FindWord`/`FindExpression` functions SHALL remain available, returning the first matching entry or nil
5. WHEN the "replace" strategy targets a specific entry, THE `UpdateWord`/`UpdateExpression` functions SHALL accept an entry ID to update a specific version among multiple versions
### Requirement 58: Named Config Profiles

**User Story:** As a user with multiple LLM setups (local Ollama, sandbox Bedrock, production Bedrock), I want named configuration profiles in `config.yaml`, so that I can switch between them instantly via `--profile <name>` without editing the config file.

#### Acceptance Criteria

1. WHEN the Config_File contains a `profiles:` key, THE Config_Manager SHALL parse it as a `map[string]ProfileConfig` where each profile holds provider-related fields (provider, aws_profile, aws_region, model_id, base_url, gcp_project, gcp_region)
2. WHEN the Config_File contains a `default_profile:` key, THE Config_Manager SHALL use the named profile when no `--profile` flag is provided
3. WHEN the `--profile <name>` CLI flag is provided, THE Config_Manager SHALL resolve the named profile and populate the runtime `Config` struct from it
4. WHEN the `--profile` flag specifies a profile name that does not exist in the Config_File, THE Config_Manager SHALL return a descriptive error listing available profile names
5. WHEN the Config_File does NOT contain a `profiles:` key (flat format), THE Config_Manager SHALL treat the entire file as a single implicit `default` profile for backward compatibility
6. WHEN `SaveConfig` is called on a multi-profile config, THE Config_Manager SHALL preserve the multi-profile YAML structure (not flatten to single-profile format)
7. WHEN the Web_UI config page loads, THE Config_Page SHALL always display a profile selector dropdown showing the active profile name, regardless of how many profiles exist (including single-profile configs)
8. WHEN the user switches profiles in the Web_UI, THE Config_Page SHALL reload the form with the selected profile's values
9. WHEN the existing `--profile` flag (currently mapped to AWS profile) conflicts with the new config profile flag, THE CLI SHALL rename the existing flag to `--aws-profile` and use `--profile` for config profiles
10. WHEN the user selects "Add new profile" from the profile selector dropdown, THE Config_Page SHALL display an inline form prompting for a profile name
11. WHEN the user submits a new profile name via the inline form, THE Config_Manager SHALL create a new profile copying the current active profile's values as defaults, add it to the Config_File, and switch to the new profile
12. WHEN the user submits a profile name that already exists, THE Config_Manager SHALL return a descriptive error and not overwrite the existing profile
13. WHEN the user submits an empty profile name, THE Config_Page SHALL display a validation error without making a server request

#### Properties

- P58.1: Multi-profile config round-trip — save then load preserves all profiles and default_profile (Req 58, AC 1, 6)
- P58.2: Unknown profile name always returns error listing available profiles (Req 58, AC 4)
- P58.3: Flat config backward compatibility — old format loads as implicit `default` profile (Req 58, AC 5)
- P58.4: CreateProfile with duplicate name returns error without modifying existing profiles (Req 58, AC 12)
- P58.5: CreateProfile copies source profile values into new profile and adds it to FileConfig (Req 58, AC 11)

### Requirement 59: One-Click Local LLM Setup

**User Story:** As a non-technical user (e.g., a classmate), I want a one-click setup for a local LLM via Ollama, so that I can use VocabGen without cloud API keys, costs, or configuration knowledge.

#### Acceptance Criteria

1. WHEN the user runs `scripts/setup-local-llm.sh`, THE script SHALL detect the OS (macOS or Linux via `uname`)
2. WHEN Ollama is not installed, THE script SHALL download and install Ollama using the appropriate method for the detected OS
3. WHEN Ollama is already installed, THE script SHALL skip installation and proceed to model setup
4. WHEN Ollama is not running, THE script SHALL start the Ollama server (`ollama serve &`) and wait for it to become reachable
5. WHEN the Ollama server is running, THE script SHALL pull a recommended model (e.g., `mistral`) suitable for B2-C1 vocabulary tasks
6. WHEN the model is pulled, THE script SHALL verify the model responds by sending a quick test prompt via the OpenAI-compatible endpoint
7. WHEN verification succeeds, THE script SHALL write `~/.vocabgen/config.yaml` with a `local` profile: `provider: openai`, `base_url: http://localhost:11434/v1`, `model_id: mistral`, and set `default_profile: local`
8. WHEN the Web_UI config page is displayed, THE Config_Page SHALL include a "Setup Local LLM" button that triggers a backend SSE endpoint running the same setup logic
9. WHEN the SSE setup endpoint runs, THE endpoint SHALL stream progress events (detecting OS, checking Ollama, installing, pulling model, verifying, writing config) to the Web_UI
10. WHEN the setup completes successfully, THE Web_UI SHALL update the in-memory config and display a success message
11. WHEN the `openai` provider is configured with `base_url` pointing to `localhost:11434`, THE `validateProviderEnv` function SHALL check Ollama reachability instead of requiring `OPENAI_API_KEY`

#### Properties

- P59.1: Setup script always produces a valid config — provider, base_url, and model_id are all non-empty after successful setup (Req 59, AC 7)

### Requirement 60: E2E Tests Default to Local LLM

**User Story:** As a developer, I want E2E tests to default to the local Ollama profile, so that test runs are free, offline, and don't consume cloud API credits.

#### Acceptance Criteria

1. WHEN `scripts/e2e-test.sh` is invoked without a `-p` flag, THE script SHALL default to `--profile local` for all LLM-dependent test sections
2. WHEN the `-p PROFILE` flag is provided, THE script SHALL use the specified profile instead of `local`
3. WHEN the `E2E_PROFILE` environment variable is set, THE script SHALL use it as the profile (flag takes precedence over env var)
4. WHEN the profile is `local` and Ollama is not reachable at `http://localhost:11434/api/tags`, THE script SHALL print an actionable error message pointing to `scripts/setup-local-llm.sh` and exit 1
5. WHEN the `local` profile does not exist in the config, THE script SHALL print an actionable error message and exit 1
6. WHEN the Makefile `e2e` target is invoked, THE Makefile SHALL pass through the `E2E_PROFILE` environment variable to the script

#### Properties

- P60.1: E2E script defaults to local profile — invocation without flags passes `--profile local` to all LLM-dependent binary calls (Req 60, AC 1)

### Requirement 61: Dedicated Sentence Prompt Template

**User Story:** As a language learner, I want sentence lookups to use a dedicated prompt template that analyzes my sentence for grammar errors and extracts key vocabulary, so that I get useful feedback on sentences I write rather than having them treated as fixed expressions.

#### Acceptance Criteria

1. WHEN the Sentence_Template is defined, THE Sentence_Template SHALL be a Go string constant containing `{source_language}`, `{sentence}`, `{context}`, and `{target_language_name}` placeholders
2. WHEN the Sentence_Template is defined, THE Sentence_Template SHALL instruct the LLM to return JSON with English field names: `sentence` (string, required — the original sentence), `translation` (string, required — full sentence translation into the target language), `grammar_check` (object, required — containing `has_errors` boolean, `corrected_sentence` string, and `errors` array), and `vocabulary` (array, required — key vocabulary items extracted from the sentence)
3. WHEN the Sentence_Template is defined, THE Sentence_Template SHALL instruct the LLM to check for grammatical errors including word order, verb conjugation, spelling, case/gender agreement, and preposition usage
4. WHEN the Sentence_Template is defined, THE Sentence_Template SHALL instruct the LLM to provide a corrected version of the sentence when errors are found, and explain each error (what went wrong, why, and the correct form)
5. WHEN the Sentence_Template is defined, THE Sentence_Template SHALL instruct the LLM to extract key vocabulary items from the sentence, each with `word`, `type` (POS in source language terminology), `definition` (in source language), and `english` (English translation) fields
6. WHEN the Sentence_Template is formatted with any source language name, THE Sentence_Template SHALL produce a valid prompt that generates correct sentence analysis for that language
7. WHEN `BuildPrompt` is called with mode `"sentence"`, THE `BuildPrompt` function SHALL select the Sentence_Template and inject all parameters
8. WHEN `ValidateResponse` is called with mode `"sentence"`, THE Validator SHALL validate against the sentence schema: `sentence` (string, required), `translation` (string, required), `grammar_check` (object, required with `has_errors` bool, `corrected_sentence` string, `errors` array), `vocabulary` (array, required)
9. WHEN the `grammar_check.errors` array contains entries, THE Validator SHALL verify each entry has `original` (string), `corrected` (string), and `explanation` (string) fields
10. WHEN the `mode()` function in the service layer receives `"sentence"`, THE function SHALL return `"sentence"` (not `"expressions"`)

#### Properties

- P61.1: Sentence template formatting produces valid prompts for any source language name — contains resolved name, sentence, target language, no unresolved placeholders (Req 61, AC 1, 6)
- P61.2: BuildPrompt with mode "sentence" selects the Sentence_Template and produces distinct output from words and expressions modes (Req 61, AC 7)
- P61.3: ValidateResponse with mode "sentence" validates against the sentence schema and rejects responses missing required fields (Req 61, AC 8, 9)

### Requirement 62: Sentence Lookups Are Ephemeral

**User Story:** As a language learner, I want sentence lookups to be rendered directly without being saved to the database, so that my vocabulary database contains only reusable word and expression entries, not one-off sentence analyses.

#### Acceptance Criteria

1. WHEN a sentence lookup completes, THE App SHALL NOT write the result to the Database
2. WHEN a sentence lookup completes, THE App SHALL render the result directly to CLI output or Web UI and then discard it
3. WHEN a sentence lookup is requested, THE App SHALL NOT check the Database cache (sentences are always fresh LLM invocations)
4. WHEN a sentence lookup is requested via the Web_UI, THE Web_UI SHALL display the grammar check results, corrected sentence, error explanations, and extracted vocabulary in a structured layout
5. WHEN a sentence lookup is requested via the CLI, THE CLI SHALL print the sentence analysis as formatted JSON

#### Properties

- P62.1: Sentence lookup ephemeral — sentence lookups never write to the Database and never read from the cache; the LLM is always invoked (Req 62, AC 1, 3)

### Requirement 63: Bulk Delete Database Entries

**User Story:** As a language learner, I want to select multiple database entries and delete them in one action, so that I can clean up my vocabulary database efficiently.

#### Acceptance Criteria

1. WHEN the Database_Page is displayed, THE Web_UI SHALL show a checkbox next to each entry row
2. WHEN the user clicks a "select all" checkbox, THE Web_UI SHALL toggle all visible entry checkboxes
3. WHEN one or more entries are selected, THE Web_UI SHALL display a bulk delete action bar
4. WHEN the user confirms bulk delete, THE Web_UI SHALL send a single DELETE request with all selected entry IDs
5. WHEN the bulk delete API receives a list of IDs, THE Store SHALL delete all entries matching those IDs
6. WHEN the bulk delete completes, THE Web_UI SHALL refresh the entry table to reflect the deletions

### Requirement 64: Batch Processing Cancellation

**User Story:** As a language learner, I want to cancel a running batch from the Web UI, so that I can stop processing without losing partial results.

#### Acceptance Criteria

1. WHEN a batch is processing in the Web_UI, THE Web_UI SHALL display a Cancel button
2. WHEN the user clicks Cancel, THE Web_UI SHALL abort the fetch request via AbortController
3. WHEN the server detects context cancellation during batch processing, THE service layer SHALL stop processing remaining tokens
4. WHEN batch processing is cancelled, THE server SHALL emit a `cancelled` SSE event with partial results
5. WHEN batch processing is cancelled, THE Web_UI SHALL display partial results with a cancellation notice
6. WHEN batch processing is cancelled, THE entries processed before cancellation SHALL remain in the Database

## Correctness Properties

*Carried forward from the Python prototype and adapted for Go. Each property is tested with `rapid` (Go PBT library) using table-driven test patterns. Properties are also documented inline under their respective requirements above.*

| ID | Property | Validates |
|---|---|---|
| P1 | Template formatting produces valid prompts for any source language — contains resolved name, token, target language, no unresolved placeholders, Core Rule Block, Decision Rubric (words/expressions) or grammar check instructions (sentence) | Req 1, 2, 6, 61 |
| P2 | Translation field normalization — plain string or object with `primary`/`alternatives` normalizes to consistent object format | Req 3.3, 3.4 |
| P3 | Optional fields default to empty string when absent | Req 3.5 |
| P4 | Missing required fields return validation error listing all missing fields | Req 3.6, 3.7, 3.9 |
| P5 | BuildPrompt injects all parameters (source language, token, context, target language) into output | Req 6.1–6.7 |
| P6 | CSV parsing returns exactly the non-empty lines as (token, context) pairs | Req 14.2, 14.5, 14.6 |
| P7 | Field mapper pass-through preserves non-translation fields | Req 5.1 |
| P8 | Translation flattening — `{primary} (alternatives)` when alternatives non-empty, `primary` when empty | Req 5.2, 5.5 |
| P9 | Validation accepts any valid English-schema JSON for matching mode | Req 3.1, 3.2, 43.10 |
| P10 | Token normalization is idempotent — `normalize(normalize(x)) == normalize(x)` | Req 15, 16 |
| P11 | Database cache idempotency — two lookups = one LLM call + one cache hit, identical data | Req 18.1–18.4 |
| P12 | UTF-8 round-trip consistency — read → write to DB → read back preserves exact characters | Req 39 |
| P13 | Dry-run no side effects — no DB writes, no LLM invocations, no persistent state changes | Req 37 |
| P14 | Config file round-trip — `SaveConfig` then `LoadConfig` produces equal struct | Req 34 |
| P15 | Limit enforcement — for limit N and M tokens (M > N), at most N LLM invocations (excluding cached) | Req 23.6 |
| P16 | Error resilience — valid items complete even when errors occur for other items in batch | Req 36.3 |
| P17 | Provider interface consistency — invoke returns string or error (never both nil); name returns non-empty string | Req 7.2, 7.3, 7.6 |
| P18 | Multi-version entry integrity — "add" = N+1 entries, "replace" by ID = N entries with target updated, "skip" = no changes; FindWords/FindExpressions returns all entries | Req 53, 54, 57 |
| P19 | Context-aware cache bypass — empty context returns cache without LLM; non-empty context invokes LLM regardless; DB state consistent with conflict strategy | Req 55, 18.6 |
| P20 | Multi-profile config round-trip — save then load preserves all profiles and default_profile setting | Req 58, 34 |
| P21 | Unknown profile name returns error — loading with non-existent profile name always returns descriptive error | Req 58.4 |
| P22 | Flat config backward compat — old single-profile format loads as implicit `default` profile without error | Req 58.5 |
| P24 | CreateProfile duplicate name error — creating a profile with an existing name returns error without modifying profiles | Req 58.12 |
| P25 | CreateProfile copies source — new profile contains same values as source profile and is added to FileConfig | Req 58.11 |
| P23 | Local LLM setup produces valid config — after successful setup, config contains non-empty provider, base_url, and model_id | Req 59.7 |
| P26 | Sentence template formatting produces valid prompts for any source language — no unresolved placeholders, distinct from words/expressions | Req 61.1, 61.6, 61.7 |
| P27 | Sentence validation accepts valid sentence JSON and rejects missing required fields | Req 61.8, 61.9 |
| P28 | Sentence lookup ephemeral — never writes to DB, never reads cache, always invokes LLM | Req 62.1, 62.3 |

### Requirement 65: Docker Image Distribution

**User Story:** As a user, I want to run VocabGen as a Docker container, so that I can use it on any Docker-capable host (NAS, home server, CI) without downloading platform-specific binaries.

#### Acceptance Criteria

1. WHEN `docker build` is run on the repository, THE Dockerfile SHALL produce a working image using a multi-stage build (Go builder → distroless runtime)
2. WHEN the image is built, THE binary SHALL be compiled with `CGO_ENABLED=0` and ldflags for version injection via a `VERSION` build arg
3. WHEN the container starts without arguments, THE entrypoint SHALL default to `vocabgen serve --port 8080`
4. WHEN the container runs, THE image SHALL expose port 8080 and declare a volume for `~/.vocabgen/` persistence
5. WHEN the image is inspected, THE runtime stage SHALL use `gcr.io/distroless/static:nonroot` (no shell, no root user)
6. WHEN a `v*` tag is pushed, THE goreleaser config SHALL build Docker images for both `linux/amd64` and `linux/arm64` and push to `ghcr.io/npozs77/vocabgen`
7. WHEN images are pushed, THE goreleaser config SHALL create multi-arch manifest lists for both `:<version>` and `:latest` tags
8. WHEN the release workflow runs, THE job SHALL have `packages: write` permission and authenticate with GHCR via `docker/login-action`
9. WHEN the release workflow runs, THE job SHALL set up Docker Buildx and QEMU for cross-platform builds
10. WHEN `docker build` runs, THE `.dockerignore` SHALL exclude `.git`, `.github`, `.kiro`, `.vscode`, `reference/`, `coverage.out`, `dist/`, and the local binary
11. WHEN a user reads the README, THE Docker section SHALL show a `docker run` command with port mapping, volume mount, and env var for API keys
12. WHEN a user reads `docs/deployment.md`, THE Docker section SHALL document image tags, volume mount path, CLI commands via Docker, and local build instructions
13. WHEN a user reads `docs/user-guide.md`, THE Installation section SHALL mention Docker as an alternative to binary download

#### Properties

- P65.1: Image builds successfully from clean checkout (Req 65, AC 1)
- P65.2: Default CMD starts web server on port 8080 (Req 65, AC 3)
- P65.3: Container runs as nonroot user (Req 65, AC 5)

**Property coverage**: 28 properties across 65 requirements. Requirements without dedicated properties are either structural (CLI flags, UI layout), integration-level (tested via integration tests with mocks), or covered transitively by properties on their dependencies.

## Non-Functional Requirements

### Usability

- WHEN the App returns an error, THE error message SHALL be actionable — identifying what failed and suggesting corrective action
- WHEN the CLI is invoked with `--help`, THE output SHALL clearly describe all flags, subcommands, and defaults
- WHEN the Web_UI displays forms, THE forms SHALL pre-populate defaults from the Config_Manager
- WHEN batch processing runs, THE App SHALL provide real-time progress feedback (CLI: log lines; Web: SSE progress bar)

### Performance

- WHEN local operations run (parsing, validation, DB queries), THE operations SHALL complete in negligible time relative to LLM latency
- WHEN the Database is queried, THE Cache_Layer SHALL return cached results without perceptible delay for databases up to 50,000 entries
- WHEN batch processing runs, THE App SHALL not introduce overhead beyond LLM API latency per item

### Maintainability

- WHEN new source languages are added, THE App SHALL require only a registry entry (no template or schema changes)
- WHEN new LLM providers are added, THE App SHALL require only a new provider file and a registry entry
- WHEN the codebase is maintained, THE App SHALL use `log/slog` structured logging (no `fmt.Println` in production code)
- WHEN tests are written, THE test suite SHALL use table-driven tests and `rapid` PBT — no mocking frameworks

### Compatibility

- WHEN the App is distributed, THE App SHALL run on macOS (amd64, arm64), Linux (amd64, arm64), and Windows (amd64) as a single binary
- WHEN the App is distributed, THE App SHALL also be available as a multi-arch Docker image (`linux/amd64`, `linux/arm64`) on GitHub Container Registry
- WHEN the App is compiled, THE App SHALL use a pure-Go SQLite driver compatible with cross-compilation
- WHEN the App connects to LLM providers, THE App SHALL support Bedrock, OpenAI, Anthropic, Vertex AI, and OpenAI-compatible servers (Ollama, LM Studio)
- WHEN the App is built, THE App SHALL require Go 1.22+ and no external runtime dependencies

## Constraints

1. **No API Keys in Config**: API keys and AWS secrets SHALL NOT be stored in `config.yaml`; use environment variables or `--api-key` CLI flag
2. **No Per-Language Code Branches**: Templates, validation, and field mapping SHALL be language-agnostic; no per-language prompt constants or schema definitions
3. **No Web Framework**: Use stdlib `net/http` — no gin, echo, fiber, or other HTTP frameworks
4. **No ORM**: Use `database/sql` with parameterized queries — no GORM, sqlx, or similar
5. **No Mocking Frameworks**: Use Go interfaces and manual test doubles — no gomock, testify/mock
6. **No fmt.Println in Production**: All logging via `log/slog`
7. **Pure-Go SQLite**: Use a pure-Go or CGo-compatible SQLite driver (e.g., modernc.org/sqlite) — no CGo-only drivers that break cross-compilation
8. **Single Binary**: All templates, static assets, and the SQLite driver SHALL be embedded in the binary via `go:embed`
9. **Reference Directory Read-Only**: The `reference/` directory contains the Python prototype and is for reference only — never modify
10. **Solo Developer Scale**: Optimized for personal use and occasional sharing with classmates — not enterprise

## Success Criteria

The Go Vocabulary Generator is successfully implemented when:

1. All 65 requirements have passing acceptance criteria verified by tests
2. All 28 correctness properties pass with `rapid` PBT
3. `go test -race ./...` passes with zero failures
4. `go vet ./...` and `staticcheck ./...` report zero issues
5. The binary cross-compiles for all 5 target platforms (macOS amd64/arm64, Linux amd64/arm64, Windows amd64)
6. CLI subcommands (`lookup`, `batch`, `serve`, `backup`, `restore`, `version`, `update`) work end-to-end
7. Web UI pages (Lookup, Batch, Config, Database) render and function correctly with HTMX interactions
8. LLM provider interface works with at least Bedrock and one API-key provider (OpenAI or Anthropic)
9. SQLite cache eliminates duplicate LLM calls for previously looked-up tokens
10. Documentation covers CLI usage (via Cobra `--help`) and godoc comments on all exported symbols
11. Named config profiles allow switching between providers via `--profile <name>` on CLI and dropdown in Web UI
12. `scripts/setup-local-llm.sh` installs Ollama, pulls a model, and writes a working `local` profile
13. E2E tests default to the `local` profile and run without cloud API costs when Ollama is available
14. Docker images are published to GHCR on tagged releases with multi-arch support (amd64/arm64)


### Requirement 66: Database Picker and Live Database Switching

**User Story:** As a language learner, I want to select or create databases from the Web UI Config page and have the switch take effect immediately, so that I can manage per-course or per-project vocabulary without restarting the server.

#### Acceptance Criteria

1. WHEN the Config page loads, THE database picker dropdown SHALL list all `.db` files found in the config directory
2. WHEN the user selects an existing database from the dropdown and saves, THE server SHALL close the current database connection and open the selected database immediately (no restart required)
3. WHEN the user selects "Create new…" and enters a valid name, THE server SHALL create the empty `.db` file on save and switch to it immediately
4. WHEN the user enters an invalid database name (containing characters other than letters, numbers, hyphens, underscores), THE server SHALL display a validation error
5. WHEN the user enters a name that conflicts with an existing `.db` file, THE server SHALL display a conflict error
6. WHEN the user switches config profiles and the new profile has a different `db_path`, THE server SHALL switch to the new database immediately
7. WHEN `db_path` is set to `__new__` without a `db_path_new_name`, THE server SHALL fall back to the current database path
8. WHEN the database picker dropdown is rendered, THE dropdown SHALL be populated server-side (not via client-side HTMX/JS fetch)

#### Properties

- P66.1: Database picker lists all .db files in config directory (Req 66, AC 1)
- P66.2: Live database switch closes old connection and opens new one (Req 66, AC 2, 6)
- P66.3: New database file is created on disk when "Create new" is saved (Req 66, AC 3)
- P66.4: Invalid names are rejected with clear error (Req 66, AC 4, 5)

### Requirement 67: Multiple Meanings / Skip-Cache Lookup

**User Story:** As a language learner, I want to add multiple meanings for the same word (e.g., "beslaan" as "to fog up", "to occupy", "to shoe a horse"), each with its own context sentence, so that polysemous words are captured as separate, properly contextualized entries.

#### Acceptance Criteria

1. WHEN the user checks "Skip cache / Add new meaning" in the Web UI lookup form, THE system SHALL bypass the SQLite cache entirely and invoke the LLM
2. WHEN skip-cache is active, THE context sentence field SHALL be required (enforced client-side and server-side)
3. WHEN skip-cache is active and a context sentence is provided, THE system SHALL insert the LLM result as a new row without conflict resolution
4. WHEN the user uses `--new-meaning` on the CLI, THE system SHALL require `--context` and return an error if missing
5. WHEN a word has 2 or more entries in the database, THE disambiguation display SHALL show numeric suffixes: `word (1)`, `word (2)`, etc.
6. WHEN a word has exactly 1 entry, THE disambiguation display SHALL NOT show any suffix
7. WHEN disambiguation is applied, THE suffixes SHALL appear in the database browser, flashcards, and XLSX export
8. WHEN skip-cache is active in batch mode, THE system SHALL skip tokens without a context sentence and insert new rows for tokens with context
9. WHEN the `SkipCache` field is false (default), THE system SHALL behave identically to the existing cache-first flow

#### Properties

- P67.1: Skip-cache with context always inserts a new row (Req 67, AC 3)
- P67.2: Skip-cache without context returns error (Req 67, AC 2, 4)
- P67.3: Disambiguation suffix appears only when total > 1 (Req 67, AC 5, 6)
- P67.4: DisambiguatedWord is idempotent for display — never modifies stored data (Req 67, AC 7)

### Requirement 68: Database Detail Panel Toggle

**User Story:** As a user browsing the database, I want to click an expanded word/expression detail panel to collapse it, so that I can manage screen space without scrolling.

#### Acceptance Criteria

1. WHEN the user clicks an expanded detail panel in the database view, THE panel SHALL collapse (toggle behavior)
2. WHEN the user clicks a collapsed entry, THE detail panel SHALL expand as before
