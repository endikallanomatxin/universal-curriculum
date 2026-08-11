package main

import (
	"bytes"
	"fmt"
	"os"
)

const (
	sourcePath = "../../../docs/openapi.yaml"
	targetPath = "openapi.yaml"
)

func main() {
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		fail("read canonical OpenAPI contract", err)
	}
	current, err := os.ReadFile(targetPath)
	if err == nil && bytes.Equal(current, source) {
		return
	}
	if err != nil && !os.IsNotExist(err) {
		fail("read embedded OpenAPI contract", err)
	}
	if err := os.WriteFile(targetPath, source, 0o644); err != nil {
		fail("write embedded OpenAPI contract", err)
	}
}

func fail(operation string, err error) {
	_, _ = fmt.Fprintf(os.Stderr, "openapi generation: %s: %v\n", operation, err)
	os.Exit(1)
}
