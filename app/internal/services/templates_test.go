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
	for _, fragment := range []string{
		`class="brand__initial">u</span>`,
		`class="brand__expansion">niversal`,
		`class="brand__word brand__word--curriculum"`,
		`class="brand__initial">c</span>`,
		`class="brand__expansion">urriculum`,
	} {
		if !strings.Contains(memberOutput, fragment) {
			t.Errorf("text brand does not contain %q", fragment)
		}
	}
	if strings.Contains(memberOutput, "universal-curriculum-logo.svg") {
		t.Fatal("navigation still renders the removed logo asset")
	}

	adminOutput := render(&models.User{FullName: "Admin", Email: "admin@example.com", IsAdmin: true})
	if !strings.Contains(adminOutput, `href="/curriculum-modification"`) {
		t.Fatal("curriculum modification navigation is not visible to an admin user")
	}
}

func TestLearningPathPanelHasCloseNavigation(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "learn.html", map[string]any{
		"ShowGraph": true,
		"Graph":     &models.CurriculumGraphLayout{},
	})

	for _, fragment := range []string{
		`aria-label="Close learning path"`,
		`href="/learn"`,
		`hx-trigger="panel-close"`,
		`data-panel-navigation="close"`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("learning path close control does not contain %q", fragment)
		}
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

func TestRecognizedUnitCompletionCanBeCompletedLiterally(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "unit-completion-form", map[string]any{
		"UnitID":     int64(7),
		"Completed":  true,
		"Recognized": true,
	})

	if !strings.Contains(output, ">Recognized<") || !strings.Contains(output, "is-recognized") {
		t.Fatal("recognized completion does not show its state")
	}
	if !strings.Contains(output, `action="/learn/units/7/completion"`) ||
		!strings.Contains(output, `name="completed" value="false"`) ||
		!strings.Contains(output, `name="completed" value="true"`) {
		t.Fatal("recognized completion cannot be returned to pending or completed against the current version")
	}
}

func TestCurriculumProposalRendersRecognitionWorkflowAndPublishWarning(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "admin-curriculum.html", map[string]any{
		"User":            &models.User{FullName: "Admin", IsAdmin: true},
		"CSRFToken":       "csrf",
		"CanEditProposal": true,
		"ActiveProposal": &models.CurriculumProposal{
			ID: 12, Title: "Replace foundations", Status: "draft",
			Changes: []models.CurriculumProposalChange{{
				ID: 13, Kind: "recognition",
				Recognition: &models.Recognition{
					Sources: []models.Unit{{ID: 1, Name: "Old foundations"}},
					Targets: []models.Unit{{ID: 2, Name: "New foundations"}},
				},
			}},
		},
		"RecognitionSources": []models.Unit{{ID: 1, Name: "Old foundations"}},
		"RecognitionTargets": []models.Unit{{ID: 2, Name: "New foundations"}},
		"PublishWarning":     "One unit has no recognized successor. Publish anyway?",
	})

	for _, fragment := range []string{
		`action="/curriculum-modification/recognitions"`,
		`name="source_unit_ids"`,
		`name="target_unit_ids"`,
		"Recognition",
		`href="/curriculum-modification?proposal=12&amp;unit=2&amp;content=2"`,
		`data-panel-navigation="open-or-replace"`,
		"Publish anyway?",
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered recognition workflow does not contain %q", fragment)
		}
	}
	if strings.Contains(output, `id="recognition-rationale"`) {
		t.Error("recognition workflow unexpectedly asks for a per-change rationale")
	}
}

func TestProposalDependencyChangeLinksBothUnitsIndependently(t *testing.T) {
	templates := loadTestTemplates(t)
	prerequisiteID := int64(1)
	output := renderTemplate(t, templates, "admin-curriculum.html", map[string]any{
		"ActiveProposal": &models.CurriculumProposal{
			ID: 12, Title: "Connect algebra", Status: "draft",
			Changes: []models.CurriculumProposalChange{{
				ID: 13, Kind: "add_dependency", UnitID: 2, UnitName: "Algebra",
				PrerequisiteID: &prerequisiteID, PrerequisiteName: "Foundations",
			}},
		},
	})

	for _, fragment := range []string{
		`href="/curriculum-modification?proposal=12&amp;unit=1&amp;content=1"`,
		`href="/curriculum-modification?proposal=12&amp;unit=2&amp;content=2"`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("dependency change does not contain %q", fragment)
		}
	}
}

func TestCurriculumProposalContentPanelRendersUnitContentDiff(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "admin-curriculum.html", map[string]any{
		"ActiveProposal": &models.CurriculumProposal{
			ID: 12, Title: "Improve explanations", Status: "draft",
		},
		"CanEditProposal": true,
		"ContentUnit": map[string]any{
			"ID": 7, "Name": "Energy", "Content": "Energy can be stored.",
			"HasContentDiff": true, "PreviousContent": "Energy is stored.",
		},
	})

	for _, fragment := range []string{
		"Proposed content changes",
		`class="view-switcher"`,
		`data-view-switcher-trigger="source"`,
		`data-view-switcher-trigger="rendered"`,
		`data-view-switcher-panel="rendered" hidden`,
		"<del>is</del>",
		"<ins>can be</ins>",
		"Before",
		"After",
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered content change does not contain %q", fragment)
		}
	}
	if strings.Contains(output, "View content changes") || strings.Contains(output, "<details") {
		t.Error("content diff should be shown directly in the unit panel")
	}
}

func TestCurriculumProposalRendersRebaseResolutionInUnifiedWorkspace(t *testing.T) {
	templates := loadTestTemplates(t)
	change := models.CurriculumProposalChange{
		ID: 31, Kind: "update_content", UnitID: 7, UnitName: "Energy",
		UnitContent: "Proposed energy content.", PreviousUnitContent: "Original energy content.",
	}
	output := renderTemplate(t, templates, "admin-curriculum.html", map[string]any{
		"CSRFToken": "csrf",
		"ActiveProposal": &models.CurriculumProposal{
			ID: 12, Title: "Improve energy", Rationale: "Clarify the unit.", Status: "draft",
			Changes: []models.CurriculumProposalChange{change},
		},
		"ProposalRebase": &CurriculumProposalRebasePlan{
			Status: ProposalRebaseNeedsReview,
			Conflicts: []CurriculumProposalRebaseConflict{{
				Change:       change,
				AcceptedUnit: &models.Unit{ID: 7, Name: "Energy", Content: "Accepted energy content."},
				Units:        []models.Unit{{ID: 7, Name: "Energy"}},
				AcceptedWork: []CurriculumProposalRebaseAcceptedWork{{
					Proposal: models.CurriculumProposal{ID: 11, Title: "Update physics", Status: "accepted"},
					Changes:  []models.CurriculumProposalChange{{ID: 30, Kind: "update_content", UnitID: 7, UnitName: "Energy"}},
				}},
			}},
		},
		"RebaseTimeline": map[string]any{
			"BaseTitle": "Original curriculum", "DraftTitle": "Improve energy",
			"Items": []map[string]any{{"ID": int64(11), "Title": "Update physics", "Conflicting": true, "Current": true}},
			"Edges": []map[string]any{{"Source": "base", "Target": "draft"}, {"Source": "base", "Target": "accepted-11"}},
		},
	})

	for _, fragment := range []string{
		`action="/curriculum-modification/proposals/12/rebase"`,
		`name="resolution_31"`,
		"proposal-rebase-graph",
		"Original curriculum",
		"Update physics",
		"Resolved source",
		"Accepted energy content.",
		"Proposed energy content.",
		`data-merge-comparison`,
		"Comparison",
		"Result",
		`name="resolution_content_31"`,
		`action="/curriculum-modification/proposals/12"`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("rendered rebase workspace does not contain %q", fragment)
		}
	}
	if strings.Contains(output, `id="proposal-details-panel"`) {
		t.Fatal("proposal details should not be rendered as a separate workspace")
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
