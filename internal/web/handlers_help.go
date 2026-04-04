package web

import (
	"html/template"
	"net/http"
	"runtime"

	"github.com/user/vocabgen/docs"
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
		pageData: s.newPageData("docs"),
		Title:    title,
		Content:  htmlContent,
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
		pageData: s.newPageData("docs"),
		Content:  indexHTML,
		Docs:     docs.Available,
	}
	if err := renderPage(w, "docs", data); err != nil {
		s.logger.Error("render page failed", "page", "docs", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// updatePageData extends pageData with update-specific fields.
type updatePageData struct {
	pageData
	OS   string
	Arch string
}

// handleUpdatePage renders the update page with current version and OS/arch info.
func (s *Server) handleUpdatePage(w http.ResponseWriter, r *http.Request) {
	data := updatePageData{
		pageData: s.newPageData("update"),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
	if err := renderPage(w, "update", data); err != nil {
		s.logger.Error("render page failed", "page", "update", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// handleUpdateCheck performs a fresh update check (or returns cached) and renders the result partial.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	info := s.updater.checkNow(r.Context())
	if err := renderPartial(w, "update_result", info); err != nil {
		s.logger.Error("render partial failed", "partial", "update_result", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// handleUpdateDismiss dismisses the update banner for this server session.
func (s *Server) handleUpdateDismiss(w http.ResponseWriter, _ *http.Request) {
	s.updater.dismiss()
	w.WriteHeader(http.StatusOK)
}

// handleChangelog renders the changelog page with the embedded CHANGELOG.md.
func (s *Server) handleChangelog(w http.ResponseWriter, r *http.Request) {
	htmlContent, _, err := docs.Render("changelog")
	if err != nil {
		s.logger.Error("render changelog failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	data := docsPageData{
		pageData: s.newPageData("changelog"),
		Title:    "Changelog",
		Content:  htmlContent,
	}
	if err := renderPage(w, "changelog", data); err != nil {
		s.logger.Error("render page failed", "page", "changelog", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
