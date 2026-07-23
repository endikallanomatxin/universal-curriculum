package services

import (
	"testing"

	"universal-curriculum/internal/models"
)

func TestCurriculumPathSubgraphContainsTargetsAndEveryPrerequisite(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Shared foundation"},
			{ID: 2, Name: "First branch"},
			{ID: 3, Name: "Second branch"},
			{ID: 4, Name: "First target"},
			{ID: 5, Name: "Second target"},
			{ID: 6, Name: "Unrelated"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 1},
			{UnitID: 4, PrerequisiteID: 2},
			{UnitID: 5, PrerequisiteID: 3},
			{UnitID: 6, PrerequisiteID: 1},
		},
	}

	subgraph := CurriculumPathSubgraph(graph, []int64{4, 5})

	visible := make(map[int64]bool)
	for _, unit := range subgraph.Units {
		visible[unit.ID] = true
	}
	for _, expected := range []int64{1, 2, 3, 4, 5} {
		if !visible[expected] {
			t.Fatalf("expected unit %d in path subgraph: %#v", expected, subgraph.Units)
		}
	}
	if visible[6] {
		t.Fatalf("unrelated unit leaked into path subgraph: %#v", subgraph.Units)
	}
	if len(subgraph.Dependencies) != 4 {
		t.Fatalf("dependencies = %#v, want four induced edges", subgraph.Dependencies)
	}
}

func TestCurriculumPathSubgraphIgnoresMissingAndRepeatedTargets(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{{ID: 1}, {ID: 2}},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
		},
	}

	subgraph := CurriculumPathSubgraph(graph, []int64{2, 2, 99})

	if len(subgraph.Units) != 2 || len(subgraph.Dependencies) != 1 {
		t.Fatalf("unexpected path subgraph: %#v", subgraph)
	}
}
