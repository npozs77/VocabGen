package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateProviderEnv(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		baseURL    string
		gcpProject string
		envVars    map[string]string
		wantEmpty  bool // true = no warning expected
		wantSubstr string
	}{
		{
			name:      "openai with env var set",
			provider:  "openai",
			envVars:   map[string]string{"OPENAI_API_KEY": "sk-test"},
			wantEmpty: true,
		},
		{
			name:       "openai without env var",
			provider:   "openai",
			wantSubstr: "OPENAI_API_KEY",
		},
		{
			name:      "openai with base_url skips key check",
			provider:  "openai",
			baseURL:   "http://localhost:11434/v1",
			wantEmpty: true,
		},
		{
			name:      "anthropic with env var set",
			provider:  "anthropic",
			envVars:   map[string]string{"ANTHROPIC_API_KEY": "sk-ant-test"},
			wantEmpty: true,
		},
		{
			name:       "anthropic without env var",
			provider:   "anthropic",
			wantSubstr: "ANTHROPIC_API_KEY",
		},
		{
			name:      "bedrock with AWS_PROFILE set",
			provider:  "bedrock",
			envVars:   map[string]string{"AWS_PROFILE": "vocabgen"},
			wantEmpty: true,
		},
		{
			name:      "bedrock with AWS_ACCESS_KEY_ID set",
			provider:  "bedrock",
			envVars:   map[string]string{"AWS_ACCESS_KEY_ID": "AKIA..."},
			wantEmpty: true,
		},
		{
			name:       "vertexai with gcp_project form field",
			provider:   "vertexai",
			gcpProject: "my-project",
			wantEmpty:  true,
		},
		{
			name:      "vertexai with GCP_PROJECT env var",
			provider:  "vertexai",
			envVars:   map[string]string{"GCP_PROJECT": "my-project"},
			wantEmpty: true,
		},
		{
			name:       "vertexai without project",
			provider:   "vertexai",
			wantSubstr: "GCP project ID",
		},
		{
			name:      "unknown provider passes",
			provider:  "unknown",
			wantEmpty: true,
		},
	}

	envKeys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_ACCESS_KEY_ID", "AWS_PROFILE", "AWS_SESSION_TOKEN", "GCP_PROJECT"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			got := validateProviderEnv(tc.provider, tc.baseURL, tc.gcpProject)
			if tc.wantEmpty && got != "" {
				t.Fatalf("expected no warning, got %q", got)
			}
			if !tc.wantEmpty && got == "" {
				t.Fatal("expected a warning, got empty string")
			}
			if tc.wantSubstr != "" && !strings.Contains(got, tc.wantSubstr) {
				t.Fatalf("expected warning to contain %q, got %q", tc.wantSubstr, got)
			}
		})
	}
}

func TestPutConfig_EnvVarValidation(t *testing.T) {
	tests := []struct {
		name       string
		form       string
		envVars    map[string]string
		wantStatus int
		wantSubstr string // expected in response body
	}{
		{
			name:       "openai missing env var returns error",
			form:       "provider=openai&model_id=gpt-4o&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "OPENAI_API_KEY",
		},
		{
			name:       "openai with env var saves ok",
			form:       "provider=openai&model_id=gpt-4o&default_source_language=nl&default_target_language=hu",
			envVars:    map[string]string{"OPENAI_API_KEY": "sk-test"},
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "openai with base_url skips key check",
			form:       "provider=openai&model_id=llama3&base_url=http://localhost:11434/v1&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "anthropic missing env var returns error",
			form:       "provider=anthropic&model_id=claude-sonnet-4-20250514&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "ANTHROPIC_API_KEY",
		},
		{
			name:       "anthropic with env var saves ok",
			form:       "provider=anthropic&model_id=claude-sonnet-4-20250514&default_source_language=nl&default_target_language=hu",
			envVars:    map[string]string{"ANTHROPIC_API_KEY": "sk-ant-test"},
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "vertexai missing project returns error",
			form:       "provider=vertexai&model_id=gemini-pro&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "GCP project ID",
		},
		{
			name:       "vertexai with gcp_project field saves ok",
			form:       "provider=vertexai&model_id=gemini-pro&gcp_project=my-proj&default_source_language=nl&default_target_language=hu",
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
		{
			name:       "bedrock with AWS_PROFILE saves ok",
			form:       "provider=bedrock&aws_profile=vocabgen&aws_region=us-east-1&default_source_language=nl&default_target_language=hu",
			envVars:    map[string]string{"AWS_PROFILE": "vocabgen"},
			wantStatus: http.StatusOK,
			wantSubstr: "Configuration saved",
		},
	}

	envKeys := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "AWS_ACCESS_KEY_ID", "AWS_PROFILE", "AWS_SESSION_TOKEN", "GCP_PROJECT"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range envKeys {
				t.Setenv(k, "")
			}
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			srv := newTestServer()
			req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(tc.form))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d; body: %s", tc.wantStatus, w.Code, w.Body.String())
			}
			if tc.wantSubstr != "" && !strings.Contains(w.Body.String(), tc.wantSubstr) {
				t.Fatalf("expected body to contain %q, got %q", tc.wantSubstr, w.Body.String())
			}
		})
	}
}
