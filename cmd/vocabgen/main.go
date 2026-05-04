package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/vocabgen/internal/config"
	"github.com/user/vocabgen/internal/db"
	"github.com/user/vocabgen/internal/llm"
	"github.com/user/vocabgen/internal/parsing"
	"github.com/user/vocabgen/internal/service"
	"github.com/user/vocabgen/internal/update"
	"github.com/user/vocabgen/internal/web"
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	buildDate = "unknown"
)

// appConfig holds the loaded config after PersistentPreRun.
var appConfig config.Config

// main executes the root Cobra command and exits with code 1 on error.
func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd is the base command for vocabgen.
var rootCmd = &cobra.Command{
	Use:     "vocabgen",
	Short:   "Vocabulary generator powered by LLM providers",
	Long:    "A CLI and embedded web app for generating structured vocabulary lists using LLM providers.",
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for version and update subcommands
		if cmd.Name() == "version" || cmd.Name() == "update" {
			return nil
		}

		// Configure slog
		verbose, _ := cmd.Flags().GetBool("verbose")
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
		slog.SetDefault(slog.New(handler))

		// Load config — use profile-aware loading if --profile is set.
		profileFlag, _ := cmd.Flags().GetString("profile")
		var cfg config.Config
		if profileFlag != "" {
			var loadErr error
			cfg, loadErr = config.LoadConfigWithProfile(profileFlag)
			if loadErr != nil {
				return loadErr
			}
		} else {
			var loadErr error
			cfg, loadErr = config.LoadConfig()
			if loadErr != nil {
				slog.Warn("failed to load config, using defaults", "error", loadErr)
				cfg = config.DefaultConfig()
			}
		}
		// Log if config file was not found (using defaults)
		if _, statErr := os.Stat(config.FilePath()); os.IsNotExist(statErr) {
			slog.Info("no config file found, using defaults", "path", config.FilePath())
		}

		// Apply CLI flag overrides
		if f := cmd.Flags().Lookup("provider"); f != nil && f.Changed {
			cfg.Provider, _ = cmd.Flags().GetString("provider")
		}
		if f := cmd.Flags().Lookup("region"); f != nil && f.Changed {
			cfg.AWSRegion, _ = cmd.Flags().GetString("region")
		}
		if f := cmd.Flags().Lookup("model-id"); f != nil && f.Changed {
			cfg.ModelID, _ = cmd.Flags().GetString("model-id")
		}
		if f := cmd.Flags().Lookup("base-url"); f != nil && f.Changed {
			cfg.BaseURL, _ = cmd.Flags().GetString("base-url")
		}
		if f := cmd.Flags().Lookup("aws-profile"); f != nil && f.Changed {
			cfg.AWSProfile, _ = cmd.Flags().GetString("aws-profile")
		}
		if f := cmd.Flags().Lookup("source-language"); f != nil && f.Changed {
			cfg.DefaultSourceLanguage, _ = cmd.Flags().GetString("source-language")
		}
		if f := cmd.Flags().Lookup("target-language"); f != nil && f.Changed {
			cfg.DefaultTargetLanguage, _ = cmd.Flags().GetString("target-language")
		}
		if f := cmd.Flags().Lookup("gcp-project"); f != nil && f.Changed {
			cfg.GCPProject, _ = cmd.Flags().GetString("gcp-project")
		}
		if f := cmd.Flags().Lookup("gcp-region"); f != nil && f.Changed {
			cfg.GCPRegion, _ = cmd.Flags().GetString("gcp-region")
		}
		if f := cmd.Flags().Lookup("db-path"); f != nil && f.Changed {
			cfg.DBPath, _ = cmd.Flags().GetString("db-path")
		}

		appConfig = cfg
		return nil
	},
}

// init configures the root command's version template, persistent flags, and
// registers all subcommands.
func init() {
	// Custom version template to include build date
	rootCmd.SetVersionTemplate(fmt.Sprintf("vocabgen %s (%s, built %s)\n", version, runtime.Version(), buildDate))

	// Persistent flags available to all subcommands
	pf := rootCmd.PersistentFlags()
	pf.BoolP("verbose", "v", false, "Enable debug logging")
	pf.String("provider", "bedrock", "LLM provider (bedrock, openai, anthropic, vertexai)")
	pf.StringP("region", "r", "us-east-1", "AWS region for Bedrock provider")
	pf.Int("timeout", 60, "Per-request timeout in seconds")
	pf.String("tags", "", "Comma-separated tags for entries")
	pf.StringP("source-language", "l", "", "Source language code or name")
	pf.String("target-language", "hu", "Target language code or name")
	pf.String("model-id", "", "LLM model identifier")
	pf.String("api-key", "", "API key for OpenAI/Anthropic providers")
	pf.String("base-url", "", "Custom API base URL (OpenAI-compatible servers)")
	pf.String("aws-profile", "", "AWS profile name (Bedrock provider)")
	pf.String("profile", "", "Config profile name (from profiles: in config.yaml)")
	pf.String("gcp-project", "", "GCP project ID (Vertex AI provider)")
	pf.String("gcp-region", "us-central1", "GCP region (Vertex AI provider)")
	pf.String("db-path", "", "Database file path (overrides config)")

	// Register subcommands
	rootCmd.AddCommand(lookupCmd)
	rootCmd.AddCommand(batchCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
}

// createProvider builds a Provider from the current config and CLI flags.
func createProvider(cmd *cobra.Command) (llm.Provider, error) {
	providerName := appConfig.Provider
	slog.Debug("creating provider", slog.String("provider", providerName))

	// Validate provider name
	constructor, ok := llm.Registry[providerName]
	if !ok {
		valid := make([]string, 0, len(llm.Registry))
		for k := range llm.Registry {
			valid = append(valid, k)
		}
		return nil, fmt.Errorf("unsupported provider %q: valid providers are %s", providerName, strings.Join(valid, ", "))
	}

	// Resolve API key: --api-key flag > env var
	apiKey, _ := cmd.Flags().GetString("api-key")
	if apiKey == "" {
		switch providerName {
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
			if apiKey == "" && appConfig.BaseURL == "" {
				return nil, fmt.Errorf("openai provider requires an API key: set --api-key flag or OPENAI_API_KEY environment variable")
			}
		case "anthropic":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
			if apiKey == "" {
				return nil, fmt.Errorf("anthropic provider requires an API key: set --api-key flag or ANTHROPIC_API_KEY environment variable")
			}
		}
	}

	// Resolve GCP project: flag > env var
	gcpProject := appConfig.GCPProject
	if gcpProject == "" {
		gcpProject = os.Getenv("GCP_PROJECT")
	}
	if providerName == "vertexai" && gcpProject == "" {
		return nil, fmt.Errorf("vertexai provider requires a GCP project ID: set --gcp-project flag or GCP_PROJECT environment variable")
	}

	opts := llm.ProviderOptions{
		APIKey:     apiKey,
		BaseURL:    appConfig.BaseURL,
		Region:     appConfig.AWSRegion,
		Profile:    appConfig.AWSProfile,
		GCPProject: gcpProject,
	}

	p, err := constructor(opts)
	if err != nil {
		slog.Error("provider creation failed", slog.String("provider", providerName), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create %s provider: %w", providerName, err)
	}
	slog.Debug("provider ready", slog.String("provider", providerName))
	return p, nil
}

// warnIfLocalModel prints a warning to stderr when the active config uses a
// local Ollama model, which may produce lower quality results.
func warnIfLocalModel() {
	if strings.Contains(appConfig.BaseURL, "localhost:11434") {
		fmt.Fprintln(os.Stderr, "⚠ Using a local model (Ollama). Translation quality may be lower — especially for sentence analysis and less common languages. For best results, use a cloud provider (OpenAI, Anthropic, or AWS Bedrock).")
	}
}

// openStore opens the SQLite database from the configured path.
func openStore() (*db.SQLiteStore, error) {
	dbPath := appConfig.DBPath
	if strings.HasPrefix(dbPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		dbPath = filepath.Join(home, dbPath[2:])
	}
	store, err := db.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}
	return store, nil
}

// printJSON marshals v to indented JSON and writes to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// lookupCmd implements the "vocabgen lookup" subcommand.
var lookupCmd = &cobra.Command{
	Use:   "lookup [text]",
	Short: "Look up a word or expression",
	Args:  cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		srcLang, _ := cmd.Flags().GetString("source-language")
		if srcLang == "" && appConfig.DefaultSourceLanguage == "" {
			return fmt.Errorf("--source-language (-l) is required")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		lookupType, _ := cmd.Flags().GetString("type")
		ctxSentence, _ := cmd.Flags().GetString("context")
		onConflict, _ := cmd.Flags().GetString("on-conflict")
		tags, _ := cmd.Flags().GetString("tags")
		timeout, _ := cmd.Flags().GetInt("timeout")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		sourceLang := appConfig.DefaultSourceLanguage
		if sl, _ := cmd.Flags().GetString("source-language"); sl != "" {
			sourceLang = sl
		}
		targetLang := appConfig.DefaultTargetLanguage

		provider, err := createProvider(cmd)
		if err != nil {
			return err
		}
		warnIfLocalModel()

		store, err := openStore()
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		var conflictStrategy service.ConflictStrategy
		if onConflict != "" {
			cs, err := service.ParseConflictStrategy(onConflict)
			if err != nil {
				return err
			}
			conflictStrategy = cs
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second)
		defer cancel()

		result, err := service.Lookup(ctx, store, service.LookupParams{
			SourceLang: sourceLang,
			LookupType: lookupType,
			Text:       text,
			Provider:   provider,
			ModelID:    appConfig.ModelID,
			Context:    ctxSentence,
			TargetLang: targetLang,
			Tags:       tags,
			DryRun:     dryRun,
			Timeout:    time.Duration(timeout) * time.Second,
			OnConflict: conflictStrategy,
		})
		if err != nil {
			return err
		}

		if result.FromCache {
			slog.Info("served from cache")
		}

		// Handle conflict resolution
		if result.NeedsResolution && onConflict == "" {
			// Interactive conflict resolution
			fmt.Fprintln(os.Stderr, "\nConflict detected: existing entry found for this word/expression with context.")
			fmt.Fprintln(os.Stderr, "\nExisting entries:")
			for i, e := range result.Existing {
				data, _ := json.MarshalIndent(e, "  ", "  ")
				fmt.Fprintf(os.Stderr, "  [%d] (ID %d):\n  %s\n", i+1, result.ExistingIDs[i], string(data))
			}
			fmt.Fprintln(os.Stderr, "\nNew result:")
			data, _ := json.MarshalIndent(result.Entry, "  ", "  ")
			fmt.Fprintf(os.Stderr, "  %s\n", string(data))

			fmt.Fprint(os.Stderr, "\nChoose action — (r)eplace, (a)dd, (s)kip: ")
			var choice string
			_, _ = fmt.Scanln(&choice)

			switch strings.ToLower(strings.TrimSpace(choice)) {
			case "r", "replace":
				conflictStrategy = service.ConflictReplace
			case "a", "add":
				conflictStrategy = service.ConflictAdd
			case "s", "skip":
				conflictStrategy = service.ConflictSkip
			default:
				slog.Info("invalid choice, skipping")
				conflictStrategy = service.ConflictSkip
			}

			m := "words"
			if lookupType == "expression" || lookupType == "sentence" {
				m = "expressions"
			}
			targetID := int64(0)
			if len(result.ExistingIDs) > 0 {
				targetID = result.ExistingIDs[0]
			}
			if err := service.ResolveConflict(ctx, store, conflictStrategy, m, result.Entry, targetID, sourceLang, targetLang, tags); err != nil {
				return fmt.Errorf("resolve conflict: %w", err)
			}
		} else if result.NeedsResolution && onConflict != "" {
			// Already resolved via OnConflict in LookupParams — service handled it
			slog.Info("conflict resolved", "strategy", onConflict)
		}

		if result.Warning != "" {
			fmt.Fprintln(os.Stderr, result.Warning)
		}

		return printJSON(result.Entry)
	},
}

func init() {
	lookupCmd.Flags().String("type", "word", "Lookup type (word, expression, sentence)")
	lookupCmd.Flags().String("context", "", "Context sentence for the lookup")
	lookupCmd.Flags().String("on-conflict", "", "Conflict resolution strategy (replace, add, skip)")
	lookupCmd.Flags().Bool("dry-run", false, "Preview without LLM invocation or DB writes")
}

// batchCmd implements the "vocabgen batch" subcommand.
var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Process a batch of words or expressions from a CSV file",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		srcLang, _ := cmd.Flags().GetString("source-language")
		if srcLang == "" && appConfig.DefaultSourceLanguage == "" {
			return fmt.Errorf("--source-language (-l) is required")
		}
		mode, _ := cmd.Flags().GetString("mode")
		if mode != "words" && mode != "expressions" {
			return fmt.Errorf("--mode must be \"words\" or \"expressions\", got %q", mode)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile, _ := cmd.Flags().GetString("input-file")
		mode, _ := cmd.Flags().GetString("mode")
		limit, _ := cmd.Flags().GetInt("limit")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		onConflict, _ := cmd.Flags().GetString("on-conflict")
		tags, _ := cmd.Flags().GetString("tags")
		timeout, _ := cmd.Flags().GetInt("timeout")

		sourceLang := appConfig.DefaultSourceLanguage
		if sl, _ := cmd.Flags().GetString("source-language"); sl != "" {
			sourceLang = sl
		}
		targetLang := appConfig.DefaultTargetLanguage

		// Parse conflict strategy
		cs, err := service.ParseConflictStrategy(onConflict)
		if err != nil {
			return err
		}

		// Read input CSV
		tokens, err := parsing.ReadInputFile(inputFile)
		if err != nil {
			return err
		}
		slog.Info("read input file", "path", inputFile, "tokens", len(tokens))

		provider, err := createProvider(cmd)
		if err != nil {
			return err
		}
		warnIfLocalModel()

		store, err := openStore()
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(timeout)*time.Second*time.Duration(len(tokens)+1))
		defer cancel()

		slog.Info("starting batch processing",
			slog.String("mode", mode),
			slog.String("source_lang", sourceLang),
			slog.Int("tokens", len(tokens)),
			slog.Int("limit", limit),
			slog.Bool("dry_run", dryRun),
		)

		result, err := service.ProcessBatch(ctx, store, service.BatchParams{
			SourceLang: sourceLang,
			Mode:       mode,
			Tokens:     tokens,
			Provider:   provider,
			ModelID:    appConfig.ModelID,
			TargetLang: targetLang,
			Tags:       tags,
			Limit:      limit,
			DryRun:     dryRun,
			Timeout:    time.Duration(timeout) * time.Second,
			OnConflict: cs,
		})
		if err != nil {
			return err
		}

		// Print summary
		slog.Info("batch complete",
			"processed", result.Processed,
			"cached", result.Cached,
			"failed", result.Failed,
			"skipped", result.Skipped,
			"replaced", result.Replaced,
			"added", result.Added,
		)

		fmt.Fprintf(os.Stderr, "\n--- Batch Summary ---\n")
		fmt.Fprintf(os.Stderr, "Processed: %d\n", result.Processed)
		fmt.Fprintf(os.Stderr, "Cached:    %d\n", result.Cached)
		fmt.Fprintf(os.Stderr, "Failed:    %d\n", result.Failed)
		fmt.Fprintf(os.Stderr, "Skipped:   %d\n", result.Skipped)
		fmt.Fprintf(os.Stderr, "Replaced:  %d\n", result.Replaced)
		fmt.Fprintf(os.Stderr, "Added:     %d\n", result.Added)

		if len(result.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "\nFailed items:\n")
			for _, e := range result.Errors {
				slog.Error("batch item failed", slog.String("token", e.Token), slog.String("error", e.Message))
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", e.Token, e.Message)
			}
		}

		return nil
	},
}

func init() {
	batchCmd.Flags().String("input-file", "", "Path to input CSV file (required)")
	batchCmd.Flags().String("mode", "", "Processing mode (words, expressions) (required)")
	batchCmd.Flags().Int("limit", 0, "Maximum number of new items to process (0 = no limit)")
	batchCmd.Flags().Bool("dry-run", false, "Preview without LLM invocation or DB writes")
	batchCmd.Flags().String("on-conflict", "skip", "Conflict resolution strategy (replace, add, skip)")
	_ = batchCmd.MarkFlagRequired("input-file")
	_ = batchCmd.MarkFlagRequired("mode")
}

// serveCmd implements the "vocabgen serve" subcommand.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the embedded web UI server",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")

		store, err := openStore()
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		// Resolve the DB path for display in the web UI.
		resolvedDBPath := appConfig.DBPath
		if strings.HasPrefix(resolvedDBPath, "~/") {
			if home, hErr := os.UserHomeDir(); hErr == nil {
				resolvedDBPath = filepath.Join(home, resolvedDBPath[2:])
			}
		}

		addr := fmt.Sprintf(":%d", port)
		slog.Info("starting web server", "addr", addr)

		// Graceful shutdown via signal handling
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		srv := web.NewServer(store, &appConfig, slog.Default(), version, buildDate, runtime.Version(), resolvedDBPath)
		return srv.ListenAndServe(ctx, addr)
	},
}

func init() {
	serveCmd.Flags().Int("port", 8080, "Port to listen on")
}

// backupCmd implements the "vocabgen backup" subcommand.
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a backup of the vocabulary database",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		// Build timestamped backup path
		dbPath := appConfig.DBPath
		if strings.HasPrefix(dbPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory: %w", err)
			}
			dbPath = filepath.Join(home, dbPath[2:])
		}
		timestamp := time.Now().Format("2006-01-02T15-04-05")
		backupPath := dbPath + "." + timestamp + ".bak"

		if err := store.BackupTo(cmd.Context(), backupPath); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}

		slog.Info("backup created", "path", backupPath)
		fmt.Println(backupPath)
		return nil
	},
}

// restoreCmd implements the "vocabgen restore" subcommand.
var restoreCmd = &cobra.Command{
	Use:   "restore [backup-file]",
	Short: "Restore the vocabulary database from a backup",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		backupFile := args[0]

		store, err := openStore()
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		if err := store.RestoreFrom(cmd.Context(), backupFile); err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}

		slog.Info("database restored", "from", backupFile)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Database restored successfully.")
		return nil
	},
}

// versionCmd implements the "vocabgen version" subcommand.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "vocabgen %s (%s, built %s)\n", version, runtime.Version(), buildDate)

		// Best-effort update check with 5-second timeout — never fails the version command.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		info := update.CheckNow(ctx, version)
		if info.HasUpdate && info.Error == "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Update available: v%s — run 'vocabgen update' for details\n", info.LatestVersion)
		}

		return nil
	},
}

// updateCmd implements the "vocabgen update" subcommand.
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for newer versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		info := update.CheckNow(ctx, version)

		if info.Error != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "vocabgen %s\n", info.CurrentVersion)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", info.Error)
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Visit https://github.com/npozs77/VocabGen/releases for manual download.\n")
			return fmt.Errorf("%s", info.Error)
		}

		if !info.HasUpdate {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "vocabgen %s is up to date.\n", info.CurrentVersion)
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", info.CurrentVersion)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Latest version:  %s\n", info.LatestVersion)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Download: %s\n", info.DownloadURL)
		if info.ChangelogText != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nChangelog:\n%s", info.ChangelogText)
		}

		return nil
	},
}
