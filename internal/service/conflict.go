package service

import "fmt"

// ConflictStrategy represents the user's choice when a new LLM result
// conflicts with an existing database entry.
type ConflictStrategy string

// Supported conflict resolution strategies.
const (
	// ConflictReplace updates the existing entry in-place.
	ConflictReplace ConflictStrategy = "replace"
	// ConflictAdd inserts the new result as a separate entry alongside existing ones.
	ConflictAdd ConflictStrategy = "add"
	// ConflictSkip discards the new result and keeps the existing entry unchanged.
	ConflictSkip ConflictStrategy = "skip"
)

// ParseConflictStrategy converts a string to a ConflictStrategy.
// Returns an error for invalid values.
func ParseConflictStrategy(s string) (ConflictStrategy, error) {
	switch s {
	case "replace":
		return ConflictReplace, nil
	case "add":
		return ConflictAdd, nil
	case "skip":
		return ConflictSkip, nil
	default:
		return "", fmt.Errorf("invalid conflict strategy %q: must be \"replace\", \"add\", or \"skip\"", s)
	}
}
