package views

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
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
			name: "curriculum-modification.html",
			data: map[string]any{
				"User": user, "CSRFToken": "csrf", "CurrentSection": "curriculum-modification",
			},
		},
		{
			name: "administration.html",
			data: map[string]any{
				"User": user, "CSRFToken": "csrf", "CurrentSection": "administration",
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

func TestAdministrationRendersVisibleWorkspace(t *testing.T) {
	output := renderTemplate(t, loadTestTemplates(t), "administration.html", map[string]any{
		"User": &models.User{FullName: "Admin", IsAdmin: true}, "CSRFToken": "csrf",
	})
	for _, fragment := range []string{
		`class="pane-stack" id="workspace"`, `data-panel-required-mode="content"`,
		`id="administration-title"`, `href="/admin/users"`, "User administration",
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("administration page does not contain %q", fragment)
		}
	}
	if strings.Contains(output, "All users") || strings.Contains(output, "Proposals") {
		t.Fatal("administration index exposes user details before opening user administration")
	}
}

func TestUserAdministrationRendersNestedListAndDetailPanels(t *testing.T) {
	selected := &models.User{ID: 7, FullName: "Contributor", Email: "person@example.com", IsContributor: true}
	output := renderTemplate(t, loadTestTemplates(t), "administration.html", map[string]any{
		"User": &models.User{FullName: "Admin", IsAdmin: true}, "CSRFToken": "csrf",
		"ShowUsers": true, "Users": []models.User{*selected}, "SelectedUser": selected,
		"ActiveInvitations": []models.ContributorInvitation{{ID: 3, Email: "invited@example.com"}},
		"UserProposals":     []models.CurriculumProposal{{ID: 9, Title: "Proposal", Rationale: "Reason", Status: "rejected"}},
	})
	for _, fragment := range []string{
		`id="user-administration-title"`, `id="contributor-invitations-title"`, "invited@example.com", "New invitation",
		`href="/admin/users/7"`, ">Contributor<",
		`id="new-contributor-invitation-panel"`, `hidden aria-labelledby="invite-contributor-title"`,
		`id="user-detail-title"`, "person@example.com", "Proposals", `href="/curriculum-modification?proposal=9"`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("user administration does not contain %q", fragment)
		}
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

func TestAccountAPITokenCreationUsesNestedPanel(t *testing.T) {
	templates := loadTestTemplates(t)
	render := func(extra map[string]any) string {
		data := map[string]any{
			"User":      &models.User{FullName: "Example User", Email: "user@example.com"},
			"CSRFToken": "csrf-token",
		}
		for key, value := range extra {
			data[key] = value
		}
		return renderTemplate(t, templates, "account.html", data)
	}

	closed := render(nil)
	if !strings.Contains(closed, `aria-expanded="false"`) ||
		!strings.Contains(closed, `id="external-access-panel"`) ||
		!strings.Contains(closed, `data-panel-breadcrumb="External access" hidden`) ||
		!strings.Contains(closed, `data-panel-breadcrumb="New API token" hidden`) {
		t.Fatal("external access panels are not initially closed")
	}

	output := render(map[string]any{"TokenError": "token name is required"})

	for _, fragment := range []string{
		`data-open-panel="external-access-panel"`,
		`aria-controls="external-access-panel"`,
		`id="external-access-panel" data-nested-panel data-panel-motion="horizontal"`,
		`data-close-panel data-close-descendants aria-label="Close external access panel"`,
		`data-open-panel="new-api-token-panel"`,
		`aria-controls="new-api-token-panel"`,
		`aria-expanded="true"`,
		`id="new-api-token-panel"`,
		`data-api-token-panel data-nested-panel`,
		`data-close-panel aria-label="Close API token panel"`,
		`name="csrf_token" value="csrf-token"`,
		`role="alert">token name is required`,
	} {
		if !strings.Contains(output, fragment) {
			t.Errorf("API token panel does not contain %q", fragment)
		}
	}
	if strings.Contains(output, `data-panel-breadcrumb="New API token" hidden`) {
		t.Fatal("API token panel remains hidden after validation fails")
	}
	if strings.Contains(output, `data-panel-breadcrumb="External access" hidden`) {
		t.Fatal("external access panel remains hidden with its child open")
	}

	created := render(map[string]any{"NewAPIToken": "uc_api_secret"})
	if !strings.Contains(created, `API token created`) ||
		!strings.Contains(created, `role="status"`) ||
		!strings.Contains(created, `data-copy-api-token`) ||
		!strings.Contains(created, `data-api-token-form hidden`) ||
		!strings.Contains(created, `uc_api_secret`) {
		t.Fatal("new API token is not shown with copy controls in the open panel")
	}
}

func TestLearningPathPanelHasCloseNavigation(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "learn.html", map[string]any{
		"ShowGraph": true,
		"Graph":     &models.CurriculumGraphLayout{},
		"GraphURL":  "/learn?path=7&unit=3",
	})

	for _, fragment := range []string{
		`aria-label="Close learning path"`,
		`href="/learn"`,
		`hx-trigger="panel-close"`,
		`data-panel-navigation="close"`,
		`data-panel-breadcrumb-url="/learn?path=7&amp;unit=3"`,
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
	output := renderTemplate(t, templates, "curriculum-modification.html", map[string]any{
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
	output := renderTemplate(t, templates, "curriculum-modification.html", map[string]any{
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
	output := renderTemplate(t, templates, "curriculum-modification.html", map[string]any{
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
		`data-panel-breadcrumb-url="/curriculum-modification?proposal=12"`,
		`data-panel-preserve-scroll`,
		`data-panel-close-query="content"`,
		`data-close-panel`,
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

func TestAcceptedProposalHistoryLinksOpenDetailBesideHistory(t *testing.T) {
	templates := loadTestTemplates(t)
	accepted := models.CurriculumProposal{ID: 8, Title: "Clarify electricity", Status: "accepted", AuthorName: "Ada"}
	output := renderTemplate(t, templates, "curriculum-modification.html", map[string]any{
		"ShowProposalHistory":  true,
		"ProposalHistoryLimit": 10,
		"ProposalHistory": []map[string]any{{
			"ID": accepted.ID, "Title": accepted.Title, "Status": accepted.Status,
			"AuthorName": accepted.AuthorName, "IsHead": true,
		}},
		"ReviewedProposal":        &accepted,
		"ViewingAcceptedProposal": true,
		"GraphURL":                "/curriculum-modification?history=1&history-limit=10&review-proposal=8",
		"Graph":                   map[string]any{"Nodes": []map[string]any{{"ID": 4}}},
		"GraphView": map[string]any{
			"IDPrefix": "accepted-proposal", "Description": "Accepted curriculum graph",
			"Nodes": []map[string]any{{
				"ID": 4, "Name": "Electric charge", "Lane": 0,
				"NavigateURL": "/curriculum-modification?history=1&history-limit=10&review-proposal=8&unit=4",
				"ContentURL":  "/curriculum-modification?history=1&history-limit=10&review-proposal=8&unit=4&content=4",
			}},
			"Layout": map[string]any{},
		},
		"GraphSearch": map[string]any{"ID": "accepted-search", "Label": "Find a unit", "Placeholder": "Find a unit"},
	})

	link := `href="/curriculum-modification?history=1&amp;history-limit=10&amp;review-proposal=8"`
	if !strings.Contains(output, link) {
		t.Errorf("accepted proposal history does not contain detail link %q", link)
	}
	if !strings.Contains(output, `href="/curriculum-modification?history=1&amp;history-limit=10"`) {
		t.Error("accepted proposal detail does not close back to history")
	}
	if strings.Index(output, `id="proposal-history-panel"`) > strings.Index(output, `aria-labelledby="related-proposal-title"`) {
		t.Error("accepted proposal detail should render to the right of history")
	}
	if !strings.Contains(output, "Showing the curriculum produced by this proposal") ||
		!strings.Contains(output, `href="/curriculum-modification?history=1&amp;history-limit=10&amp;review-proposal=8&amp;unit=4&amp;content=4"`) {
		t.Error("accepted proposal detail does not expose its read-only curriculum graph and content")
	}
	if strings.Contains(output, `data-open-panel="new-unit-panel"`) ||
		strings.Contains(output, `data-open-panel="edit-dependencies-panel"`) {
		t.Error("accepted proposal detail exposes curriculum editing controls")
	}
}

func TestProposalHistoryShowsNewestFirstAndLoadsOlderEntriesOnDemand(t *testing.T) {
	templates := loadTestTemplates(t)
	output := renderTemplate(t, templates, "curriculum-modification.html", map[string]any{
		"ShowProposalHistory":  true,
		"ProposalHistoryMore":  true,
		"ProposalHistoryLimit": 10,
		"ProposalHistoryNext":  20,
		"ProposalHistory": []map[string]any{
			{"ID": 2, "Title": "Newest", "AuthorName": "Ada", "IsHead": true,
				"Drafts": []map[string]any{{"ID": 3, "Title": "New draft"}}},
			{"ID": 1, "Title": "Older", "AuthorName": "Grace"},
		},
	})

	if strings.Index(output, "Newest") > strings.Index(output, "Older") {
		t.Error("proposal history should render newest proposals first")
	}
	if strings.Index(output, `data-rebase-node="history-draft-3"`) > strings.Index(output, `data-rebase-node="history-accepted-2"`) {
		t.Error("draft branches should render above the accepted proposal they come from")
	}
	if !strings.Contains(output, `history-limit=20`) || !strings.Contains(output, `>Show more</a>`) {
		t.Error("paginated proposal history does not offer the next page")
	}
	if !strings.Contains(output, `data-rebase-edge data-source="history-accepted-1" data-target="history-accepted-2"`) {
		t.Error("proposal history should point from older proposals to newer proposals")
	}
	if strings.Contains(output, "Initial curriculum") {
		t.Error("partial proposal history should not connect to the initial curriculum")
	}
}

func TestCurriculumProposalRendersRebaseResolutionInUnifiedWorkspace(t *testing.T) {
	templates := loadTestTemplates(t)
	change := models.CurriculumProposalChange{
		ID: 31, Kind: "update_content", UnitID: 7, UnitName: "Energy",
		UnitContent: "Proposed energy content.", PreviousUnitContent: "Original energy content.",
	}
	output := renderTemplate(t, templates, "curriculum-modification.html", map[string]any{
		"CSRFToken": "csrf",
		"ActiveProposal": &models.CurriculumProposal{
			ID: 12, Title: "Improve energy", Rationale: "Clarify the unit.", Status: "draft",
			Changes: []models.CurriculumProposalChange{change},
		},
		"ProposalRebase": &services.CurriculumProposalRebasePlan{
			Status: services.ProposalRebaseNeedsReview,
			Conflicts: []services.CurriculumProposalRebaseConflict{{
				Change:       change,
				AcceptedUnit: &models.Unit{ID: 7, Name: "Energy", Content: "Accepted energy content."},
				Units:        []models.Unit{{ID: 7, Name: "Energy"}},
				AcceptedWork: []services.CurriculumProposalRebaseAcceptedWork{{
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
	if strings.Index(output, `data-rebase-node="draft"`) > strings.Index(output, `data-rebase-node="base"`) {
		t.Error("the proposal being rebased should render above its base")
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
	if err := os.Chdir("../../.."); err != nil {
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
