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

func init() {
	templates = make(map[string]*template.Template)

	// Parse page templates — each page combines base.html + its own template.
	pages := []string{"lookup", "batch", "config", "database"}
	for _, page := range pages {
		t, err := template.ParseFS(templateFS,
			"templates/base.html",
			fmt.Sprintf("templates/%s.html", page),
		)
		if err != nil {
			panic(fmt.Sprintf("parse template %s: %v", page, err))
		}
		templates[page] = t
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
