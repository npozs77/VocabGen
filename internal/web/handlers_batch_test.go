package web

import (
	"testing"

	"github.com/user/vocabgen/internal/parsing"
)

func TestParseTextList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []parsing.TokenWithContext
		wantErr string
	}{
		{
			name:  "multi-line input",
			input: "eraan toe zijn\nerachter komen\neraan gaan",
			want: []parsing.TokenWithContext{
				{Token: "eraan toe zijn"},
				{Token: "erachter komen"},
				{Token: "eraan gaan"},
			},
		},
		{
			name:  "blank lines skipped",
			input: "hello\n\n\nworld\n",
			want: []parsing.TokenWithContext{
				{Token: "hello"},
				{Token: "world"},
			},
		},
		{
			name:  "whitespace trimmed",
			input: "  hello  \n  world  ",
			want: []parsing.TokenWithContext{
				{Token: "hello"},
				{Token: "world"},
			},
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: "word list is empty",
		},
		{
			name:    "only whitespace and blank lines",
			input:   "  \n  \n\n  ",
			wantErr: "word list is empty",
		},
		{
			name:  "commas preserved in token",
			input: "huis, Ik woon in een groot huis\nboek",
			want: []parsing.TokenWithContext{
				{Token: "huis, Ik woon in een groot huis"},
				{Token: "boek"},
			},
		},
		{
			name:  "commas in conjugation preserved",
			input: "doorkrijgen (kreeg door, heeft doorgekregen) (sep.)",
			want: []parsing.TokenWithContext{
				{Token: "doorkrijgen (kreeg door, heeft doorgekregen) (sep.)"},
			},
		},
		{
			name:  "windows line endings",
			input: "hello\r\nworld\r\n",
			want: []parsing.TokenWithContext{
				{Token: "hello"},
				{Token: "world"},
			},
		},
		{
			name:  "single token",
			input: "huis",
			want: []parsing.TokenWithContext{
				{Token: "huis"},
			},
		},
		{
			name:  "comma only line kept as token",
			input: ", some context",
			want: []parsing.TokenWithContext{
				{Token: ", some context"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTextList(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d tokens, got %d", len(tc.want), len(got))
			}
			for i, w := range tc.want {
				if got[i].Token != w.Token {
					t.Errorf("[%d] token: expected %q, got %q", i, w.Token, got[i].Token)
				}
				if got[i].Context != w.Context {
					t.Errorf("[%d] context: expected %q, got %q", i, w.Context, got[i].Context)
				}
			}
		})
	}
}
