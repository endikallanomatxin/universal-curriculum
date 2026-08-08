package views

import (
	"strings"
	"testing"
)

func TestRenderUnitContentRendersMarkdownAndKeepsMathDelimiters(t *testing.T) {
	rendered := string(RenderUnitContent("# Heading\n\nUse **energy**: $E = mc^2$.\n"))

	for _, expected := range []string{"<h1>Heading</h1>", "<strong>energy</strong>", "$E = mc^2$"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered content does not contain %q: %s", expected, rendered)
		}
	}
}

func TestRenderUnitContentRejectsRawHTML(t *testing.T) {
	rendered := string(RenderUnitContent(`<script>alert("xss")</script>

[unsafe](javascript:alert(1))`))

	if strings.Contains(rendered, "<script>") || strings.Contains(rendered, `href="javascript:`) {
		t.Fatalf("unsafe HTML survived Markdown rendering: %s", rendered)
	}
}

func TestRenderUnitContentIsolatesIframes(t *testing.T) {
	rendered := string(RenderUnitContent(`<iframe title="Counter" srcdoc="&lt;style&gt;button{color:red}&lt;/style&gt;&lt;button onclick=&quot;this.textContent='Done'&quot;&gt;Try&lt;/button&gt;"></iframe>`))

	for _, expected := range []string{
		`class="unit-visualization"`,
		`sandbox="allow-scripts"`,
		`title="Counter"`,
		`default-src &#39;none&#39;`,
		`onclick=&#34;this.textContent=&#39;Done&#39;&#34;`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("visualization does not contain %q: %s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "<button") {
		t.Fatalf("visualization escaped its iframe: %s", rendered)
	}
}

func TestRenderUnitContentAllowsYouTubeEmbed(t *testing.T) {
	rendered := string(RenderUnitContent(`<iframe title="A useful explanation" src="https://www.youtube-nocookie.com/embed/aircAruvnKk"></iframe>`))

	for _, expected := range []string{
		`class="unit-visualization unit-visualization--video"`,
		`src="https://www.youtube-nocookie.com/embed/aircAruvnKk"`,
		`sandbox="allow-scripts allow-same-origin allow-presentation"`,
		`allowfullscreen`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("YouTube embed does not contain %q: %s", expected, rendered)
		}
	}
}

func TestRenderUnitContentRejectsUnsupportedIframeSource(t *testing.T) {
	rendered := string(RenderUnitContent(`<iframe src="https://example.com"></iframe>`))

	if strings.Contains(rendered, "https://example.com") || !strings.Contains(rendered, "source is not supported") {
		t.Fatalf("external iframe was not rejected: %s", rendered)
	}
}
