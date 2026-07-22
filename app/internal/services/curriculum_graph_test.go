package services

import (
	"errors"
	"testing"

	"universal-curriculum/internal/models"
)

func TestBuildCurriculumGraphLayoutOrdersDependenciesFirst(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 3, Name: "Calculus"},
			{ID: 1, Name: "Foundations"},
			{ID: 4, Name: "Probability"},
			{ID: 2, Name: "Algebra"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 2},
			{UnitID: 4, PrerequisiteID: 1},
		},
	}
	layout, err := BuildCurriculumGraphLayout(graph)
	if err != nil {
		t.Fatal(err)
	}
	positions := make(map[int64]int)
	for index, node := range layout.Nodes {
		positions[node.ID] = index
	}
	for _, edge := range layout.Edges {
		if positions[edge.PrerequisiteID] >= positions[edge.DependentID] {
			t.Fatalf("dependency is not before dependent: %#v", edge)
		}
	}
	if layout.LaneCount < 1 {
		t.Fatalf("expected graph lanes, got %d", layout.LaneCount)
	}
}

func TestBuildCurriculumGraphLayoutRejectsCycles(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{{ID: 1, Name: "One"}, {ID: 2, Name: "Two"}},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 1, PrerequisiteID: 2},
		},
	}
	if _, err := BuildCurriculumGraphLayout(graph); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle returned %v", err)
	}
}

func TestAssignCurriculumNodeLanesAvoidsEdgesCrossingTheNodeRow(t *testing.T) {
	layout := &models.CurriculumGraphLayout{
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}, Lane: 0},
			{Unit: models.Unit{ID: 2}, Lane: 1},
			{Unit: models.Unit{ID: 3}},
			{Unit: models.Unit{ID: 4}},
			{Unit: models.Unit{ID: 5}},
		},
		Edges: []models.CurriculumGraphEdge{
			{PrerequisiteID: 1, DependentID: 5, Lane: 1},
			{PrerequisiteID: 2, DependentID: 3, Lane: 0},
		},
	}
	nodeIndexes := map[int64]int{1: 0, 2: 1, 3: 2, 4: 3, 5: 4}

	assignCurriculumNodeLanes(layout, nodeIndexes)

	if layout.Nodes[2].Lane == 1 {
		t.Fatal("dependent unit was placed on a lane crossing its row")
	}
}

func TestBuildCurriculumGraphLayoutKeepsUnitsOffRenderedDependencyLanes(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 5, Name: "Foundations of Logic"},
			{ID: 6, Name: "Set Theory"},
			{ID: 7, Name: "Functions and Relations"},
			{ID: 8, Name: "Linear Algebra"},
			{ID: 9, Name: "Limits"},
			{ID: 10, Name: "Differential Calculus"},
			{ID: 11, Name: "Probability Basics"},
			{ID: 12, Name: "Introduction to Programming"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 12, PrerequisiteID: 5},
			{UnitID: 6, PrerequisiteID: 5},
			{UnitID: 7, PrerequisiteID: 6},
			{UnitID: 11, PrerequisiteID: 6},
			{UnitID: 9, PrerequisiteID: 7},
			{UnitID: 8, PrerequisiteID: 7},
			{UnitID: 10, PrerequisiteID: 9},
		},
	}

	layout, err := BuildCurriculumGraphLayout(graph)
	if err != nil {
		t.Fatal(err)
	}
	assertNoCurriculumNodeLaneCollisions(t, layout)
	lanes := make(map[int64]float64, len(layout.Nodes))
	for _, node := range layout.Nodes {
		lanes[node.ID] = node.Lane
	}
	if lanes[9] != lanes[10] {
		t.Fatalf("straight descendants were split across lanes: Limits=%g Differential Calculus=%g", lanes[9], lanes[10])
	}
	if !(lanes[7] < lanes[9] && lanes[9] < lanes[6]) {
		t.Fatalf("branch was not inserted between its neighbouring lanes: Functions=%g Limits=%g Set Theory=%g", lanes[7], lanes[9], lanes[6])
	}
}

func assertNoCurriculumNodeLaneCollisions(t *testing.T, layout *models.CurriculumGraphLayout) {
	t.Helper()
	indexes := make(map[int64]int, len(layout.Nodes))
	for index, node := range layout.Nodes {
		indexes[node.ID] = index
	}
	for nodeIndex, node := range layout.Nodes {
		for _, edge := range layout.Edges {
			start, end := indexes[edge.PrerequisiteID], indexes[edge.DependentID]
			if start >= nodeIndex || nodeIndex >= end {
				continue
			}
			lane := edge.Lane
			if layout.Nodes[start].Lane == layout.Nodes[end].Lane {
				lane = layout.Nodes[start].Lane
			}
			if node.Lane == lane {
				t.Fatalf("unit %d occupies lane %g used by dependency %d -> %d", node.ID, lane, edge.PrerequisiteID, edge.DependentID)
			}
		}
	}
}
