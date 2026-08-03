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

func TestCurriculumModificationUsesProtectedRoutes(t *testing.T) {
	handler := (&Server{}).routes()

	for _, test := range []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/curriculum-modification"},
		{method: http.MethodPost, target: "/curriculum-modification/proposals"},
		{method: http.MethodPost, target: "/curriculum-modification/proposals/1/rebase"},
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

func TestDeletedProposalUnitsRemainVisibleForInspection(t *testing.T) {
	published := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations", Content: "Original foundations."},
			{ID: 2, Name: "Algebra", Content: "Original algebra."},
		},
		Dependencies: []models.UnitDependency{{UnitID: 2, PrerequisiteID: 1}},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{{
		Kind: "delete_unit", UnitID: 2,
	}}}
	working := curriculumGraphWithProposal(published, proposal)

	visual := curriculumGraphWithRemovedDependencies(working, published, proposal)

	if graphUnitByID(visual, 2) == nil || len(visual.Dependencies) != 1 {
		t.Fatalf("deleted unit context is missing from proposal graph: %#v", visual)
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
	if view.Edges[0].ProposalState != "deleted" {
		t.Fatal("edge connected to deleted unit is not marked as deleted")
	}
}

func TestApplyUnitContentDiffUsesMatchingProposalChange(t *testing.T) {
	unit := &curriculumUnitView{Unit: models.Unit{ID: 2, Content: "Proposed explanation"}}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "update_content", UnitID: 1, PreviousUnitContent: "Another unit"},
		{Kind: "update_content", UnitID: 2, PreviousUnitContent: "Published explanation"},
	}}

	applyUnitContentDiff(unit, proposal)

	if !unit.HasContentDiff || unit.PreviousContent != "Published explanation" {
		t.Fatalf("content diff state = %#v", unit)
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

func TestCurriculumProposalHistoryShowsAcceptedLineAndDraftBranches(t *testing.T) {
	baseID := int64(1)
	history, roots := curriculumProposalHistory(
		[]models.CurriculumProposal{
			{ID: 2, Title: "Second", Status: "accepted", BaseProposalID: &baseID},
			{ID: 1, Title: "First", Status: "accepted"},
		},
		[]curriculumDraftProposalView{
			{CurriculumProposal: models.CurriculumProposal{ID: 3, BaseProposalID: &baseID}},
			{CurriculumProposal: models.CurriculumProposal{ID: 4}},
		},
	)

	if len(history) != 2 || history[0].ID != 1 || history[1].ID != 2 || !history[1].IsHead {
		t.Fatalf("accepted proposal history = %#v", history)
	}
	if len(history[0].Drafts) != 1 || history[0].Drafts[0].ID != 3 {
		t.Fatalf("draft branches = %#v", history[0].Drafts)
	}
	if len(roots) != 1 || roots[0].ID != 4 {
		t.Fatalf("root drafts = %#v", roots)
	}
}

func TestCurriculumRebaseTimelineKeepsConflictsAndCompressesOtherAcceptedWork(t *testing.T) {
	base := models.CurriculumProposal{ID: 1, Title: "Original base", Status: "accepted"}
	draft := models.CurriculumProposal{ID: 9, Title: "Working draft", Status: "draft"}
	plan := &services.CurriculumProposalRebasePlan{
		Status:       services.ProposalRebaseNeedsReview,
		BaseProposal: &base,
		AcceptedProposals: []models.CurriculumProposal{
			{ID: 2, Title: "Unrelated first"},
			{ID: 3, Title: "Overlapping work"},
			{ID: 4, Title: "Unrelated second"},
			{ID: 5, Title: "Current head"},
		},
		Conflicts: []services.CurriculumProposalRebaseConflict{{
			AcceptedWork: []services.CurriculumProposalRebaseAcceptedWork{{
				Proposal: models.CurriculumProposal{ID: 3, Title: "Overlapping work"},
			}},
		}},
	}

	view := curriculumRebaseTimeline(plan, &draft)
	if view == nil || view.BaseTitle != "Original base" || view.DraftTitle != "Working draft" {
		t.Fatalf("rebase timeline identity = %#v", view)
	}
	if len(view.Items) != 4 || !view.Items[0].Ellipsis ||
		view.Items[1].Title != "Overlapping work" || view.Items[1].Current || !view.Items[1].Conflicts ||
		!view.Items[2].Ellipsis || view.Items[3].Title != "Current head" || !view.Items[3].Current {
		t.Fatalf("rebase timeline items = %#v", view.Items)
	}
	if len(view.Edges) != 3 || view.Edges[0].Source != "base" || view.Edges[0].Target != "draft" ||
		view.Edges[1].Target != "accepted-3" || view.Edges[2].Target != "accepted-5" {
		t.Fatalf("rebase timeline edges = %#v", view.Edges)
	}
	related := visibleRebaseProposal(plan, 3)
	if related == nil || related.Title != "Overlapping work" {
		t.Fatalf("related conflicting proposal = %#v", related)
	}
	current := visibleRebaseProposal(plan, 5)
	if current == nil || current.Title != "Current head" {
		t.Fatalf("visible current proposal = %#v", current)
	}
	if hidden := visibleRebaseProposal(plan, 4); hidden != nil {
		t.Fatalf("hidden proposal should not be inspectable: %#v", hidden)
	}
	if relatedBase := visibleRebaseProposal(plan, 1); relatedBase == nil || relatedBase.Title != "Original base" {
		t.Fatalf("visible base proposal = %#v", relatedBase)
	}
}
