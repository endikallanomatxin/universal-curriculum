package server

import (
	"testing"

	"universal-curriculum/internal/models"
)

func TestHomeLearningRecommendationsListAvailableUnitsForIncompletePaths(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations"},
			{ID: 2, Name: "Algebra"},
			{ID: 3, Name: "Completed target"},
		},
		Dependencies: []models.UnitDependency{{UnitID: 2, PrerequisiteID: 1}},
	}
	paths := []models.LearningPath{
		{ID: 7, Name: "Mathematics", Units: []models.Unit{{ID: 2}}},
		{ID: 8, Name: "Finished", Units: []models.Unit{{ID: 3}}},
	}

	recommendations := newHomeLearningRecommendations(paths, graph, map[int64]bool{3: true})

	if len(recommendations) != 1 {
		t.Fatalf("recommendations = %#v, want one incomplete path", recommendations)
	}
	recommendation := recommendations[0]
	if recommendation.ID != 7 || recommendation.PendingCount != 2 ||
		recommendation.URL != "/learn?path=7" || len(recommendation.NextUnits) != 1 {
		t.Fatalf("unexpected path recommendation: %#v", recommendation)
	}
	next := recommendation.NextUnits[0]
	if next.ID != 1 || next.URL != "/learn?path=7&unit=1&content=1" {
		t.Fatalf("unexpected next unit: %#v", next)
	}
}
