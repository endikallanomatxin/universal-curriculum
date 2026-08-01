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

func TestRenderRenderedContentDiffRendersOnlyChangedBlocksTwice(t *testing.T) {
	output := string(RenderRenderedContentDiff(
		"Shared introduction.\n\nThe old **formula** is $x$.\n\nShared ending.",
		"Shared introduction.\n\nThe new **formula** is $y$.\n\nShared ending.",
	))

	for _, fragment := range []string{
		`class="rendered-content-diff__unchanged"`,
		`rendered-content-diff__version--before`,
		`<p>The old <strong>formula</strong> is $x$.</p>`,
		`rendered-content-diff__version--after`,
		`<p>The new <strong>formula</strong> is $y$.</p>`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered content diff does not contain %q: %s", fragment, output)
		}
	}
	if strings.Count(output, "Shared introduction.") != 1 || strings.Count(output, "Shared ending.") != 1 {
		t.Fatalf("unchanged blocks should be rendered once: %s", output)
	}
}

func TestMarkdownContentBlocksKeepsFencedCodeTogether(t *testing.T) {
	blocks := markdownContentBlocks("Before.\n\n```go\nfirst()\n\nsecond()\n```\n\nAfter.")
	if len(blocks) != 3 || !strings.Contains(blocks[1], "first()\n\nsecond()") {
		t.Fatalf("markdownContentBlocks() = %#v", blocks)
	}
}
