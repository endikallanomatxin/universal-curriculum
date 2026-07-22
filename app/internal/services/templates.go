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
	if _, err := templates.ParseGlob("web/templates/auth/*.html"); err != nil {
		return nil, fmt.Errorf("parse auth templates: %w", err)
	}
	return templates, nil
}
