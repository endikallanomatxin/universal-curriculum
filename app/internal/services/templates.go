package services

import (
	"fmt"
	"html/template"
)

func LoadTemplates() (*template.Template, error) {
	templates, err := template.ParseGlob("web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return templates, nil
}
