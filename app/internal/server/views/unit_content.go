package views

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	unitContentMarkdown = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)
	unitContentIframe = regexp.MustCompile(`(?is)<iframe\b([^>]*)>\s*</iframe>`)
	iframeAttribute   = regexp.MustCompile(`(?is)([a-z][a-z0-9:_-]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

func RenderUnitContent(source string) template.HTML {
	markdown, embeds := extractUnitContentIframes(source)
	var rendered bytes.Buffer
	if err := unitContentMarkdown.Convert([]byte(markdown), &rendered); err != nil {
		return template.HTML("<p>Unable to render this unit.</p>")
	}
	output := rendered.String()
	for _, embed := range embeds {
		output = strings.Replace(output, "<p>"+embed.placeholder+"</p>\n", renderUnitContentIframe(embed), 1)
	}
	return template.HTML(output)
}

type unitContentEmbed struct {
	placeholder string
	title       string
	srcdoc      string
	src         string
}

func extractUnitContentIframes(source string) (string, []unitContentEmbed) {
	sum := sha256.Sum256([]byte(source))
	prefix := "UCIFRAME" + strings.ToUpper(hex.EncodeToString(sum[:6]))
	var embeds []unitContentEmbed
	markdown := unitContentIframe.ReplaceAllStringFunc(source, func(tag string) string {
		match := unitContentIframe.FindStringSubmatch(tag)
		attributes := parseIframeAttributes(match[1])
		embed := unitContentEmbed{
			placeholder: prefix + string(rune('A'+len(embeds))),
			title:       strings.TrimSpace(html.UnescapeString(attributes["title"])),
			srcdoc:      html.UnescapeString(attributes["srcdoc"]),
			src:         strings.TrimSpace(html.UnescapeString(attributes["src"])),
		}
		if embed.title == "" {
			embed.title = "Interactive unit content"
		}
		embeds = append(embeds, embed)
		return "\n\n" + embed.placeholder + "\n\n"
	})
	return markdown, embeds
}

func parseIframeAttributes(source string) map[string]string {
	attributes := make(map[string]string)
	for _, match := range iframeAttribute.FindAllStringSubmatch(source, -1) {
		value := match[2]
		if value == "" {
			value = match[3]
		}
		attributes[strings.ToLower(match[1])] = value
	}
	return attributes
}

func renderUnitContentIframe(embed unitContentEmbed) string {
	if strings.TrimSpace(embed.srcdoc) != "" {
		document := embeddedDocument(embed.srcdoc)
		return `<iframe class="unit-visualization" title="` + html.EscapeString(embed.title) +
			`" loading="lazy" sandbox="allow-scripts" srcdoc="` + html.EscapeString(document) + `"></iframe>`
	}
	if source, ok := safeYouTubeEmbedURL(embed.src); ok {
		return `<iframe class="unit-visualization unit-visualization--video" title="` + html.EscapeString(embed.title) +
			`" loading="lazy" sandbox="allow-scripts allow-same-origin allow-presentation" referrerpolicy="strict-origin-when-cross-origin"` +
			` allow="accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen src="` +
			html.EscapeString(source) + `"></iframe>`
	}
	return `<p class="unit-content__embed-error">This iframe source is not supported.</p>`
}

func safeYouTubeEmbedURL(source string) (string, bool) {
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil ||
		(parsed.Hostname() != "www.youtube.com" && parsed.Hostname() != "www.youtube-nocookie.com") ||
		parsed.Port() != "" || !strings.HasPrefix(parsed.EscapedPath(), "/embed/") ||
		strings.TrimPrefix(parsed.EscapedPath(), "/embed/") == "" {
		return "", false
	}
	return parsed.String(), true
}

func embeddedDocument(body string) string {
	const shell = `<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:; font-src data:">` +
		`<style>:root{font-family:Montserrat,system-ui,sans-serif;color:#20201e;background:#faf9f6;color-scheme:light}*{box-sizing:border-box}body{margin:0;padding:1rem;min-height:100vh}button,input{font:inherit}</style>` +
		`</head><body>`
	const resize = `<script>(()=>{const send=()=>parent.postMessage({type:"unit-visualization-height",height:document.documentElement.scrollHeight},"*");new ResizeObserver(send).observe(document.documentElement);addEventListener("load",send);send()})()</script>`
	return shell + strings.TrimSpace(body) + resize + `</body></html>`
}
