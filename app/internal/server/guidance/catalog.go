package guidance

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed pages/*.md
var files embed.FS

type Page struct {
	Slug    string
	Title   string
	Summary string
	Content string
}

var catalog = []Page{
	{Slug: "curriculum-units", Title: "Curriculum units", Summary: "Scope and granularity of a learnable unit"},
	{Slug: "dependencies", Title: "Dependencies", Summary: "How actual prerequisites structure the curriculum"},
	{Slug: "writing-content", Title: "Writing content", Summary: "Clear educational Markdown with optional LaTeX"},
	{Slug: "proposals", Title: "Proposals", Summary: "Describe the intended curriculum change"},
	{Slug: "recognitions", Title: "Recognitions", Summary: "Preserve progress as the curriculum changes"},
	{Slug: "learning-paths", Title: "Learning paths", Summary: "Private goals over the shared curriculum"},
	{Slug: "mcp-api", Title: "MCP and API", Summary: "Inspect and modify the curriculum programmatically"},
}

func Pages() []Page {
	pages := make([]Page, len(catalog))
	for index, entry := range catalog {
		content, err := files.ReadFile("pages/" + entry.Slug + ".md")
		if err != nil {
			panic(fmt.Sprintf("read embedded documentation %q: %v", entry.Slug, err))
		}
		entry.Content = strings.TrimSpace(string(content))
		pages[index] = entry
	}
	return pages
}

func Find(slug string) (Page, bool) {
	for _, page := range Pages() {
		if page.Slug == slug {
			return page, true
		}
	}
	return Page{}, false
}

func Index() string {
	var output strings.Builder
	output.WriteString("# Universal Curriculum documentation\n\n")
	for _, page := range Pages() {
		fmt.Fprintf(&output, "- [%s](curriculum://documentation/%s) — %s\n", page.Title, page.Slug, page.Summary)
	}
	return output.String()
}
