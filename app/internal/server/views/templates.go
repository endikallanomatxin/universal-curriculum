package views

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

type viewSwitcherView struct {
	Label   string
	Options []viewSwitcherOptionView
}

type viewSwitcherOptionView struct {
	Value    string
	Label    string
	Selected bool
}

func newViewSwitcher(label, selected string, options ...string) viewSwitcherView {
	view := viewSwitcherView{Label: label}
	for index := 0; index+1 < len(options); index += 2 {
		view.Options = append(view.Options, viewSwitcherOptionView{
			Value: options[index], Label: options[index+1], Selected: options[index] == selected,
		})
	}
	return view
}

func LoadTemplates() (*template.Template, error) {
	assetVersion, err := staticAssetVersion("web/static")
	if err != nil {
		return nil, fmt.Errorf("version static assets: %w", err)
	}
	templates, err := template.New("application").Funcs(template.FuncMap{
		"assetVersion":              func() string { return assetVersion },
		"renderContentDiff":         RenderContentDiff,
		"renderRenderedContentDiff": RenderRenderedContentDiff,
		"renderUnitContent":         RenderUnitContent,
		"viewSwitcher":              newViewSwitcher,
	}).ParseGlob("web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	if _, err := templates.ParseGlob("web/templates/shared/*.html"); err != nil {
		return nil, fmt.Errorf("parse shared templates: %w", err)
	}
	if _, err := templates.ParseGlob("web/templates/curriculum-modification/*.html"); err != nil {
		return nil, fmt.Errorf("parse curriculum modification templates: %w", err)
	}
	if _, err := templates.ParseGlob("web/templates/learn/*.html"); err != nil {
		return nil, fmt.Errorf("parse learning templates: %w", err)
	}
	if _, err := templates.ParseGlob("web/templates/auth/*.html"); err != nil {
		return nil, fmt.Errorf("parse auth templates: %w", err)
	}
	if _, err := templates.ParseGlob("web/templates/about/*.html"); err != nil {
		return nil, fmt.Errorf("parse about templates: %w", err)
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
