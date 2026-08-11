package models

import "testing"

func TestCurriculumGraphOperationsAndIndex(t *testing.T) {
	graph := &CurriculumGraph{
		Units:        []Unit{{ID: 1, Name: "Foundation"}, {ID: 2, Name: "Target"}},
		Dependencies: []UnitDependency{{UnitID: 2, PrerequisiteID: 1}},
	}
	index := IndexCurriculumGraph(graph)
	if graph.Unit(2) == nil || index.Unit(2) == nil || !index.HasUnit(1) {
		t.Fatalf("unit lookup failed: graph=%#v index=%#v", graph.Unit(2), index.Unit(2))
	}
	if !graph.HasDependency(2, 1) || !index.HasDependency(2, 1) {
		t.Fatal("dependency lookup failed")
	}
	if got := index.Prerequisites(2); len(got) != 1 || got[0] != 1 {
		t.Fatalf("prerequisites = %v", got)
	}
	if got := index.Dependents(1); len(got) != 1 || got[0] != 2 {
		t.Fatalf("dependents = %v", got)
	}
	clone := graph.Clone()
	clone.Units[0].Name = "Changed"
	clone.Dependencies[0].PrerequisiteID = 9
	if graph.Units[0].Name != "Foundation" || graph.Dependencies[0].PrerequisiteID != 1 {
		t.Fatal("clone shares graph slices with its source")
	}
}

func TestNilCurriculumGraphOperationsAreSafe(t *testing.T) {
	var graph *CurriculumGraph
	if graph.Unit(1) != nil || graph.HasDependency(2, 1) || graph.Clone() != nil {
		t.Fatal("nil graph operations returned data")
	}
	index := IndexCurriculumGraph(nil)
	if index.HasUnit(1) || index.HasDependency(2, 1) || index.Unit(1) != nil {
		t.Fatal("nil graph index returned data")
	}
}
