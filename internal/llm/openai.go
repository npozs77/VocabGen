package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider implements Provider for the OpenAI chat completions API.
// Compatible with OpenAI, Azure OpenAI, Ollama, LM Studio, vLLM, and any
// server implementing the OpenAI chat completions endpoint.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates an OpenAIProvider.
// An API key is required unless a custom base URL is set (local servers like Ollama).
func NewOpenAIProvider(opts ProviderOptions) (Provider, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// API key is required unless a custom base URL is set (local server).
	if opts.APIKey == "" && opts.BaseURL == "" {
		return nil, &ProviderError{
			Provider: "openai",
			Message:  "API key is required: set OPENAI_API_KEY environment variable or use --api-key flag",
		}
	}

	return &OpenAIProvider{
		apiKey:  opts.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string { return "openai" }

// openaiRequest is the JSON body for the chat completions endpoint.
type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

// openaiMessage represents a single message in the chat completions request.
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the JSON body returned by the chat completions endpoint.
type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
}

// openaiChoice represents a single choice in the chat completions response.
type openaiChoice struct {
	Message openaiMessage `json:"message"`
}

// Invoke sends a chat completion request and returns the text response.
// It retries once on HTTP 429 (rate limit) with a 1-second delay.
func (p *OpenAIProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	reqBody := openaiRequest{
		Model: modelID,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", &ProviderError{
			Provider: "openai",
			Message:  "failed to marshal request: " + err.Error(),
			Err:      err,
		}
	}

	url := p.baseURL + "/chat/completions"

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Debug("openai: retrying after error", slog.Int("attempt", attempt+1), slog.String("error", lastErr.Error()))
			select {
			case <-ctx.Done():
				return "", &ProviderError{
					Provider: "openai",
					Message:  "context cancelled during retry",
					Err:      ctx.Err(),
				}
			case <-time.After(1 * time.Second):
			}
		}

		text, retryable, err := p.doRequest(ctx, url, bodyBytes)
		if err != nil {
			lastErr = err
			if retryable && attempt == 0 {
				continue
			}
			return "", err
		}
		return text, nil
	}

	return "", lastErr
}

// doRequest performs a single HTTP request and returns the extracted text,
// whether the error is retryable, and any error.
func (p *OpenAIProvider) doRequest(ctx context.Context, url string, body []byte) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", false, &ProviderError{
			Provider: "openai",
			Message:  "failed to create request: " + err.Error(),
			Err:      err,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "openai",
			Message:  "request failed: " + err.Error(),
			Err:      err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, &ProviderError{
			Provider: "openai",
			Message:  "failed to read response body: " + err.Error(),
			Err:      err,
		}
	}

	// Rate limit — retryable.
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", true, &ProviderError{
			Provider: "openai",
			Message:  "rate limited (HTTP 429): retries exhausted",
		}
	}

	// Non-2xx status.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return "", false, &ProviderError{
			Provider: "openai",
			Message:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet),
		}
	}

	// Parse response JSON.
	var result openaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, &ProviderError{
			Provider: "openai",
			Message:  "failed to parse response JSON: " + err.Error(),
			Err:      err,
		}
	}

	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", false, &ProviderError{
			Provider: "openai",
			Message:  "empty response from model",
		}
	}

	return result.Choices[0].Message.Content, false, nil
}
