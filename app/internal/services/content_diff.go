package services

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

type contentDiffPart struct {
	kind string
	text string
}

var contentDiffTokenPattern = regexp.MustCompile(`\s+|[\pL\pN_]+|[^\s\pL\pN_]`)

func RenderContentDiff(previous, current string) template.HTML {
	parts := contentDiffParts(
		contentDiffTokenPattern.FindAllString(previous, -1),
		contentDiffTokenPattern.FindAllString(current, -1),
	)
	var output strings.Builder
	output.WriteString(`<pre class="content-diff" aria-label="Content changes">`)
	for _, part := range parts {
		text := html.EscapeString(part.text)
		switch part.kind {
		case "deleted":
			output.WriteString("<del>" + text + "</del>")
		case "inserted":
			output.WriteString("<ins>" + text + "</ins>")
		default:
			output.WriteString(text)
		}
	}
	output.WriteString(`</pre>`)
	return template.HTML(output.String())
}

func contentDiffParts(previous, current []string) []contentDiffPart {
	prefix := 0
	for prefix < len(previous) && prefix < len(current) && previous[prefix] == current[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(previous)-prefix && suffix < len(current)-prefix &&
		previous[len(previous)-1-suffix] == current[len(current)-1-suffix] {
		suffix++
	}

	parts := make([]contentDiffPart, 0)
	parts = appendContentDiffPart(parts, "same", strings.Join(previous[:prefix], ""))
	oldMiddle := previous[prefix : len(previous)-suffix]
	newMiddle := current[prefix : len(current)-suffix]
	if len(oldMiddle)*len(newMiddle) > 2_000_000 {
		parts = appendContentDiffPart(parts, "deleted", strings.Join(oldMiddle, ""))
		parts = appendContentDiffPart(parts, "inserted", strings.Join(newMiddle, ""))
	} else {
		parts = append(parts, compareContentDiffTokens(oldMiddle, newMiddle)...)
	}
	if suffix > 0 {
		parts = appendContentDiffPart(parts, "same", strings.Join(previous[len(previous)-suffix:], ""))
	}
	return parts
}

func compareContentDiffTokens(previous, current []string) []contentDiffPart {
	lengths := make([][]int, len(previous)+1)
	for index := range lengths {
		lengths[index] = make([]int, len(current)+1)
	}
	for oldIndex := len(previous) - 1; oldIndex >= 0; oldIndex-- {
		for newIndex := len(current) - 1; newIndex >= 0; newIndex-- {
			if previous[oldIndex] == current[newIndex] {
				lengths[oldIndex][newIndex] = lengths[oldIndex+1][newIndex+1] + 1
			} else {
				lengths[oldIndex][newIndex] = max(lengths[oldIndex+1][newIndex], lengths[oldIndex][newIndex+1])
			}
		}
	}

	parts := make([]contentDiffPart, 0)
	oldIndex, newIndex := 0, 0
	for oldIndex < len(previous) && newIndex < len(current) {
		switch {
		case previous[oldIndex] == current[newIndex]:
			parts = appendContentDiffPart(parts, "same", previous[oldIndex])
			oldIndex++
			newIndex++
		case lengths[oldIndex+1][newIndex] >= lengths[oldIndex][newIndex+1]:
			parts = appendContentDiffPart(parts, "deleted", previous[oldIndex])
			oldIndex++
		default:
			parts = appendContentDiffPart(parts, "inserted", current[newIndex])
			newIndex++
		}
	}
	parts = appendContentDiffPart(parts, "deleted", strings.Join(previous[oldIndex:], ""))
	parts = appendContentDiffPart(parts, "inserted", strings.Join(current[newIndex:], ""))
	return parts
}

func appendContentDiffPart(parts []contentDiffPart, kind, text string) []contentDiffPart {
	if text == "" {
		return parts
	}
	if len(parts) > 0 && parts[len(parts)-1].kind == kind {
		parts[len(parts)-1].text += text
		return parts
	}
	return append(parts, contentDiffPart{kind: kind, text: text})
}
