package services

import (
	"strings"
	"testing"
)

func TestRenderContentDiffHighlightsWordChangesAndEscapesContent(t *testing.T) {
	output := string(RenderContentDiff(
		"The <quick> brown fox.",
		"The <quick> blue fox.",
	))

	for _, fragment := range []string{
		"The &lt;quick&gt; ",
		"<del>brown</del>",
		"<ins>blue</ins>",
		" fox.",
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered content diff does not contain %q: %s", fragment, output)
		}
	}
	if strings.Contains(output, "<quick>") {
		t.Fatal("rendered content diff contains unescaped source HTML")
	}
}
