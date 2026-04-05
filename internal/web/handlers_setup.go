package web

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/user/vocabgen/internal/config"
)

// sseEvent sends a single SSE event to the client.
func sseEvent(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	jsonData, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
	flusher.Flush()
}

// sseProgress sends a progress SSE event with step description and status.
func sseProgress(w http.ResponseWriter, flusher http.Flusher, step, status string) {
	sseEvent(w, flusher, "progress", map[string]string{
		"step":   step,
		"status": status,
	})
}

// handleSetupLocalLLM streams local LLM setup progress via SSE.
// Runs the same logic as scripts/setup-local-llm.sh using os/exec.
func (s *Server) handleSetupLocalLLM(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	const model = "translategemma"
	const ollamaURL = "http://localhost:11434"

	// --- Step 1: Detect OS ---
	stepName := "Detecting operating system..."
	sseProgress(w, flusher, stepName, "running")

	detectedOS := runtime.GOOS
	switch detectedOS {
	case "darwin", "linux":
		sseProgress(w, flusher, stepName, "done")
	default:
		sseEvent(w, flusher, "error", map[string]string{
			"message": fmt.Sprintf("Unsupported OS: %s. This setup supports macOS and Linux only.", detectedOS),
		})
		return
	}

	// --- Step 2: Check Ollama installation ---
	stepName = "Checking Ollama installation..."
	sseProgress(w, flusher, stepName, "running")

	ollamaPath, err := exec.LookPath("ollama")
	if err != nil {
		sseProgress(w, flusher, stepName, "done")

		// --- Step 3: Install Ollama ---
		stepName = "Installing Ollama..."
		sseProgress(w, flusher, stepName, "running")

		if installErr := installOllama(r, detectedOS); installErr != nil {
			sseEvent(w, flusher, "error", map[string]string{
				"message": "Failed to install Ollama: " + installErr.Error(),
			})
			return
		}

		// Verify installation succeeded
		_, err = exec.LookPath("ollama")
		if err != nil {
			sseEvent(w, flusher, "error", map[string]string{
				"message": "Ollama installation failed. Install manually: https://ollama.com/download",
			})
			return
		}
		sseProgress(w, flusher, stepName, "done")
	} else {
		s.logger.Info("ollama found", "path", ollamaPath)
		sseProgress(w, flusher, stepName, "done")
	}

	// --- Step 4: Ensure Ollama server is running ---
	stepName = "Checking Ollama server..."
	sseProgress(w, flusher, stepName, "running")

	if !ollamaServerReady(ollamaURL) {
		s.logger.Info("ollama server not running, starting it")
		// Start ollama serve in background.
		startCmd := exec.CommandContext(r.Context(), "ollama", "serve")
		if startErr := startCmd.Start(); startErr != nil {
			sseEvent(w, flusher, "error", map[string]string{
				"message": "Failed to start Ollama server: " + startErr.Error(),
			})
			return
		}
		// Wait up to 30s for server to become ready.
		ready := false
		for i := 0; i < 30; i++ {
			if ollamaServerReady(ollamaURL) {
				ready = true
				break
			}
			select {
			case <-r.Context().Done():
				sseEvent(w, flusher, "error", map[string]string{
					"message": "Setup cancelled.",
				})
				return
			case <-time.After(1 * time.Second):
			}
		}
		if !ready {
			sseEvent(w, flusher, "error", map[string]string{
				"message": "Ollama server did not start within 30 seconds. Try running 'ollama serve' manually.",
			})
			return
		}
	}
	sseProgress(w, flusher, stepName, "done")

	// --- Step 5: Pull model (streamed) ---
	stepName = fmt.Sprintf("Pulling model '%s'...", model)
	sseProgress(w, flusher, stepName, "running")

	pullCmd := exec.CommandContext(r.Context(), "ollama", "pull", model)
	pullStdout, pipeErr := pullCmd.StdoutPipe()
	if pipeErr != nil {
		sseEvent(w, flusher, "error", map[string]string{
			"message": "Failed to start model pull: " + pipeErr.Error(),
		})
		return
	}
	pullCmd.Stderr = pullCmd.Stdout // merge stderr into stdout

	if startErr := pullCmd.Start(); startErr != nil {
		sseEvent(w, flusher, "error", map[string]string{
			"message": "Failed to start model pull: " + startErr.Error(),
		})
		return
	}

	scanner := bufio.NewScanner(pullStdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			sseEvent(w, flusher, "pull_progress", map[string]string{"line": line})
		}
	}

	if waitErr := pullCmd.Wait(); waitErr != nil {
		sseEvent(w, flusher, "error", map[string]string{
			"message": fmt.Sprintf("Failed to pull model '%s': %s", model, waitErr.Error()),
		})
		return
	}
	sseProgress(w, flusher, stepName, "done")

	// --- Step 6: Verify model responds ---
	stepName = "Verifying model responds..."
	sseProgress(w, flusher, stepName, "running")

	verifyCmd := exec.CommandContext(r.Context(), "curl", "-sf",
		ollamaURL+"/v1/chat/completions",
		"-H", "Content-Type: application/json",
		"-d", fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"Say hello in one word."}],"max_tokens":10}`, model),
	)
	verifyOutput, verifyErr := verifyCmd.CombinedOutput()
	if verifyErr != nil || !strings.Contains(string(verifyOutput), "choices") {
		sseEvent(w, flusher, "error", map[string]string{
			"message": "Model verification failed. Ollama may not be serving '" + model + "' correctly.",
		})
		return
	}
	sseProgress(w, flusher, stepName, "done")

	// --- Step 7: Write config ---
	stepName = "Writing config..."
	sseProgress(w, flusher, stepName, "running")

	if cfgErr := writeLocalLLMConfig(model); cfgErr != nil {
		sseEvent(w, flusher, "error", map[string]string{
			"message": "Failed to write config: " + cfgErr.Error(),
		})
		return
	}
	sseProgress(w, flusher, stepName, "done")

	// Update in-memory config
	cfg, loadErr := config.LoadConfigWithProfile("local")
	if loadErr != nil {
		s.logger.Error("reload config after local LLM setup failed", "error", loadErr)
	} else {
		*s.cfg = cfg
		s.activeProfile = "local"
	}

	s.logger.Info("local LLM setup complete", "model", model)

	// --- Complete ---
	sseEvent(w, flusher, "complete", map[string]string{
		"message": fmt.Sprintf("Local LLM setup complete! Using model: %s", model),
	})
}

// ollamaServerReady checks if the Ollama server is reachable.
func ollamaServerReady(baseURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// installOllama attempts to install Ollama using the appropriate method for the OS.
func installOllama(r *http.Request, detectedOS string) error {
	switch detectedOS {
	case "darwin":
		// Try Homebrew first, fall back to curl installer.
		if _, err := exec.LookPath("brew"); err == nil {
			cmd := exec.CommandContext(r.Context(), "brew", "install", "ollama")
			if output, brewErr := cmd.CombinedOutput(); brewErr != nil {
				return fmt.Errorf("brew install failed: %s — %s", brewErr.Error(), strings.TrimSpace(string(output)))
			}
			return nil
		}
		return runCurlInstaller(r)
	case "linux":
		return runCurlInstaller(r)
	default:
		return fmt.Errorf("unsupported OS: %s", detectedOS)
	}
}

// runCurlInstaller runs the Ollama curl installer script.
func runCurlInstaller(r *http.Request) error {
	cmd := exec.CommandContext(r.Context(), "bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("curl installer failed: %s — %s", err.Error(), strings.TrimSpace(string(output)))
	}
	return nil
}

// writeLocalLLMConfig writes or updates the config file with a "local" profile
// pointing to the local Ollama instance.
func writeLocalLLMConfig(model string) error {
	// Load existing config to preserve settings.
	data, err := loadExistingFileConfig()
	if err != nil {
		return err
	}

	// Add/update the local profile.
	if data.Profiles == nil {
		data.Profiles = make(map[string]config.ProfileConfig)
	}
	data.Profiles["local"] = config.ProfileConfig{
		Provider: "openai",
		BaseURL:  "http://localhost:11434/v1",
		ModelID:  model,
	}
	data.DefaultProfile = "local"

	return config.SaveFileConfig(data)
}

// loadExistingFileConfig loads the existing FileConfig, converting flat format
// if necessary. Returns a fresh FileConfig with defaults if no file exists.
func loadExistingFileConfig() (config.FileConfig, error) {
	profiles, _, err := config.ListProfiles()
	if err != nil {
		// No existing config — return defaults.
		return config.FileConfig{
			DefaultProfile:        "local",
			Profiles:              make(map[string]config.ProfileConfig),
			DefaultSourceLanguage: "nl",
			DefaultTargetLanguage: "hu",
			DBPath:                "~/.vocabgen/vocabgen.db",
		}, nil
	}

	// Build FileConfig from existing profiles.
	fc := config.FileConfig{
		Profiles: make(map[string]config.ProfileConfig),
	}

	for _, name := range profiles {
		cfg, loadErr := config.LoadConfigWithProfile(name)
		if loadErr != nil {
			continue
		}
		fc.Profiles[name] = config.ProfileConfig{
			Provider:   cfg.Provider,
			AWSProfile: cfg.AWSProfile,
			AWSRegion:  cfg.AWSRegion,
			ModelID:    cfg.ModelID,
			BaseURL:    cfg.BaseURL,
			GCPProject: cfg.GCPProject,
			GCPRegion:  cfg.GCPRegion,
		}
		// Use shared fields from any profile (they're the same across profiles).
		fc.DefaultSourceLanguage = cfg.DefaultSourceLanguage
		fc.DefaultTargetLanguage = cfg.DefaultTargetLanguage
		fc.DBPath = cfg.DBPath
	}

	// Apply defaults for shared fields if empty.
	if fc.DefaultSourceLanguage == "" {
		fc.DefaultSourceLanguage = "nl"
	}
	if fc.DefaultTargetLanguage == "" {
		fc.DefaultTargetLanguage = "hu"
	}
	if fc.DBPath == "" {
		fc.DBPath = "~/.vocabgen/vocabgen.db"
	}

	return fc, nil
}
