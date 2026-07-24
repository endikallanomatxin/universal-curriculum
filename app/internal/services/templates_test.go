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
			contains: []string{`id="app-shell"`, `class="app-shell app-shell--home"`, `class="brand__mark"`, `/static/images/universal-curriculum-logo.svg?v=`, `>universal curriculum</span>`, `>Learn</span>`, `>book_5</span>`, `>Account<`, `>Log out</span>`, `Material+Symbols+Rounded`, `/static/css/base.css?v=`, `/static/js/shell.js?v=`},
		},
		{
			name: "account.html",
			data: struct {
				User           *models.User
				CSRFToken      string
				CurrentSection string
				Home           bool
			}{User: user, CurrentSection: "account"},
			contains: []string{`data-panel-group`, `data-panel-modes="mobile:0 icons:5 sidebar:17"`, `data-mobile-menu-toggle`, `data-panel-fill`, `class="pane-stack"`, `hx-target="#workspace"`, `hx-swap="outerHTML transition:true"`, `/static/js/panel_breadcrumbs.js?v=`, `>person</span>`},
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

func TestProposalChangesExposeSemanticVisualStates(t *testing.T) {
	template, err := os.ReadFile("../../web/templates/admin-curriculum.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{
		"proposal-change--addition",
		"proposal-change--modification",
		"proposal-change--removal",
		"proposal-change__previous",
		"proposal-change__revert",
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
		".proposal-change--addition",
		".proposal-change--removal",
		"color: var(--proposal-change-color)",
		".proposal-list.proposal-change-list",
	} {
		if !strings.Contains(string(stylesheet), contract) {
			t.Errorf("proposal change styles are missing %q", contract)
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
		`class="ui-pane__eyebrow">Learning</p>`,
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

func TestPanelControllerDoesNotOwnCurriculumEditing(t *testing.T) {
	controller, err := os.ReadFile("../../web/static/js/panels.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, domainDetail := range []string{"dependentId", "prerequisite", "/admin/curriculum"} {
		if strings.Contains(string(controller), domainDetail) {
			t.Errorf("generic panel controller contains domain detail %q", domainDetail)
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
