package parsing

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// TokenWithContext pairs a raw token with its optional context sentence.
type TokenWithContext struct {
	Token   string
	Context string
}

// ReadInputFile reads a CSV file and returns (token, context) pairs.
// Skips empty/whitespace-only lines. All non-empty lines are treated as data.
// Parenthetical conjugation annotations like "(woog af, heeft afgewogen)" are
// stripped before CSV parsing so their internal commas don't cause field splits.
// Returns error for file not found or empty file (after skipping blanks).
func ReadInputFile(path string) ([]TokenWithContext, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open input file: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Pre-process: strip conjugation parentheticals from each line before
	// CSV parsing so commas inside e.g. "(greep in, heeft ingrepen)" don't
	// cause spurious field splits.
	var cleaned strings.Builder
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := conjugationInfo.ReplaceAllString(scanner.Text(), " ")
		_, _ = cleaned.WriteString(line)
		_, _ = cleaned.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading input file: %w", err)
	}

	reader := csv.NewReader(strings.NewReader(cleaned.String()))
	reader.FieldsPerRecord = -1 // variable number of fields
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	var results []TokenWithContext

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV read error: %w", err)
		}

		// Skip empty/whitespace-only lines
		if len(record) == 0 {
			continue
		}
		token := strings.TrimSpace(record[0])
		if token == "" {
			// Check if all columns are whitespace-only
			allEmpty := true
			for _, col := range record {
				if strings.TrimSpace(col) != "" {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				continue
			}
		}
		if token == "" {
			continue
		}

		tc := TokenWithContext{Token: token}
		if len(record) >= 2 {
			tc.Context = strings.TrimSpace(record[1])
		}
		results = append(results, tc)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("input file is empty (no non-blank lines)")
	}

	return results, nil
}
