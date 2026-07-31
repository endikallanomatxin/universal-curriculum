package services

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"universal-curriculum/internal/models"
)

func TestLoadTemplatesCompilesAndRendersRepresentativePages(t *testing.T) {
	templates := loadTestTemplates(t)
	user := &models.User{FullName: "Example User", Email: "user@example.com", IsAdmin: true}

	for _, test := range []struct {
		name string
		data any
	}{
		{
			name: "index.html",
			data: map[string]any{
				"User": user, "CSRFToken": "csrf", "CurrentSection": "home", "Home": true,
			},
		},
		{
			name: "account.html",
			data: map[string]any{
				"User": user, "CSRFToken": "csrf", "CurrentSection": "account",
			},
		},
		{
			name: "learn.html",
			data: map[string]any{
				"User": user, "CSRFToken": "csrf", "CurrentSection": "learn",
			},
		},
		{
			name: "admin-curriculum.html",
			data: map[string]any{
				"User": user, "CSRFToken": "csrf", "CurrentSection": "curriculum",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := renderTemplate(t, templates, test.name, test.data)
			if !strings.Contains(output, "<!doctype html>") {
				t.Fatalf("%s did not render a complete document", test.name)
			}
		})
	}
}

func TestApplicationNavigationRespectsAdminPermission(t *testing.T) {
	templates := loadTestTemplates(t)

	render := func(user *models.User) string {
		return renderTemplate(t, templates, "index.html", map[string]any{
			"User": user, "CSRFToken": "csrf-token", "CurrentSection": "home", "Home": true,
		})
	}

	memberOutput := render(&models.User{FullName: "Member", Email: "member@example.com"})
	if strings.Contains(memberOutput, `href="/curriculum-modification"`) {
		t.Fatal("curriculum modification navigation is visible to a non-admin user")
	}
	if !strings.Contains(memberOutput, `href="/learn"`) ||
		!strings.Contains(memberOutput, `href="/account"`) ||
		!strings.Contains(memberOutput, `action="/auth/logout"`) ||
		!strings.Contains(memberOutput, `name="csrf_token" value="csrf-token"`) {
		t.Fatal("authenticated navigation is missing a required destination or logout protection")
	}
	if !strings.Contains(memberOutput, `href="/" hx-get="/" hx-target="#app-shell" hx-select="#app-shell"`) {
		t.Fatal("home navigation does not replace the shell that owns personalized recommendations")
	}

	adminOutput := render(&models.User{FullName: "Admin", Email: "admin@example.com", IsAdmin: true})
	if !strings.Contains(adminOutput, `href="/curriculum-modification"`) {
		t.Fatal("curriculum modification navigation is not visible to an admin user")
	}
}

func TestAuthenticationTemplatesRenderCriticalFlows(t *testing.T) {
	templates := loadTestTemplates(t)

	for _, test := range []struct {
		name       string
		template   string
		data       any
		contains   []string
		notContain []string
	}{
		{
			name:     "login",
			template: "login.html",
			data:     map[string]any{"Next": "/learn"},
			contains: []string{
				`action="/auth/login"`,
				`name="next" value="/learn"`,
				`href="/auth/forgot-password"`,
				`href="/auth/register?next=%2flearn"`,
			},
		},
		{
			name:     "registration",
			template: "register.html",
			data:     map[string]any{"Next": "/learn"},
			contains: []string{
				`action="/auth/register"`,
				`name="full_name"`,
				`name="email" type="email"`,
				`name="password" type="password"`,
				`href="/auth/login?next=%2flearn"`,
			},
		},
		{
			name:     "password reset request",
			template: "forgot-password.html",
			data:     map[string]any{},
			contains: []string{
				`action="/auth/forgot-password"`,
				`name="email" type="email"`,
			},
		},
		{
			name:     "non-enumerating password reset confirmation",
			template: "forgot-password.html",
			data:     map[string]any{"Requested": true},
			contains: []string{
				"If an account exists for that email address",
				"The link expires in one hour",
			},
			notContain: []string{`action="/auth/forgot-password"`},
		},
		{
			name:     "valid password reset",
			template: "reset-password.html",
			data:     map[string]any{"Token": "opaque-token"},
			contains: []string{
				`action="/auth/reset-password"`,
				`name="token" value="opaque-token"`,
				`name="password_confirmation"`,
			},
		},
		{
			name:     "invalid password reset",
			template: "reset-password.html",
			data:     map[string]any{"Invalid": true},
			contains: []string{
				"This password reset link is invalid or has expired",
				`href="/auth/forgot-password"`,
			},
			notContain: []string{`action="/auth/reset-password"`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := renderTemplate(t, templates, test.template, test.data)
			for _, fragment := range test.contains {
				if !strings.Contains(output, fragment) {
					t.Errorf("rendered template does not contain %q", fragment)
				}
			}
			for _, fragment := range test.notContain {
				if strings.Contains(output, fragment) {
					t.Errorf("rendered template unexpectedly contains %q", fragment)
				}
			}
		})
	}
}

func TestUnitCompletionRendersNarrowHTMXUpdate(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "unit-completion-form", map[string]any{
		"UnitID":    int64(7),
		"CSRFToken": "csrf",
		"ReturnURL": "/learn?unit=7",
		"Completed": false,
	})

	for _, fragment := range []string{
		`action="/learn/units/7/completion"`,
		`hx-post="/learn/units/7/completion"`,
		`hx-target="this"`,
		`hx-swap="outerHTML"`,
		`name="completed" value="true"`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered completion form does not contain %q", fragment)
		}
	}
	if strings.Contains(output, "transition:true") || strings.Contains(output, `hx-target="#workspace"`) {
		t.Fatal("unit completion should update only its stable fragment")
	}
}

func TestTransferredUnitCompletionIsReadOnlyRecognition(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "unit-completion-form", map[string]any{
		"UnitID":      int64(7),
		"Completed":   true,
		"Transferred": true,
	})

	if !strings.Contains(output, "Recognized through knowledge transfer") {
		t.Fatal("transferred completion does not explain its provenance")
	}
	if strings.Contains(output, `action="/learn/units/7/completion"`) {
		t.Fatal("transferred-only completion can be independently changed")
	}
}

func TestCurriculumProposalRendersKnowledgeTransferWorkflowAndPublishWarning(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "admin-curriculum.html", map[string]any{
		"User":         &models.User{FullName: "Admin", IsAdmin: true},
		"CSRFToken":    "csrf",
		"ProposalView": "work",
		"ActiveProposal": &models.CurriculumProposal{
			ID: 12, Title: "Replace foundations", Status: "draft",
			Changes: []models.CurriculumProposalChange{{
				ID: 13, Kind: "transfer_knowledge",
				KnowledgeTransfer: &models.KnowledgeTransfer{
					Rationale: "Equivalent coverage.",
					Sources:   []models.Unit{{ID: 1, Name: "Old foundations"}},
					Targets:   []models.Unit{{ID: 2, Name: "New foundations"}},
				},
			}},
		},
		"TransferSources": []models.Unit{{ID: 1, Name: "Old foundations"}},
		"TransferTargets": []models.Unit{{ID: 2, Name: "New foundations"}},
		"PublishWarning":  "One unit has no recognized successor. Publish anyway?",
	})

	for _, fragment := range []string{
		`action="/curriculum-modification/knowledge-transfers"`,
		`name="source_unit_ids"`,
		`name="target_unit_ids"`,
		"Transfer knowledge",
		"Equivalent coverage.",
		"Publish anyway?",
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered knowledge-transfer workflow does not contain %q", fragment)
		}
	}
}

func TestStaticAssetVersionChangesWithContents(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(root, "css", "base.css")
	if err := os.WriteFile(asset, []byte("body { color: black; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := staticAssetVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	same, err := staticAssetVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	if first != same {
		t.Fatalf("asset version is not deterministic: %q != %q", first, same)
	}
	if len(first) != 12 {
		t.Fatalf("asset version length = %d, want 12", len(first))
	}

	if err := os.WriteFile(asset, []byte("body { color: purple; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := staticAssetVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("asset version did not change with asset contents")
	}
}

func loadTestTemplates(t *testing.T) *template.Template {
	t.Helper()
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
	return templates
}

func renderTemplate(t *testing.T, templates *template.Template, name string, data any) string {
	t.Helper()
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, name, data); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
