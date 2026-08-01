package services

import (
	"html"
	"html/template"
	"regexp"
	"sort"
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

func RenderRenderedContentDiff(previous, current string) template.HTML {
	parts := contentDiffParts(markdownContentBlocks(previous), markdownContentBlocks(current))
	var output strings.Builder
	output.WriteString(`<div class="rendered-content-diff" aria-label="Rendered content changes">`)
	for index := 0; index < len(parts); {
		if parts[index].kind == "same" {
			output.WriteString(`<div class="rendered-content-diff__unchanged">`)
			output.WriteString(string(RenderUnitContent(parts[index].text)))
			output.WriteString(`</div>`)
			index++
			continue
		}

		var previousBlocks, currentBlocks strings.Builder
		for index < len(parts) && parts[index].kind != "same" {
			if parts[index].kind == "deleted" {
				previousBlocks.WriteString(parts[index].text)
			} else {
				currentBlocks.WriteString(parts[index].text)
			}
			index++
		}
		output.WriteString(`<div class="rendered-content-diff__change">`)
		if previousBlocks.Len() > 0 {
			output.WriteString(`<section class="rendered-content-diff__version rendered-content-diff__version--before"><p class="rendered-content-diff__label">Before</p>`)
			output.WriteString(string(RenderUnitContent(previousBlocks.String())))
			output.WriteString(`</section>`)
		}
		if currentBlocks.Len() > 0 {
			output.WriteString(`<section class="rendered-content-diff__version rendered-content-diff__version--after"><p class="rendered-content-diff__label">After</p>`)
			output.WriteString(string(RenderUnitContent(currentBlocks.String())))
			output.WriteString(`</section>`)
		}
		output.WriteString(`</div>`)
	}
	output.WriteString(`</div>`)
	return template.HTML(output.String())
}

type EditableContentMergePart struct {
	Kind     string
	Original string
	Accepted string
	Proposed string
}

type contentMergeChange struct {
	start, end  int
	replacement []string
}

func EditableContentMerge(original, accepted, proposed string) []EditableContentMergePart {
	base := contentMergeLines(original)
	acceptedChanges := contentMergeChanges(base, contentMergeLines(accepted))
	proposedChanges := contentMergeChanges(base, contentMergeLines(proposed))
	allChanges := append(append([]contentMergeChange(nil), acceptedChanges...), proposedChanges...)
	sort.SliceStable(allChanges, func(i, j int) bool {
		if allChanges[i].start == allChanges[j].start {
			return allChanges[i].end < allChanges[j].end
		}
		return allChanges[i].start < allChanges[j].start
	})

	merge := make([]EditableContentMergePart, 0, len(allChanges)*2+1)
	cursor := 0
	for index := 0; index < len(allChanges); {
		start, end := allChanges[index].start, allChanges[index].end
		index++
		for index < len(allChanges) && allChanges[index].start <= end {
			end = max(end, allChanges[index].end)
			index++
		}
		if start > cursor {
			shared := strings.Join(base[cursor:start], "")
			merge = append(merge, EditableContentMergePart{
				Kind: "same", Original: shared, Accepted: shared, Proposed: shared,
			})
		}
		originalPart := strings.Join(base[start:end], "")
		acceptedPart := applyContentMergeChanges(base, start, end, acceptedChanges)
		proposedPart := applyContentMergeChanges(base, start, end, proposedChanges)
		kind := "change"
		if acceptedPart == proposedPart {
			kind = "same"
		}
		merge = append(merge, EditableContentMergePart{
			Kind: kind, Original: originalPart, Accepted: acceptedPart, Proposed: proposedPart,
		})
		cursor = end
	}
	if cursor < len(base) {
		shared := strings.Join(base[cursor:], "")
		merge = append(merge, EditableContentMergePart{
			Kind: "same", Original: shared, Accepted: shared, Proposed: shared,
		})
	}
	if len(merge) == 0 {
		merge = append(merge, EditableContentMergePart{
			Kind: "same", Original: original, Accepted: accepted, Proposed: proposed,
		})
	}
	return merge
}

func contentMergeLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if content == "" {
		return nil
	}
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func contentMergeChanges(original, variant []string) []contentMergeChange {
	parts := contentDiffParts(original, variant)
	changes := make([]contentMergeChange, 0)
	position := 0
	for index := 0; index < len(parts); {
		if parts[index].kind == "same" {
			position += len(contentMergeLines(parts[index].text))
			index++
			continue
		}
		change := contentMergeChange{start: position, end: position}
		for index < len(parts) && parts[index].kind != "same" {
			lines := contentMergeLines(parts[index].text)
			if parts[index].kind == "deleted" {
				change.end += len(lines)
				position += len(lines)
			} else {
				change.replacement = append(change.replacement, lines...)
			}
			index++
		}
		changes = append(changes, change)
	}
	return changes
}

func applyContentMergeChanges(base []string, start, end int, changes []contentMergeChange) string {
	position := start
	var output strings.Builder
	for _, change := range changes {
		if change.start < start || change.start > end || change.end > end {
			continue
		}
		output.WriteString(strings.Join(base[position:change.start], ""))
		output.WriteString(strings.Join(change.replacement, ""))
		position = change.end
	}
	output.WriteString(strings.Join(base[position:end], ""))
	return output.String()
}

func markdownContentBlocks(source string) []string {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.SplitAfter(source, "\n")
	blocks := make([]string, 0)
	var block strings.Builder
	insideFence := false
	fenceMarker := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !insideFence && block.Len() > 0 && trimmed == "" {
			block.WriteString(line)
			blocks = append(blocks, block.String())
			block.Reset()
			continue
		}
		block.WriteString(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !insideFence {
				insideFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				insideFence = false
				fenceMarker = ""
			}
		}
	}
	if block.Len() > 0 {
		blocks = append(blocks, block.String())
	}
	return blocks
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
