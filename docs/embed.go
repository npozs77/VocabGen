package docs

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

//go:embed *.md
var content embed.FS

// DocInfo holds metadata for a documentation page.
type DocInfo struct {
	Slug  string
	Title string
	File  string
}

// Available is the list of available documentation pages.
var Available = []DocInfo{
	{Slug: "architecture", Title: "Architecture", File: "architecture.md"},
	{Slug: "deployment", Title: "Deployment", File: "deployment.md"},
	{Slug: "user-guide", Title: "User Guide", File: "user-guide.md"},
	{Slug: "changelog", Title: "Changelog", File: "changelog.md"},
}

var slugToFile map[string]DocInfo

func init() {
	slugToFile = make(map[string]DocInfo, len(Available))
	for _, d := range Available {
		slugToFile[d.Slug] = d
	}
}

// RenderIndex renders the README.md documentation index page.
func RenderIndex() (template.HTML, error) {
	src, err := content.ReadFile("README.md")
	if err != nil {
		return "", fmt.Errorf("read README.md: %w", err)
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", fmt.Errorf("render README.md: %w", err)
	}
	return template.HTML(buf.String()), nil
}

// Render reads the markdown file for the given slug and returns rendered HTML and the doc title.
func Render(slug string) (template.HTML, string, error) {
	info, ok := slugToFile[slug]
	if !ok {
		return "", "", fmt.Errorf("unknown doc: %s", slug)
	}
	src, err := content.ReadFile(info.File)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", info.File, err)
	}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", "", fmt.Errorf("render %s: %w", info.File, err)
	}
	return template.HTML(buf.String()), info.Title, nil
}
