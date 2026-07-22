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

func TestCurriculumErrorResponse(t *testing.T) {
	message, status := curriculumErrorResponse(&services.UnitIsPrerequisiteError{DependentNames: []string{"Algebra", "Calculus"}})
	if status != http.StatusConflict || message != "Remove the dependencies from Algebra and Calculus before deleting this unit." {
		t.Fatalf("curriculumErrorResponse() = %q, %d", message, status)
	}
	if message, status = curriculumErrorResponse(errors.New("database unavailable")); status != http.StatusInternalServerError || message == "" {
		t.Fatalf("unexpected internal error response: %q, %d", message, status)
	}
}
