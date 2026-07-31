package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func LoadTemplates() (*template.Template, error) {
	assetVersion, err := staticAssetVersion("web/static")
	if err != nil {
		return nil, fmt.Errorf("version static assets: %w", err)
	}
	templates, err := template.New("application").Funcs(template.FuncMap{
		"assetVersion":      func() string { return assetVersion },
		"renderContentDiff": RenderContentDiff,
		"renderUnitContent": RenderUnitContent,
	}).ParseGlob("web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	if _, err := templates.ParseGlob("web/templates/auth/*.html"); err != nil {
		return nil, fmt.Errorf("parse auth templates: %w", err)
	}
	return templates, nil
}

func staticAssetVersion(root string) (string, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)

	hash := sha256.New()
	for _, path := range paths {
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		if _, err := io.WriteString(hash, filepath.ToSlash(relativePath)+"\x00"); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hash, file); err != nil {
			_ = file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil))[:12], nil
}
