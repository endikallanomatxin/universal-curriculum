package services

import (
	"bytes"
	"os"
	"path/filepath"
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
				User            *models.User
				CSRFToken       string
				CurrentSection  string
				Home            bool
				Recommendations []any
			}{User: user, CurrentSection: "home", Home: true},
			contains: []string{`id="app-shell"`, `class="app-shell app-shell--home"`, `class="brand__mark"`, `/static/images/universal-curriculum-logo.svg?v=`, `>universal<br>curriculum</span>`, `>Learn</span>`, `>book_5</span>`, `>Account<`, `>Log out</span>`, `Material+Symbols+Rounded`, `/static/css/base.css?v=`, `/static/js/shell.js?v=`},
		},
		{
			name: "account.html",
			data: struct {
				User           *models.User
				CSRFToken      string
				CurrentSection string
				Home           bool
			}{User: user, CurrentSection: "account"},
			contains: []string{`data-panel-group`, `data-panel-modes="mobile:0 icons:5 sidebar:13"`, `data-mobile-menu-toggle`, `data-panel-fill`, `class="pane-stack"`, `hx-target="#workspace"`, `hx-swap="outerHTML transition:true"`, `/static/js/panel_breadcrumbs.js?v=`, `>person</span>`},
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

func TestSharedCurriculumUITemplatesAreRegistered(t *testing.T) {
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
	for _, name := range []string{"curriculum-graph", "unit-navigation-search"} {
		if templates.Lookup(name) == nil {
			t.Errorf("shared UI template %q is not registered", name)
		}
	}
}

func TestEmailRegistrationIsAvailableFromLogin(t *testing.T) {
	login, err := os.ReadFile("../../web/templates/auth/login.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(login), `href="/auth/register?next={{ .Next }}"`) {
		t.Error("login does not offer account creation while preserving the destination")
	}

	registration, err := os.ReadFile("../../web/templates/auth/register.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		`define "register.html"`,
		`action="/auth/register"`,
		`name="full_name"`,
		`name="email" type="email"`,
		`name="password" type="password" autocomplete="new-password"`,
		`href="/auth/login?next={{ .Next }}"`,
	} {
		if !strings.Contains(string(registration), contract) {
			t.Errorf("registration template is missing %q", contract)
		}
	}
}

func TestProposalChangesExposeSemanticVisualStates(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/admin-curriculum.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{
		"proposal-change--addition",
		"proposal-change--removal",
		"proposal-change--rename",
		"proposal-change--content",
		"proposal-change--dependency",
		"proposal-change__previous",
		"proposal-change__revert",
		"Add dependency",
		"Remove dependency",
	} {
		if !strings.Contains(string(template), state) {
			t.Errorf("proposal change template is missing semantic state %q", state)
		}
	}

	stylesheet, err := os.ReadFile("../../web/static/css/curriculum.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		".proposal-list li.proposal-change--addition",
		".proposal-list li.proposal-change--removal",
		".proposal-list li.proposal-change--rename",
		".proposal-list li.proposal-change--content",
		".proposal-list li.proposal-change--dependency",
		"color: var(--proposal-change-color)",
		".proposal-list.proposal-change-list",
	} {
		if !strings.Contains(string(stylesheet), contract) {
			t.Errorf("proposal change styles are missing %q", contract)
		}
	}
}

func TestDraftProposalsAreListedOutsideProposalHistory(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/admin-curriculum.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`id="active-proposals-title"`,
		`class="ui-pane proposal-index-pane"`,
		`data-panel-modes="collapsed:0 breadcrumb:4 mobile:20 content:24 wide:28"`,
		`class="proposal-index__breadcrumb-title"`,
		`{{ if .DraftProposals }}`,
		`class="active-proposal-card`,
		`class="pane-list-actions"`,
		`class="pane-list-action" type="button" data-open-panel="new-proposal-panel"`,
		`class="pane-list-action" type="button" data-open-panel="proposal-history-panel"`,
		`Show history`,
		`class="active-proposal-card__identity active-proposal-card__work"`,
		`aria-label="Work on {{ .Title }}"`,
		`data-panel-navigation="{{ if $.ActiveProposal }}replace{{ else }}open{{ end }}"`,
		`href="/curriculum-modification?proposal={{ .ID }}&amp;view=details"`,
		`class="secondary-button active-proposal-card__details`,
		`{{ if and .ActiveProposal (eq .ProposalView "work") }}`,
		`id="proposal-workspace-panel" data-nested-panel data-panel-motion="horizontal"`,
		`data-panel-breadcrumb="Working on {{ .ActiveProposal.Title }}"`,
		`class="proposal-workspace__breadcrumb-title"`,
		`Working on {{ .ActiveProposal.Title }}</a>`,
		`hx-trigger="panel-close"`,
		`data-panel-navigation="close"`,
		`{{ if ne .ProposalView "details" }}hidden{{ end }}`,
		`Inspect the latest published curriculum versions.`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("proposal navigation is missing %q", contract)
		}
	}
	if strings.Contains(source, `{{ if eq .Status "draft" }}<a href="/curriculum-modification?proposal=`) {
		t.Error("proposal history should not remain responsible for selecting drafts")
	}
	if strings.Contains(source, `active-proposal-card__workspace`) {
		t.Error("active proposal cards should navigate to right-hand views instead of expanding inline")
	}
	if strings.Contains(source, `active-proposal-card__actions`) || strings.Contains(source, ">Work</a>") {
		t.Error("active proposal cards should open work directly instead of presenting a Work button")
	}
	for _, redundantHeading := range []string{
		`class="curriculum-editor__title"`,
		`class="ui-pane__eyebrow">Work</p>`,
		`class="ui-pane__eyebrow">Working proposal</p>`,
		`class="ui-pane__eyebrow">Curriculum modification</p>`,
	} {
		if strings.Contains(source, redundantHeading) {
			t.Errorf("proposal views should not repeat contextual heading %q", redundantHeading)
		}
	}

	stylesheet, err := os.ReadFile("../../web/static/css/curriculum.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		".active-proposal-card__work::after",
		".active-proposal-card__details {",
	} {
		if !strings.Contains(string(stylesheet), contract) {
			t.Errorf("active proposal card styles are missing %q", contract)
		}
	}

	components, err := os.ReadFile("../../web/static/css/components.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		".selectable-surface.is-selected {",
		"box-shadow: inset 0 0 0 2px var(--color-selection-outline)",
		".selectable-surface.is-selected:hover,",
		"transform: translateY(-0.12rem);",
		".pane-list-actions {",
		".pane-list-action {",
	} {
		if !strings.Contains(string(components), contract) {
			t.Errorf("shared selected surface styles are missing %q", contract)
		}
	}
	learnTemplate, err := os.ReadFile("../../web/templates/learn.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(learnTemplate), `class="pane-list-action"`) ||
		!strings.Contains(string(learnTemplate), `href="/learn?edit=new"`) {
		t.Error("new learning paths and proposals should use the same list action")
	}
	horizontalPanelAnimationStart := strings.Index(string(components), "@keyframes horizontal-panel-enter")
	if horizontalPanelAnimationStart < 0 {
		t.Fatal("horizontal panel animation is missing")
	}
	horizontalPanelAnimationEnd := strings.Index(string(components)[horizontalPanelAnimationStart:], "\n}")
	if horizontalPanelAnimationEnd < 0 {
		t.Fatal("horizontal panel animation is incomplete")
	}
	horizontalPanelAnimation := string(components)[horizontalPanelAnimationStart : horizontalPanelAnimationStart+horizontalPanelAnimationEnd]
	if !strings.Contains(horizontalPanelAnimation, "transform: translateX(100%);") {
		t.Error("horizontal panels do not enter from beyond the right edge")
	}
	if strings.Contains(horizontalPanelAnimation, "opacity") || strings.Contains(horizontalPanelAnimation, "clip-path") {
		t.Error("horizontal panel entry should not fade or reveal disconnected states")
	}

	panels, err := os.ReadFile("../../web/static/js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		`const navigationByRequest = new WeakMap()`,
		`function animateNavigatedPanels(root, mode)`,
		`function initializePanelNavigation(root)`,
		`[data-panel-motion="horizontal"]:not([hidden])`,
		`trigger.dataset.panelNavigation`,
		`trigger.setAttribute("hx-swap", "outerHTML transition:true")`,
		`htmx.trigger(trigger, "panel-close")`,
		`event.target.id === "workspace"`,
	} {
		if !strings.Contains(string(panels), contract) {
			t.Errorf("declarative panel navigation is missing %q", contract)
		}
	}
	for _, legacyDetail := range []string{
		"serverPanel",
		"data-panel-enter",
		"data-server-panel-close",
		"data-panel-continuity",
	} {
		if strings.Contains(source, legacyDetail) || strings.Contains(string(panels), legacyDetail) {
			t.Errorf("proposal navigation retains legacy detail %q", legacyDetail)
		}
	}
}

func TestAdminContentViewerCloseClearsOpenViewerState(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/admin-curriculum.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`href="/curriculum-modification?proposal={{ $.ActiveProposal.ID }}&amp;view=work&amp;unit={{ .ID }}"`,
		`hx-push-url="true"`,
		`aria-label="Close unit content"`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("admin content viewer close is missing %q", contract)
		}
	}
}

func TestHomeRecommendationsAvoidViewTransitionsFromHiddenWorkspace(t *testing.T) {
	components, err := os.ReadFile("../../web/templates/components.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(components)
	for _, contract := range []string{
		`define "shell-link-attributes-without-view-transition"`,
		`hx-swap="outerHTML"`,
		`template "shell-link-attributes-without-view-transition" .URL`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("home recommendation navigation is missing %q", contract)
		}
	}
}

func TestLearningPathsBreadcrumbReturnsToPathList(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/learn.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`data-panel-breadcrumb="Learning paths"`,
		`data-panel-required-mode="mobile" data-panel-fill data-panel-breadcrumb="Learning paths"`,
		`<h1 id="learning-paths-title">Learning paths</h1>`,
		`class="learning-paths__breadcrumb-title" href="/learn"`,
		`>Learning paths</a>`,
		`<h1 id="learn-graph-title">{{ if .SelectedPath }}{{ .SelectedPath.Name }}{{ else if .CombinePaths }}All my paths{{ else }}Full curriculum{{ end }}</h1>`,
		`href="/learn?path=mine"`,
		`>All my paths</strong>`,
		`>Your paths</p>`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("learning paths header is missing %q", contract)
		}
	}
	if strings.Contains(source, `class="ui-pane__eyebrow">Learning</p>`) {
		t.Error("learning paths should use one title instead of splitting its name across two styles")
	}
	if strings.Contains(source, `class="ui-pane learning-paths-pane"`) &&
		strings.Contains(source, `data-panel-max="28"`) {
		t.Error("learning paths panel should not retain the old narrow maximum")
	}
	if strings.Contains(source, "units in view") {
		t.Error("curriculum map should not show a redundant unit count")
	}

	stylesheet, err := os.ReadFile("../../web/static/css/learn.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		"align-items: flex-end;",
		"padding-bottom: calc(3.5rem + 2 * var(--space-3));",
	} {
		if !strings.Contains(string(stylesheet), contract) {
			t.Errorf("learning paths breadcrumb positioning is missing %q", contract)
		}
	}
}

func TestLearningPathEditorUsesOneActionRowAndDirectHeadings(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/learn.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`<h1 id="learning-path-editor-title">{{ if .EditingPath }}Edit path{{ else }}New path{{ end }}</h1>`,
		`<h2 class="editor-form__label" id="learning-path-targets-title">Target units</h2>`,
		`id="learning-path-form"`,
		`class="learning-path-form__targets"`,
		`class="editor-form-actions learning-path-form__actions"`,
		`type="submit" form="learning-path-form">Create learning path</button>`,
		`data-learning-path-delete-form`,
		`hx-trigger="input changed delay:600ms`,
		`hx-target="#learning-path-save-status"`,
		`template "learning-path-save-status" .PathSaveStatus`,
		`href="/learn?edit={{ .ID }}"`,
		`href="/learn?edit=new"`,
		`data-panel-motion="horizontal"`,
		`data-panel-navigation="close"`,
		`data-unit-picker-selected-ids="`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("learning path editor is missing %q", contract)
		}
	}
	for _, redundantHeading := range []string{
		`class="ui-pane__eyebrow">Learn</p>`,
		`class="section-heading__eyebrow">Current selection</p>`,
		`data-open-panel="learning-path-editor-panel"`,
		`data-learning-path-editor-title`,
		`form="learning-path-form">Save learning path`,
	} {
		if strings.Contains(source, redundantHeading) {
			t.Errorf("learning path editor retains redundant heading %q", redundantHeading)
		}
	}
	handler, err := os.ReadFile("../../internal/server/learn.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		`request.URL.Query().Get("edit")`,
		"data.ShowPathEditor = true",
		"data.EditingPath = &data.Paths[index]",
		`server.render(writer, "learning-path-save-status"`,
	} {
		if !strings.Contains(string(handler), contract) {
			t.Errorf("server-rendered learning path editor is missing %q", contract)
		}
	}

	stylesheet, err := os.ReadFile("../../web/static/css/components.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		".editor-form-actions {",
		"justify-content: flex-start;",
		"height: 2.625rem;",
		".editor-form-actions form,",
		".editor-form__label {",
	} {
		if !strings.Contains(string(stylesheet), contract) {
			t.Errorf("shared editor actions are missing %q", contract)
		}
	}

	learnStylesheet, err := os.ReadFile("../../web/static/css/learn.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		".learning-path-form {\n  gap: var(--space-5);",
		".learning-path-form__targets {\n  display: grid;\n  gap: var(--space-1);",
		".learning-path-form__targets .editor-form__field {\n  gap: var(--space-1);",
		".learning-path-card > a:not(.icon-button) {",
	} {
		if !strings.Contains(string(learnStylesheet), contract) {
			t.Errorf("learning path form spacing is missing %q", contract)
		}
	}

	components, err := os.ReadFile("../../web/templates/components.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(components), "learning_paths.js") {
		t.Error("learning path editor should not retain the ephemeral client-only controller")
	}
	for _, contract := range []string{
		`define "learning-path-save-status"`,
		`data-save-state-saved`,
		`data-save-state-saving`,
	} {
		if !strings.Contains(string(components), contract) {
			t.Errorf("learning path autosave status is missing %q", contract)
		}
	}

	picker, err := os.ReadFile("../../web/static/js/unit_picker.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		`new CustomEvent("unit-picker:change"`,
		"selectedIDs.size === 1",
	} {
		if !strings.Contains(string(picker), contract) {
			t.Errorf("unit picker autosave contract is missing %q", contract)
		}
	}
}

func TestPaneCapacityIsIndependentFromContentMeasure(t *testing.T) {
	for _, templatePath := range []string{
		"../../web/templates/account.html",
		"../../web/templates/admin-curriculum.html",
		"../../web/templates/learn.html",
	} {
		template, err := os.ReadFile(templatePath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(template), "data-panel-max") {
			t.Errorf("%s retains a hard panel maximum", templatePath)
		}
	}

	account, err := os.ReadFile("../../web/templates/account.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(account), `data-panel-fill data-panel-breadcrumb="Account"`) ||
		!strings.Contains(string(account), `data-pane-content-width="standard"`) {
		t.Error("account must fill its pane while retaining a standard content measure")
	}

	learn, err := os.ReadFile("../../web/templates/learn.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, measure := range []string{
		`data-pane-content-width="narrow"`,
		`data-pane-content-width="reading"`,
		`data-pane-content-width="wide"`,
	} {
		if !strings.Contains(string(learn), measure) {
			t.Errorf("learn is missing shared content measure %q", measure)
		}
	}

	layout, err := os.ReadFile("../../web/static/js/panel_layout.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(layout), "panelMax") {
		t.Error("panel allocator must not combine fill capacity with a hard maximum")
	}
}

func TestCollapsedPrimaryMenuPreservesVerticalSpacing(t *testing.T) {
	stylesheet, err := os.ReadFile("../../web/static/css/shell.css")
	if err != nil {
		t.Fatal(err)
	}
	source := string(stylesheet)
	selector := `.primary-navigation[data-panel-mode="icons"] .primary-menu > li {`
	start := strings.Index(source, selector)
	if start < 0 {
		t.Fatalf("shell styles are missing %q", selector)
	}
	end := strings.Index(source[start:], "}")
	if end < 0 {
		t.Fatal("collapsed primary menu rule is not closed")
	}
	if strings.Contains(source[start:start+end], "margin-top") {
		t.Error("collapsed primary menu overrides vertical spacing")
	}
	for _, contract := range []string{
		`justify-content: center;`,
		`position: relative;`,
		`transform: translateX(-50%);`,
		`line-height: 1;`,
		`.app-shell:not(.app-shell--home) .brand {`,
		`padding: 0.5rem 0.75rem;`,
		`.primary-navigation[data-panel-mode="icons"] .brand {`,
		`padding: 0;`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("collapsed primary menu alignment is missing %q", contract)
		}
	}
	if strings.Count(source, "view-transition-name: brand;") != 1 {
		t.Error("brand view transition should belong only to the visible name")
	}
	nameStart := strings.LastIndex(source, ".brand span {")
	if nameStart < 0 {
		t.Fatal("brand name styles are missing")
	}
	nameEnd := strings.Index(source[nameStart:], "}")
	if nameEnd < 0 ||
		!strings.Contains(source[nameStart:nameStart+nameEnd], "view-transition-name: brand;") {
		t.Error("brand name does not own its view transition")
	}
}

func TestAccountUsesOpenDetailsInsteadOfACard(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/account.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`<h1 id="account-title">Account</h1>`,
		`<dl class="definition-list">`,
		`<dt>Full name</dt>`,
		`<dt>Email</dt>`,
		`<dt>Access</dt>`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("account details are missing %q", contract)
		}
	}
	for _, obsoleteTreatment := range []string{
		`class="surface-card"`,
		`class="ui-pane__eyebrow">Settings</p>`,
	} {
		if strings.Contains(source, obsoleteTreatment) {
			t.Errorf("account retains obsolete treatment %q", obsoleteTreatment)
		}
	}

	stylesheet, err := os.ReadFile("../../web/static/css/components.css")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stylesheet), ".surface-card {") {
		t.Error("unused account card styling should be removed")
	}
}

func TestPanelControllerDoesNotOwnCurriculumEditing(t *testing.T) {
	controller, err := os.ReadFile("../../web/static/js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, domainDetail := range []string{"dependentId", "prerequisite", "/curriculum-modification"} {
		if strings.Contains(string(controller), domainDetail) {
			t.Errorf("generic panel controller contains domain detail %q", domainDetail)
		}
	}
}

func TestExpandedNavigationHasOneWidthContract(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/components.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(template), `data-panel-modes="mobile:0 icons:5 sidebar:13"`) {
		t.Error("primary navigation does not declare the 13rem sidebar mode")
	}

	stylesheet, err := os.ReadFile("../../web/static/css/base.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stylesheet), "--sidebar-width: 13rem;") {
		t.Error("initial navigation width does not match the negotiated sidebar mode")
	}
}

func TestInterfaceHeadingsShareOneTypeScale(t *testing.T) {
	base, err := os.ReadFile("../../web/static/css/base.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{
		"--font-size-pane-title: clamp(2rem, 3vw, 3rem);",
		"--font-size-section-title: 1.5rem;",
		"--font-size-subsection-title: 1rem;",
	} {
		if !strings.Contains(string(base), token) {
			t.Errorf("interface type scale is missing %q", token)
		}
	}

	stylesheets := map[string][]string{
		"../../web/static/css/shell.css": {
			".ui-pane h1 {",
			"font-size: var(--font-size-pane-title);",
		},
		"../../web/static/css/components.css": {
			".auth-form h1 {",
			".section-heading h2 {",
			".nested-panel__header h1 {",
			"font-size: var(--font-size-section-title);",
		},
		"../../web/static/css/curriculum.css": {
			".proposal-workspace__changes h2 {",
			"font-size: var(--font-size-subsection-title);",
		},
	}
	for path, contracts := range stylesheets {
		stylesheet, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, contract := range contracts {
			if !strings.Contains(string(stylesheet), contract) {
				t.Errorf("%s is missing shared heading contract %q", path, contract)
			}
		}
	}

	learn, err := os.ReadFile("../../web/static/css/learn.css")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(learn), ".learn-heading--path h1") {
		t.Error("selected learning paths should not shrink the shared pane title")
	}
}

func TestWhiteSurfacesShareOneRadius(t *testing.T) {
	stylesheets := []string{
		"../../web/static/css/components.css",
		"../../web/static/css/shell.css",
		"../../web/static/css/learn.css",
		"../../web/static/css/curriculum.css",
	}
	for _, path := range stylesheets {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "border-radius: var(--radius-surface);") {
			t.Errorf("%s does not use the shared surface radius", path)
		}
	}

	base, err := os.ReadFile("../../web/static/css/base.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(base), "--radius-surface: 1rem;") {
		t.Error("shared surface radius token is missing")
	}
	if strings.Contains(string(base), "--radius-card") {
		t.Error("legacy card-only radius remains defined")
	}

	shell, err := os.ReadFile("../../web/static/css/shell.css")
	if err != nil {
		t.Fatal(err)
	}
	menuStart := strings.Index(string(shell), "\n.primary-menu__link {")
	if menuStart < 0 {
		t.Fatal("primary menu link styles are missing")
	}
	menuEnd := strings.Index(string(shell)[menuStart:], "}")
	if menuEnd < 0 || !strings.Contains(string(shell)[menuStart:menuStart+menuEnd], "border-radius: var(--radius-surface);") {
		t.Error("primary menu options do not use the shared surface radius")
	}
}

func TestSelectableSurfacesShareOneState(t *testing.T) {
	for _, test := range []struct {
		path     string
		contract string
	}{
		{path: "../../web/templates/learn.html", contract: "learning-path-card selectable-surface"},
		{path: "../../web/templates/admin-curriculum.html", contract: "active-proposal-card selectable-surface"},
	} {
		source, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), test.contract) {
			t.Errorf("%s does not use shared selection surface %q", test.path, test.contract)
		}
	}

	base, err := os.ReadFile("../../web/static/css/base.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(base), "--color-selection-outline: oklch(") {
		t.Error("selection outline is not defined as a shared neutral oklch color")
	}

	for _, path := range []string{
		"../../web/static/css/learn.css",
		"../../web/static/css/curriculum.css",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "box-shadow: inset 0.2rem 0") ||
			strings.Contains(string(source), "box-shadow: inset 0 0 0 2px") {
			t.Errorf("%s retains a local selected-card outline", path)
		}
	}
}

func TestMobilePanelBreadcrumbsUseTheSharedLayoutContract(t *testing.T) {
	layout, err := os.ReadFile("../../web/static/js/panel_layout.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		`--mobile-panel-composition`,
		`layoutMobileGroup`,
		`panel-layout:complete`,
	} {
		if !strings.Contains(string(layout), contract) {
			t.Errorf("shared panel layout is missing mobile composition contract %q", contract)
		}
	}

	breadcrumbs, err := os.ReadFile("../../web/static/js/panel_breadcrumbs.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		`[data-layout-panel][data-panel-breadcrumb]`,
		`panel-layout:complete`,
		`panel:navigate`,
	} {
		if !strings.Contains(string(breadcrumbs), contract) {
			t.Errorf("mobile breadcrumb controller is missing contract %q", contract)
		}
	}

	for _, templatePath := range []string{
		"../../web/templates/account.html",
		"../../web/templates/admin-curriculum.html",
		"../../web/templates/learn.html",
	} {
		template, err := os.ReadFile(templatePath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(template), "collapsed:0") {
			t.Errorf("%s does not declare a zero-width panel mode", templatePath)
		}
		if !strings.Contains(string(template), "data-panel-breadcrumb=") {
			t.Errorf("%s does not declare a mobile breadcrumb label", templatePath)
		}
	}
}

func TestRestorableControllersDoNotSerializeInitializationState(t *testing.T) {
	for _, controllerPath := range []string{
		"../../web/static/js/curriculum_graph.js",
		"../../web/static/js/inline_editing.js",
		"../../web/static/js/panel_breadcrumbs.js",
		"../../web/static/js/panels.js",
		"../../web/static/js/shell.js",
		"../../web/static/js/unit_picker.js",
	} {
		controller, err := os.ReadFile(controllerPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(controller), "dataset.panelInitialized") ||
			strings.Contains(string(controller), "dataset.unitPickerInitialized") ||
			strings.Contains(string(controller), "dataset.inlineEditorInitialized") ||
			strings.Contains(string(controller), "dataset.curriculumGraphInitialized") ||
			strings.Contains(string(controller), "dataset.graphSearchInitialized") ||
			strings.Contains(string(controller), "dataset.menuInitialized") ||
			strings.Contains(string(controller), "dataset.breadcrumbSignature") {
			t.Errorf("%s serializes controller initialization state into HTMX history", controllerPath)
		}
	}
}

func TestShellReconcilesPanelGeometryBeforeRestoringTransitions(t *testing.T) {
	controller, err := os.ReadFile("../../web/static/js/shell.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(controller)
	refresh := strings.Index(source, `if (window.panelLayout) window.panelLayout.refresh();`)
	restore := strings.Index(source, `shell.classList.remove("is-shell-navigation");`)
	if refresh < 0 || restore < 0 || refresh > restore {
		t.Error("shell must reconcile settled panel widths before restoring CSS transitions")
	}
}

func TestHTMXKeepsCalculatedPanelGeometryDuringViewTransitions(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/components.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`<meta name="htmx-config" content='{"attributesToSettle":["class","width","height"]}'>`,
		"hx-swap=\"outerHTML transition:true\"\n         hx-push-url=\"true\"\n         data-graph-search-option",
		"hx-swap=\"outerHTML transition:true\"\n             hx-push-url=\"true\"\n             {{ if .IsCurrent }}",
		"hx-swap=\"outerHTML transition:true\"\n             hx-push-url=\"true\"\n             aria-label=\"Open content",
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("stable HTMX transition contract is missing %q", contract)
		}
	}
}

func TestUnitCompletionRefreshesWorkspaceWithoutViewTransition(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/components.html")
	if err != nil {
		t.Fatal(err)
	}
	source := string(template)
	for _, contract := range []string{
		`{{ define "unit-completion-update" -}}`,
		`hx-post="/learn/units/{{ .UnitID }}/completion"`,
		`hx-target="this"`,
		`hx-swap="outerHTML"`,
		`hx-swap-oob="outerHTML"`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("unit completion HTMX contract is missing %q", contract)
		}
	}
	completionFormStart := strings.Index(source, `{{ define "unit-completion-form" -}}`)
	if completionFormStart < 0 {
		t.Fatal("unit completion form not found")
	}
	completionFormEnd := strings.Index(source[completionFormStart:], `</form>`)
	if completionFormEnd < 0 {
		t.Fatal("unit completion form not found")
	}
	completionForm := source[completionFormStart : completionFormStart+completionFormEnd]
	if strings.Contains(completionForm, "transition:true") {
		t.Error("unit completion must not animate unchanged workspace geometry")
	}
	if strings.Contains(completionForm, `hx-target="#workspace"`) {
		t.Error("unit completion must not replace the workspace")
	}
}

func TestCurriculumGraphRendersSafeBundledBezierEdges(t *testing.T) {
	controller, err := os.ReadFile("../../web/static/js/curriculum_graph.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(controller)
	for _, contract := range []string{
		`const outgoingHubs = new Map()`,
		`const incomingHubs = new Map()`,
		`function directBezierPath(source, target)`,
		`function edgePath(edge, source, target)`,
		`if (hubSpan < branchInset)`,
		`if (sourceHubY > sourceY)`,
		`if (targetY > targetHubY)`,
		`function straightEdgePath(source, target)`,
		`function pathCrossesNode(path, edge)`,
		`path.setAttribute("d", straightEdgePath(source, target))`,
		`: rendered.edge.proposalState`,
		`? pathLayer.dataset.proposedArrowMarker`,
	} {
		if !strings.Contains(source, contract) {
			t.Errorf("curriculum graph is missing bundled-edge contract %q", contract)
		}
	}
	for _, routedEdgeDetail := range []string{
		`edge.dataset.lane`,
		`latestSafeBranchY`,
		`chamferedPath`,
		`const middleY =`,
	} {
		if strings.Contains(source, routedEdgeDetail) {
			t.Errorf("curriculum graph still contains routed-edge detail %q", routedEdgeDetail)
		}
	}
}

func TestProgressIndicatorsUseComposableConcentricCircles(t *testing.T) {
	stylesheet, err := os.ReadFile("../../web/static/css/learn.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []string{
		".unit-completion__indicator::before",
		".unit-completion__indicator.is-completed::before",
		".curriculum-graph__item.is-path-target .curriculum-graph__anchor::after",
		"height: 66.6667%;",
		"width: 33.3333%;",
	} {
		if !strings.Contains(string(stylesheet), contract) {
			t.Errorf("completion indicator is missing radius contract %q", contract)
		}
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
