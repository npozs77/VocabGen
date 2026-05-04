package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

// templates holds all parsed templates, keyed by page name.
var templates map[string]*template.Template

// IsHelpPage reports whether the given page name belongs to the Help menu section.
func IsHelpPage(page string) bool {
	switch page {
	case "about", "docs", "docs_page", "update", "changelog":
		return true
	}
	return false
}

// funcMap provides template helper functions available to all page templates.
var funcMap = template.FuncMap{
	"isHelpPage": IsHelpPage,
}

func init() {
	templates = make(map[string]*template.Template)

	// Pages that include the tag_picker partial in addition to base + profile_switcher.
	tagPickerPages := map[string]bool{
		"lookup": true, "batch": true, "database": true, "flashcards": true,
	}

	// Parse page templates — each page combines base.html + profile_switcher partial + its own template.
	// Pages that use the tag picker also include the tag_picker partial.
	pages := []string{"lookup", "batch", "config", "database", "flashcards", "about", "docs", "update", "changelog"}
	for _, page := range pages {
		files := []string{
			"templates/base.html",
			"templates/partials/profile_switcher.html",
			fmt.Sprintf("templates/%s.html", page),
		}
		if tagPickerPages[page] {
			files = append(files, "templates/partials/tag_picker.html")
		}
		t, err := template.New("").Funcs(funcMap).ParseFS(templateFS, files...)
		if err != nil {
			panic(fmt.Sprintf("parse template %s: %v", page, err))
		}
		templates[page] = t
	}

	// Parse docs_page separately (also uses base.html + profile_switcher).
	{
		t, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/base.html",
			"templates/partials/profile_switcher.html",
			"templates/docs_page.html",
		)
		if err != nil {
			panic(fmt.Sprintf("parse template docs_page: %v", err))
		}
		templates["docs_page"] = t
	}

	// Parse partial templates individually.
	partials := []string{
		"lookup_result", "lookup_conflict", "batch_summary",
		"entry_edit", "entry_table", "update_result",
		"setup_local_llm", "profile_switcher", "flashcard_card",
	}
	for _, partial := range partials {
		t, err := template.ParseFS(templateFS,
			fmt.Sprintf("templates/partials/%s.html", partial),
		)
		if err != nil {
			panic(fmt.Sprintf("parse partial %s: %v", partial, err))
		}
		templates[partial] = t
	}

	// config_form includes setup_local_llm, so parse them together.
	{
		t, err := template.ParseFS(templateFS,
			"templates/partials/config_form.html",
			"templates/partials/setup_local_llm.html",
		)
		if err != nil {
			panic(fmt.Sprintf("parse partial config_form: %v", err))
		}
		templates["config_form"] = t
	}
}

// renderPage renders a full page template (base + page content) to the response.
func renderPage(w http.ResponseWriter, page string, data any) error {
	t, ok := templates[page]
	if !ok {
		return fmt.Errorf("template %q not found", page)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.ExecuteTemplate(w, "base", data)
}

// renderPartial renders a named partial template to the response.
func renderPartial(w http.ResponseWriter, name string, data any) error {
	t, ok := templates[name]
	if !ok {
		return fmt.Errorf("partial template %q not found", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.ExecuteTemplate(w, name, data)
}
