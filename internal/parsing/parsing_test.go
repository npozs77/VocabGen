package parsing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// --- Property Test P6: CSV parsing ---

// TestPropertyP6CSVParsing verifies that ReadInputFile correctly parses CSV
// files with a mix of empty lines, single-column data, and two-column data.
// The returned count equals the non-empty line count; two-column lines have
// both token and context; single-column lines have empty context.
//
// Validates: Requirements 14.2, 14.5, 14.6
func TestPropertyP6CSVParsing(t *testing.T) {
	dir := t.TempDir()
	counter := 0

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a list of CSV line types
		type lineSpec struct {
			kind    string // "empty", "single", "double"
			token   string
			context string
		}

		numLines := rapid.IntRange(1, 30).Draw(rt, "numLines")
		lines := make([]lineSpec, numLines)
		var expectedCount int

		for i := range lines {
			kind := rapid.SampledFrom([]string{"empty", "single", "double"}).Draw(rt, "kind")
			switch kind {
			case "empty":
				lines[i] = lineSpec{kind: "empty"}
			case "single":
				token := rapid.StringMatching(`[A-Za-z\x{00C0}-\x{024F}]{1,20}`).Draw(rt, "token")
				lines[i] = lineSpec{kind: "single", token: token}
				expectedCount++
			case "double":
				token := rapid.StringMatching(`[A-Za-z\x{00C0}-\x{024F}]{1,20}`).Draw(rt, "token")
				ctx := rapid.StringMatching(`[A-Za-z ]{1,40}`).Draw(rt, "context")
				lines[i] = lineSpec{kind: "double", token: token, context: ctx}
				expectedCount++
			}
		}

		// Need at least one non-empty line for a valid file
		if expectedCount == 0 {
			return
		}

		// Build CSV content
		var sb strings.Builder
		var expectedTokens []lineSpec
		for _, l := range lines {
			switch l.kind {
			case "empty":
				sb.WriteString("\n")
			case "single":
				sb.WriteString(l.token + "\n")
				expectedTokens = append(expectedTokens, l)
			case "double":
				sb.WriteString(l.token + "," + l.context + "\n")
				expectedTokens = append(expectedTokens, l)
			}
		}

		// Write to temp file with unique name per iteration
		counter++
		path := filepath.Join(dir, fmt.Sprintf("input_%d.csv", counter))
		if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
			rt.Fatalf("failed to write temp file: %v", err)
		}

		results, err := ReadInputFile(path)
		if err != nil {
			rt.Fatalf("ReadInputFile returned error: %v", err)
		}

		if len(results) != expectedCount {
			rt.Fatalf("expected %d results, got %d", expectedCount, len(results))
		}

		for i, exp := range expectedTokens {
			got := results[i]
			if got.Token != exp.token {
				rt.Errorf("result[%d].Token = %q, want %q", i, got.Token, exp.token)
			}
			switch exp.kind {
			case "single":
				if got.Context != "" {
					rt.Errorf("result[%d].Context = %q, want empty for single-column", i, got.Context)
				}
			case "double":
				if got.Context != strings.TrimSpace(exp.context) {
					rt.Errorf("result[%d].Context = %q, want %q", i, got.Context, strings.TrimSpace(exp.context))
				}
			}
		}
	})
}

// --- Property Test P10: Token normalization idempotence ---

// TestPropertyP10TokenNormalizationIdempotence verifies that applying
// NormalizeWord or NormalizeExpression twice produces the same result as
// applying it once, for any random input string.
//
// Validates: Requirements 15.1–15.4, 16.1–16.3
func TestPropertyP10TokenNormalizationIdempotence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate strings with quotes, spaces, parentheses mixed in
		raw := rapid.StringMatching(`[A-Za-z"' \(\)\x{00C0}-\x{024F}]{0,50}`).Draw(rt, "raw")

		// NormalizeWord idempotence
		once := NormalizeWord(raw)
		twice := NormalizeWord(once)
		if once != twice {
			rt.Errorf("NormalizeWord not idempotent: NormalizeWord(%q) = %q, NormalizeWord(%q) = %q",
				raw, once, once, twice)
		}

		// NormalizeExpression idempotence
		once = NormalizeExpression(raw)
		twice = NormalizeExpression(once)
		if once != twice {
			rt.Errorf("NormalizeExpression not idempotent: NormalizeExpression(%q) = %q, NormalizeExpression(%q) = %q",
				raw, once, once, twice)
		}
	})
}

// --- Table-driven tests for parsing edge cases ---

// TestReadInputFileErrors tests error conditions for ReadInputFile.
//
// Validates: Requirements 14.3, 14.4
func TestReadInputFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(dir string) string // returns file path
		wantErr string
	}{
		{
			name: "file not found",
			setup: func(dir string) string {
				return filepath.Join(dir, "nonexistent.csv")
			},
			wantErr: "cannot open input file",
		},
		{
			name: "empty file",
			setup: func(dir string) string {
				p := filepath.Join(dir, "empty.csv")
				_ = os.WriteFile(p, []byte(""), 0644)
				return p
			},
			wantErr: "input file is empty",
		},
		{
			name: "whitespace-only lines",
			setup: func(dir string) string {
				p := filepath.Join(dir, "blanks.csv")
				_ = os.WriteFile(p, []byte("  \n\t\n   \n"), 0644)
				return p
			},
			wantErr: "input file is empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tc.setup(dir)
			_, err := ReadInputFile(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestReadInputFileSkipsWhitespaceLines verifies that whitespace-only lines
// are skipped and only data lines are returned.
//
// Validates: Requirement 14.2
func TestReadInputFileSkipsWhitespaceLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "mixed.csv")
	content := "hello\n  \nworld\n\t\nfoo,bar\n"
	_ = os.WriteFile(p, []byte(content), 0644)

	results, err := ReadInputFile(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Token != "hello" || results[0].Context != "" {
		t.Errorf("result[0] = %+v, want {Token:hello Context:}", results[0])
	}
	if results[1].Token != "world" || results[1].Context != "" {
		t.Errorf("result[1] = %+v, want {Token:world Context:}", results[1])
	}
	if results[2].Token != "foo" || results[2].Context != "bar" {
		t.Errorf("result[2] = %+v, want {Token:foo Context:bar}", results[2])
	}
}

// TestNormalizeWordEdgeCases tests specific normalization behaviors.
//
// Validates: Requirements 15.1–15.4
func TestNormalizeWordEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "nested parentheses preserved",
			input: `werk (werken (gewerkt))`,
			want:  "werk (werken (gewerkt))",
		},
		{
			name:  "mixed quotes removed",
			input: `"hello" 'world'`,
			want:  "hello world",
		},
		{
			name:  "whitespace-only returns empty",
			input: "   \t  ",
			want:  "",
		},
		{
			name:  "multiple spaces collapsed",
			input: "hello    world",
			want:  "hello world",
		},
		{
			name:  "leading and trailing whitespace stripped",
			input: "  hello  ",
			want:  "hello",
		},
		{
			name:  "parenthetical inflection preserved",
			input: "lopen (liep, gelopen)",
			want:  "lopen (liep, gelopen)",
		},
		{
			name:  "curly quotes removed",
			input: "\u201chello\u201d \u2018world\u2019",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeWord(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeWord(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestNormalizeExpressionEdgeCases tests specific normalization behaviors.
//
// Validates: Requirements 16.1–16.3
func TestNormalizeExpressionEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "quotes removed",
			input: `"in de war"`,
			want:  "in de war",
		},
		{
			name:  "multiple spaces collapsed",
			input: "op   de   hoogte",
			want:  "op de hoogte",
		},
		{
			name:  "whitespace-only returns empty",
			input: "   ",
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeExpression(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeExpression(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// --- Fuzz Tests for NormalizeWord and NormalizeExpression ---

// FuzzNormalizeWord fuzzes NormalizeWord with random strings to find panics.
// Also verifies idempotence: applying twice gives the same result as once.
//
// Validates: Requirements 43.3
func FuzzNormalizeWord(f *testing.F) {
	// Seed corpus: empty string, quotes, parentheses, Unicode, mixed whitespace
	f.Add("")
	f.Add(`"hello"`)
	f.Add(`'world'`)
	f.Add("\u201Ccurly\u201D")
	f.Add("\u2018single\u2019")
	f.Add("\u00ABguillemets\u00BB")
	f.Add("lopen (liep, gelopen)")
	f.Add("(nested (parens))")
	f.Add("ëüőű")
	f.Add("café résumé naïve")
	f.Add("   \t  \t  ")
	f.Add("hello    world")
	f.Add("  leading and trailing  ")
	f.Add("mixed\t  spaces\t\ttabs")
	f.Add(`"quoted 'nested' quotes"`)

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic
		once := NormalizeWord(input)

		// Idempotence: applying twice gives the same result as once.
		// Only check for valid UTF-8 — invalid byte sequences can form new
		// quote characters after stripping, which is expected behavior.
		if utf8.ValidString(input) {
			twice := NormalizeWord(once)
			if once != twice {
				t.Errorf("NormalizeWord not idempotent: NormalizeWord(%q) = %q, NormalizeWord(%q) = %q",
					input, once, once, twice)
			}
		}
	})
}

// FuzzNormalizeExpression fuzzes NormalizeExpression with random strings to find panics.
// Also verifies idempotence: applying twice gives the same result as once.
//
// Validates: Requirements 43.3
func FuzzNormalizeExpression(f *testing.F) {
	// Seed corpus: empty string, quotes, parentheses, Unicode, mixed whitespace
	f.Add("")
	f.Add(`"in de war"`)
	f.Add(`'op de hoogte'`)
	f.Add("\u201Ccurly\u201D")
	f.Add("\u2018single\u2019")
	f.Add("\u00ABguillemets\u00BB")
	f.Add("(with parens)")
	f.Add("ëüőű")
	f.Add("café résumé naïve")
	f.Add("   \t  \t  ")
	f.Add("op   de   hoogte")
	f.Add("  leading and trailing  ")
	f.Add("mixed\t  spaces\t\ttabs")
	f.Add(`"quoted 'nested' quotes"`)

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic
		once := NormalizeExpression(input)

		// Idempotence: applying twice gives the same result as once.
		// Only check for valid UTF-8 — invalid byte sequences can form new
		// quote characters after stripping, which is expected behavior.
		if utf8.ValidString(input) {
			twice := NormalizeExpression(once)
			if once != twice {
				t.Errorf("NormalizeExpression not idempotent: NormalizeExpression(%q) = %q, NormalizeExpression(%q) = %q",
					input, once, once, twice)
			}
		}
	})
}
