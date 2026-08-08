package guidance

import (
	"strings"
	"testing"
)

func TestCatalogLoadsCanonicalDocumentation(t *testing.T) {
	pages := Pages()
	if len(pages) != 7 {
		t.Fatalf("page count = %d, want 7", len(pages))
	}
	writing, ok := Find("writing-content")
	if !ok || !strings.Contains(writing.Content, "$...$") || !strings.Contains(writing.Content, "$$...$$") {
		t.Fatalf("writing documentation = %#v", writing)
	}
	if index := Index(); !strings.Contains(index, "curriculum://documentation/writing-content") {
		t.Fatalf("documentation index = %q", index)
	}
}
