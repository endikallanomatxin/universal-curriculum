package services

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"universal-curriculum/internal/models"
)

func TestApplicationShellTemplates(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })

	templates, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}

	user := &models.User{FullName: "Example User", Email: "user@example.com"}
	for _, test := range []struct {
		name     string
		data     any
		contains []string
	}{
		{
			name: "index.html",
			data: struct {
				User           *models.User
				CSRFToken      string
				CurrentSection string
				Home           bool
			}{User: user, CurrentSection: "home", Home: true},
			contains: []string{`id="app-shell"`, `class="app-shell app-shell--home"`, `>Account<`, `>Log out</span>`, `Material+Symbols+Rounded`},
		},
		{
			name: "account.html",
			data: struct {
				User           *models.User
				CSRFToken      string
				CurrentSection string
				Home           bool
			}{User: user, CurrentSection: "account"},
			contains: []string{`class="app-shell"`, `class="pane-stack"`, `hx-swap="outerHTML transition:true"`, `>person</span>`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := templates.ExecuteTemplate(&output, test.name, test.data); err != nil {
				t.Fatal(err)
			}
			for _, fragment := range test.contains {
				if !strings.Contains(output.String(), fragment) {
					t.Errorf("rendered template does not contain %q", fragment)
				}
			}
			if strings.Contains(output.String(), `>Home</span>`) {
				t.Error("home should not be listed as a navigation destination")
			}
			if strings.Contains(output.String(), `primary-navigation__footer`) {
				t.Error("account actions should not be duplicated in a navigation footer")
			}
		})
	}
}

func TestNavigationCounterpartsHaveSharedTransitions(t *testing.T) {
	stylesheet, err := os.ReadFile("../../web/static/css/shell.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, transition := range []string{
		"view-transition-name: brand",
		"view-transition-name: account-navigation",
		"view-transition-name: logout-action",
	} {
		if !strings.Contains(string(stylesheet), transition) {
			t.Errorf("navigation counterpart is missing %q", transition)
		}
	}
}
