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
	case "about", "docs", "docs_page", "update":
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

	// Parse page templates — each page combines base.html + its own template.
	pages := []string{"lookup", "batch", "config", "database", "about", "docs", "update"}
	for _, page := range pages {
		t, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/base.html",
			fmt.Sprintf("templates/%s.html", page),
		)
		if err != nil {
			panic(fmt.Sprintf("parse template %s: %v", page, err))
		}
		templates[page] = t
	}

	// Parse docs_page separately (also uses base.html).
	{
		t, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/base.html",
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
		"config_form", "entry_edit", "entry_table",
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
