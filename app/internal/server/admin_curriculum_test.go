package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func TestCurriculumModificationUsesCleanAdminProtectedRoutes(t *testing.T) {
	handler := (&Server{}).routes()

	for _, test := range []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/curriculum-modification"},
		{method: http.MethodPost, target: "/curriculum-modification/proposals"},
	} {
		request := httptest.NewRequest(test.method, test.target, nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusSeeOther {
			t.Errorf("%s %s status = %d, want %d", test.method, test.target, recorder.Code, http.StatusSeeOther)
		}
		wantLocation := "/auth/login?next=" + url.QueryEscape(test.target)
		if location := recorder.Header().Get("Location"); location != wantLocation {
			t.Errorf("%s %s location = %q, want %q", test.method, test.target, location, wantLocation)
		}
	}
}

func TestCurriculumUnitViewsConnectBothDirections(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{{ID: 1, Name: "Foundations"}, {ID: 2, Name: "Algebra"}},
		Dependencies: []models.UnitDependency{{
			UnitID: 2, UnitName: "Algebra", PrerequisiteID: 1, PrerequisiteName: "Foundations",
		}},
	}
	views := curriculumUnitViews(graph, nil)
	if len(views) != 2 || len(views[0].Dependents) != 1 || views[0].Dependents[0].ID != 2 {
		t.Fatalf("prerequisite view does not contain its dependent: %#v", views)
	}
	if len(views[1].Prerequisites) != 1 || views[1].Prerequisites[0].ID != 1 {
		t.Fatalf("dependent view does not contain its prerequisite: %#v", views)
	}
}

func TestCurriculumGraphWithProposalIncludesAndPreviewsChangedUnits(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations", Content: "Original"},
			{ID: 2, Name: "Algebra", Content: "Original"},
		},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "create_unit", UnitID: 3, UnitName: "Geometry", UnitContent: "New"},
		{Kind: "rename_unit", UnitID: 1, UnitName: "Mathematical foundations"},
		{Kind: "update_content", UnitID: 2, UnitContent: "Revised"},
		{Kind: "add_dependency", UnitID: 3, PrerequisiteID: pointerToInt64(1)},
	}}

	preview := curriculumGraphWithProposal(graph, proposal)
	if len(preview.Units) != 3 {
		t.Fatalf("preview unit count = %d, want 3", len(preview.Units))
	}
	if preview.Units[0].Name != "Mathematical foundations" {
		t.Fatalf("renamed unit = %q", preview.Units[0].Name)
	}
	if preview.Units[1].Content != "Revised" {
		t.Fatalf("updated content = %q", preview.Units[1].Content)
	}
	if preview.Units[2].Name != "Geometry" || preview.Units[2].Content != "New" {
		t.Fatalf("created unit = %#v", preview.Units[2])
	}
	if len(preview.Dependencies) != 1 || preview.Dependencies[0].UnitID != 3 || preview.Dependencies[0].PrerequisiteID != 1 {
		t.Fatalf("proposed dependencies = %#v", preview.Dependencies)
	}
	if graph.Units[0].Name != "Foundations" {
		t.Fatal("proposal preview mutated the published graph")
	}
}

func TestCurriculumGraphWithProposalRemovesDeletedUnitsAndDependencies(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations"},
			{ID: 2, Name: "Algebra"},
			{ID: 3, Name: "Geometry"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 2},
		},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{{
		Kind: "delete_unit", UnitID: 2,
	}}}

	preview := curriculumGraphWithProposal(graph, proposal)

	if len(preview.Units) != 2 || preview.Units[0].ID != 1 || preview.Units[1].ID != 3 {
		t.Fatalf("preview units = %#v", preview.Units)
	}
	if len(preview.Dependencies) != 0 {
		t.Fatalf("preview retained deleted unit dependencies: %#v", preview.Dependencies)
	}
}

func pointerToInt64(value int64) *int64 {
	return &value
}

func TestCreatedProposalUnitsAreAlwaysIncludedInVisibleGraph(t *testing.T) {
	visible := &models.CurriculumGraph{Units: []models.Unit{{ID: 1, Name: "Focused"}}}
	working := &models.CurriculumGraph{Units: []models.Unit{
		{ID: 1, Name: "Focused"},
		{ID: 3, Name: "Proposed"},
	}}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "create_unit", UnitID: 3},
	}}

	includeCreatedProposalUnits(visible, working, proposal)
	includeCreatedProposalUnits(visible, working, proposal)

	if len(visible.Units) != 2 || visible.Units[1].ID != 3 {
		t.Fatalf("visible proposal units = %#v", visible.Units)
	}
}

func TestCreatedProposalUnitsBringTheirDependenciesIntoView(t *testing.T) {
	visible := &models.CurriculumGraph{Units: []models.Unit{{ID: 1, Name: "Focused"}}}
	working := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Focused"},
			{ID: 2, Name: "Published prerequisite"},
			{ID: 3, Name: "Proposed dependent"},
		},
		Dependencies: []models.UnitDependency{{UnitID: 3, PrerequisiteID: 2}},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "create_unit", UnitID: 3},
	}}

	includeCreatedProposalUnits(visible, working, proposal)

	if len(visible.Units) != 3 || len(visible.Dependencies) != 1 {
		t.Fatalf("connected proposed unit was not integrated: %#v", visible)
	}
	if visible.Dependencies[0].PrerequisiteID != 2 || visible.Dependencies[0].UnitID != 3 {
		t.Fatalf("visible proposed dependency = %#v", visible.Dependencies[0])
	}
}

func TestProposedUnitsDoNotLeaveBoundariesAfterTheyBecomeVisible(t *testing.T) {
	visual := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 3, Name: "Proposed prerequisite"},
			{ID: 4, Name: "Proposed dependent"},
		},
		Dependencies: []models.UnitDependency{{UnitID: 4, PrerequisiteID: 3}},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "create_unit", UnitID: 3},
		{Kind: "create_unit", UnitID: 4},
	}}

	visible, _, initialBoundaries, err := services.CurriculumNeighborhood(visual, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(initialBoundaries) != 1 {
		t.Fatalf("initial boundaries = %#v, want the hidden proposed dependent", initialBoundaries)
	}

	includeCreatedProposalUnits(visible, visual, proposal)
	boundaries := services.CurriculumGraphBoundaries(visual, visible)

	if len(visible.Units) != 2 || len(visible.Dependencies) != 1 {
		t.Fatalf("proposed units were not made visible: %#v", visible)
	}
	if len(boundaries) != 0 {
		t.Fatalf("visible proposed dependency left a boundary: %#v", boundaries)
	}
}

func TestIsolatedCreatedUnitsArePositionedBeforeConnectedGraph(t *testing.T) {
	layout := &models.CurriculumGraphLayout{
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}},
			{Unit: models.Unit{ID: 2}},
			{Unit: models.Unit{ID: 3}},
		},
		Edges: []models.CurriculumGraphEdge{{PrerequisiteID: 1, DependentID: 2}},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "create_unit", UnitID: 3},
	}}

	positionIsolatedCreatedUnits(layout, proposal)

	if layout.Nodes[0].ID != 3 || layout.Nodes[1].ID != 1 || layout.Nodes[2].ID != 2 {
		t.Fatalf("positioned nodes = %#v", layout.Nodes)
	}
}

func TestRemovedDependencyBetweenProposedUnitsRemainsVisible(t *testing.T) {
	working := &models.CurriculumGraph{Units: []models.Unit{
		{ID: 3, Name: "Proposed prerequisite"},
		{ID: 4, Name: "Proposed dependent"},
	}}
	prerequisiteID := int64(3)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "create_unit", UnitID: 3},
		{Kind: "create_unit", UnitID: 4},
		{Kind: "add_dependency", UnitID: 4, PrerequisiteID: &prerequisiteID},
		{Kind: "remove_dependency", UnitID: 4, PrerequisiteID: &prerequisiteID},
	}}

	visual := curriculumGraphWithRemovedDependencies(working, &models.CurriculumGraph{}, proposal)

	if len(visual.Dependencies) != 1 ||
		visual.Dependencies[0].PrerequisiteID != 3 ||
		visual.Dependencies[0].UnitID != 4 {
		t.Fatalf("removed proposed dependency is not visible: %#v", visual.Dependencies)
	}
	view := curriculumGraphView{Edges: []curriculumGraphEdgeView{{
		CurriculumGraphEdge: models.CurriculumGraphEdge{PrerequisiteID: 3, DependentID: 4},
	}}}
	applyProposalGraphStates(&view, proposal)
	if view.Edges[0].ProposalState != "deleted" {
		t.Fatalf("removed proposed dependency state = %q", view.Edges[0].ProposalState)
	}
}

func TestProposalGraphStatesUseStructuralChangePrecedence(t *testing.T) {
	view := curriculumGraphView{Nodes: []curriculumGraphNodeView{
		{CurriculumGraphNode: models.CurriculumGraphNode{Unit: models.Unit{ID: 1}}},
		{CurriculumGraphNode: models.CurriculumGraphNode{Unit: models.Unit{ID: 2}}},
	}, Edges: []curriculumGraphEdgeView{
		{CurriculumGraphEdge: models.CurriculumGraphEdge{PrerequisiteID: 1, DependentID: 2}},
	}}
	prerequisiteID := int64(1)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "update_content", UnitID: 1},
		{Kind: "rename_unit", UnitID: 1},
		{Kind: "update_content", UnitID: 2},
		{Kind: "delete_unit", UnitID: 2},
		{Kind: "add_dependency", UnitID: 2, PrerequisiteID: &prerequisiteID},
	}}

	applyProposalGraphStates(&view, proposal)

	if view.Nodes[0].ProposalState != "rename" || view.Nodes[1].ProposalState != "deleted" {
		t.Fatalf("proposal states = %q, %q", view.Nodes[0].ProposalState, view.Nodes[1].ProposalState)
	}
	if view.Edges[0].ProposalState != "created" {
		t.Fatal("added dependency edge is not marked as proposed")
	}
}

func TestCurriculumErrorResponse(t *testing.T) {
	message, status := curriculumErrorResponse(&services.UnitIsPrerequisiteError{DependentNames: []string{"Algebra", "Calculus"}})
	if status != http.StatusConflict || message != "Remove the dependencies from Algebra and Calculus before deleting this unit." {
		t.Fatalf("curriculumErrorResponse() = %q, %d", message, status)
	}
	validationError := &services.ProposalValidationError{ChangeID: 12, Reason: "the dependency creates a cycle"}
	if message, status = curriculumErrorResponse(validationError); status != http.StatusConflict || message != validationError.Error() {
		t.Fatalf("proposal validation response = %q, %d", message, status)
	}
	if message, status = curriculumErrorResponse(errors.New("database unavailable")); status != http.StatusInternalServerError || message == "" {
		t.Fatalf("unexpected internal error response: %q, %d", message, status)
	}
}
