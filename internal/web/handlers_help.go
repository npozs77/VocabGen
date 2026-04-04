package web

import (
	"html/template"
	"net/http"

	"github.com/user/vocabgen/docs"
	"github.com/user/vocabgen/internal/service"
)

// docsPageData extends pageData with documentation-specific fields.
type docsPageData struct {
	pageData
	Title   string
	Content template.HTML
}

// docsIndexData extends pageData with the rendered documentation index.
type docsIndexData struct {
	pageData
	Content template.HTML
	Docs    []docs.DocInfo
}

// handleDocsPage renders a single documentation page identified by slug.
func (s *Server) handleDocsPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	htmlContent, title, err := docs.Render(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data := docsPageData{
		pageData: pageData{
			ActivePage: "docs",
			Languages:  service.GetSupportedLanguages(),
			Config:     s.cfg,
			Version:    s.version,
			BuildDate:  s.buildDate,
			GoVersion:  s.goVersion,
		},
		Title:   title,
		Content: htmlContent,
	}
	if err := renderPage(w, "docs_page", data); err != nil {
		s.logger.Error("render page failed", "page", "docs_page", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// handleDocsIndex renders the documentation index page with the rendered README.
func (s *Server) handleDocsIndex(w http.ResponseWriter, r *http.Request) {
	indexHTML, err := docs.RenderIndex()
	if err != nil {
		s.logger.Error("render docs index failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	data := docsIndexData{
		pageData: pageData{
			ActivePage: "docs",
			Languages:  service.GetSupportedLanguages(),
			Config:     s.cfg,
			Version:    s.version,
			BuildDate:  s.buildDate,
			GoVersion:  s.goVersion,
		},
		Content: indexHTML,
		Docs:    docs.Available,
	}
	if err := renderPage(w, "docs", data); err != nil {
		s.logger.Error("render page failed", "page", "docs", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
