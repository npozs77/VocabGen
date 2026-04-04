package docs

import (
	"testing"

	"pgregory.net/rapid"
)

// TestPropertyP4_2_RenderSlugValidation verifies that each documentation subpage
// renders content from the corresponding docs/*.md file, and unknown slugs return errors.
//
// **Validates: Requirements 4.3, 4.4**
func TestPropertyP4_2_RenderSlugValidation(t *testing.T) {
	// Build set of known slugs for quick lookup.
	knownSlugs := make(map[string]DocInfo, len(Available))
	for _, d := range Available {
		knownSlugs[d.Slug] = d
	}

	// Sub-test: random strings that are NOT known slugs must return an error.
	t.Run("unknown_slugs_return_error", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			slug := rapid.StringMatching(`[a-z0-9\-]{1,30}`).Draw(rt, "slug")

			// Skip if the random string happens to be a known slug.
			if _, ok := knownSlugs[slug]; ok {
				rt.Skip("generated a known slug, skipping")
			}

			html, title, err := Render(slug)
			if err == nil {
				rt.Fatalf("expected error for unknown slug %q, got html=%q title=%q", slug, html, title)
			}
			if html != "" {
				rt.Fatalf("expected empty HTML for unknown slug %q, got %q", slug, html)
			}
			if title != "" {
				rt.Fatalf("expected empty title for unknown slug %q, got %q", slug, title)
			}
		})
	})

	// Sub-test: every known slug returns non-empty HTML and the correct title.
	t.Run("known_slugs_return_content", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			idx := rapid.IntRange(0, len(Available)-1).Draw(rt, "index")
			info := Available[idx]

			html, title, err := Render(info.Slug)
			if err != nil {
				rt.Fatalf("Render(%q) returned error: %v", info.Slug, err)
			}
			if html == "" {
				rt.Fatalf("Render(%q) returned empty HTML", info.Slug)
			}
			if title != info.Title {
				rt.Fatalf("Render(%q) title = %q, want %q", info.Slug, title, info.Title)
			}
		})
	})
}

// TestRenderKnownSlugs verifies that each known slug returns non-empty HTML and the correct title.
//
// _Requirements: 4.2, 4.3, 4.4_
func TestRenderKnownSlugs(t *testing.T) {
	tests := []struct {
		slug      string
		wantTitle string
	}{
		{slug: "architecture", wantTitle: "Architecture"},
		{slug: "deployment", wantTitle: "Deployment"},
		{slug: "user-guide", wantTitle: "User Guide"},
		{slug: "changelog", wantTitle: "Changelog"},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			html, title, err := Render(tt.slug)
			if err != nil {
				t.Fatalf("Render(%q) returned error: %v", tt.slug, err)
			}
			if html == "" {
				t.Errorf("Render(%q) returned empty HTML", tt.slug)
			}
			if title != tt.wantTitle {
				t.Errorf("Render(%q) title = %q, want %q", tt.slug, title, tt.wantTitle)
			}
		})
	}
}

// TestRenderUnknownSlugs verifies that unknown slugs return an error.
//
// _Requirements: 4.3, 4.4_
func TestRenderUnknownSlugs(t *testing.T) {
	tests := []struct {
		slug string
	}{
		{slug: "nonexistent"},
		{slug: ""},
		{slug: "ARCHITECTURE"},
		{slug: "deploy"},
		{slug: "user_guide"},
	}

	for _, tt := range tests {
		name := tt.slug
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			html, title, err := Render(tt.slug)
			if err == nil {
				t.Fatalf("Render(%q) expected error, got html=%q title=%q", tt.slug, html, title)
			}
			if html != "" {
				t.Errorf("Render(%q) expected empty HTML on error, got %q", tt.slug, html)
			}
			if title != "" {
				t.Errorf("Render(%q) expected empty title on error, got %q", tt.slug, title)
			}
		})
	}
}

// TestAvailableEntries verifies that Available has exactly 4 entries with the expected slugs.
//
// _Requirements: 4.2_
func TestAvailableEntries(t *testing.T) {
	expectedSlugs := []string{"architecture", "deployment", "user-guide", "changelog"}

	if len(Available) != len(expectedSlugs) {
		t.Fatalf("Available has %d entries, want %d", len(Available), len(expectedSlugs))
	}

	for i, want := range expectedSlugs {
		if Available[i].Slug != want {
			t.Errorf("Available[%d].Slug = %q, want %q", i, Available[i].Slug, want)
		}
		if Available[i].Title == "" {
			t.Errorf("Available[%d].Title is empty for slug %q", i, want)
		}
		if Available[i].File == "" {
			t.Errorf("Available[%d].File is empty for slug %q", i, want)
		}
	}
}
