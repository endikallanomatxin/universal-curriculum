package server

import (
	"errors"
	"net/http"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

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
		{Kind: "update_unit", UnitID: 1, UnitName: "Mathematical foundations"},
		{Kind: "update_content", UnitID: 2, UnitContent: "Revised"},
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
	if graph.Units[0].Name != "Foundations" {
		t.Fatal("proposal preview mutated the published graph")
	}
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

func TestProposalGraphStatesUseStructuralChangePrecedence(t *testing.T) {
	view := curriculumGraphView{Nodes: []curriculumGraphNodeView{
		{CurriculumGraphNode: models.CurriculumGraphNode{Unit: models.Unit{ID: 1}}},
		{CurriculumGraphNode: models.CurriculumGraphNode{Unit: models.Unit{ID: 2}}},
	}}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "update_content", UnitID: 1},
		{Kind: "update_unit", UnitID: 1},
		{Kind: "update_content", UnitID: 2},
		{Kind: "delete_unit", UnitID: 2},
	}}

	applyProposalGraphStates(&view, proposal)

	if view.Nodes[0].ProposalState != "rename" || view.Nodes[1].ProposalState != "deleted" {
		t.Fatalf("proposal states = %q, %q", view.Nodes[0].ProposalState, view.Nodes[1].ProposalState)
	}
}

func TestCurriculumErrorResponse(t *testing.T) {
	message, status := curriculumErrorResponse(&services.UnitIsPrerequisiteError{DependentNames: []string{"Algebra", "Calculus"}})
	if status != http.StatusConflict || message != "Remove the dependencies from Algebra and Calculus before deleting this unit." {
		t.Fatalf("curriculumErrorResponse() = %q, %d", message, status)
	}
	if message, status = curriculumErrorResponse(errors.New("database unavailable")); status != http.StatusInternalServerError || message == "" {
		t.Fatalf("unexpected internal error response: %q, %d", message, status)
	}
}
