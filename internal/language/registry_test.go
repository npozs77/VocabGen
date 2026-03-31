package language

import "testing"

// TestResolveLanguageName verifies that known codes resolve to full names
// and unknown codes/names pass through unchanged.
//
// Validates: Requirements 6.2, 6.3, 43.6
func TestResolveLanguageName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"nl→Dutch", "nl", "Dutch"},
		{"hu→Hungarian", "hu", "Hungarian"},
		{"it→Italian", "it", "Italian"},
		{"ru→Russian", "ru", "Russian"},
		{"en→English", "en", "English"},
		{"de→German", "de", "German"},
		{"fr→French", "fr", "French"},
		{"es→Spanish", "es", "Spanish"},
		{"pt→Portuguese", "pt", "Portuguese"},
		{"pl→Polish", "pl", "Polish"},
		{"tr→Turkish", "tr", "Turkish"},
		{"unknown code passes through", "xx", "xx"},
		{"full name passes through", "German", "German"},
		{"non-Latin name passes through", "日本語", "日本語"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveLanguageName(tc.input)
			if got != tc.want {
				t.Errorf("ResolveLanguageName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
