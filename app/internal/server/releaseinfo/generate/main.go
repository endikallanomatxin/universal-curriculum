package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type document struct {
	version string
	summary string
	content string
}

func main() {
	releases := readDocuments("../../../../docs/releases", true)
	roadmap := readDocuments("../../../../docs/plan", false)

	var output strings.Builder
	output.WriteString("// Code generated from docs/releases and docs/plan; DO NOT EDIT.\n\npackage releaseinfo\n\n")
	writeCatalog(&output, "releases", releases)
	writeCatalog(&output, "roadmap", roadmap)
	if err := os.WriteFile("catalog_generated.go", []byte(output.String()), 0o644); err != nil {
		panic(err)
	}
}

func readDocuments(directory string, descending bool) []document {
	entries, err := os.ReadDir(directory)
	if err != nil {
		panic(err)
	}
	documents := make([]document, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			panic(err)
		}
		documents = append(documents, document{
			version: strings.TrimSuffix(entry.Name(), ".md"),
			summary: mainObjective(string(content)),
			content: strings.TrimSpace(string(content)),
		})
	}
	sort.Slice(documents, func(i, j int) bool {
		comparison := compareVersions(documents[i].version, documents[j].version)
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
	return documents
}

func mainObjective(content string) string {
	const heading = "## Main objective\n"
	start := strings.Index(content, heading)
	if start < 0 {
		return ""
	}
	objective := content[start+len(heading):]
	if end := strings.Index(objective, "\n## "); end >= 0 {
		objective = objective[:end]
	}
	return strings.TrimSpace(objective)
}

func compareVersions(left, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	for index := 0; index < 3; index++ {
		leftPart, _ := strconv.Atoi(leftParts[index])
		rightPart, _ := strconv.Atoi(rightParts[index])
		if leftPart != rightPart {
			return leftPart - rightPart
		}
	}
	return 0
}

func writeCatalog(output *strings.Builder, name string, documents []document) {
	fmt.Fprintf(output, "var %s = []Document{\n", name)
	for _, document := range documents {
		fmt.Fprintf(output, "\t{Version: %q, Summary: %q, Content: %q},\n", document.version, document.summary, document.content)
	}
	output.WriteString("}\n\n")
}
