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
	if !ok || !containsAll(writing.Content,
		"final material a learner studies", "worked example",
		"Factor knowledge for reuse", "Review every created or modified unit",
		"complement rather than replace", "$...$", "$$...$$",
	) {
		t.Fatalf("writing documentation = %#v", writing)
	}
	units, unitsOK := Find("curriculum-units")
	dependencies, dependenciesOK := Find("dependencies")
	if !unitsOK || !containsAll(units.Content, "self-contained microlesson", "overlapping alternative", "one shared unit") {
		t.Fatalf("curriculum unit documentation = %#v", units)
	}
	if !dependenciesOK || !containsAll(dependencies.Content, "actual conceptual prerequisite", "explicit unit and dependency", "one shared prerequisite") {
		t.Fatalf("dependency documentation = %#v", dependencies)
	}
	if index := Index(); !strings.Contains(index, "curriculum://documentation/writing-content") {
		t.Fatalf("documentation index = %q", index)
	}
}

func containsAll(value string, fragments ...string) bool {
	value = strings.Join(strings.Fields(value), " ")
	for _, fragment := range fragments {
		if !strings.Contains(value, strings.Join(strings.Fields(fragment), " ")) {
			return false
		}
	}
	return true
}
