package web

import (
	"testing"

	"pgregory.net/rapid"
)

// TestIsHelpPage_TrueForHelpPages verifies that IsHelpPage returns true
// for all pages that belong to the Help menu section.
//
// Validates: Requirements 1.4, 2.3, 4.5, 5.3
func TestIsHelpPage_TrueForHelpPages(t *testing.T) {
	tests := []struct {
		page string
	}{
		{"about"},
		{"docs"},
		{"docs_page"},
		{"update"},
		{"changelog"},
	}
	for _, tc := range tests {
		t.Run(tc.page, func(t *testing.T) {
			if !IsHelpPage(tc.page) {
				t.Errorf("IsHelpPage(%q) = false, want true", tc.page)
			}
		})
	}
}

// TestIsHelpPage_FalseForNonHelpPages verifies that IsHelpPage returns false
// for pages that do not belong to the Help menu section.
//
// Validates: Requirements 1.4, 2.3, 4.5, 5.3
func TestIsHelpPage_FalseForNonHelpPages(t *testing.T) {
	tests := []struct {
		name string
		page string
	}{
		{"lookup", "lookup"},
		{"batch", "batch"},
		{"config", "config"},
		{"database", "database"},
		{"empty", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if IsHelpPage(tc.page) {
				t.Errorf("IsHelpPage(%q) = true, want false", tc.page)
			}
		})
	}
}

// helpPages is the canonical set of pages that belong to the Help menu.
var helpPages = map[string]bool{
	"about":     true,
	"docs":      true,
	"docs_page": true,
	"update":    true,
	"changelog": true,
}

// TestPropertyP14_ActiveHelpSubpageHighlightsHelpMenu verifies that for any
// arbitrary page string, IsHelpPage returns true if and only if the page is
// one of the known help pages. This ensures the Help menu item is highlighted
// exactly when a Help subpage is active.
//
// Property P1.4: Active Help subpage highlights the Help menu item
// Validates: Requirements 1.4, 2.3, 4.5, 5.3
func TestPropertyP14_ActiveHelpSubpageHighlightsHelpMenu(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		page := rapid.String().Draw(t, "page")
		got := IsHelpPage(page)
		want := helpPages[page]
		if got != want {
			t.Errorf("IsHelpPage(%q) = %v, want %v", page, got, want)
		}
	})
}
