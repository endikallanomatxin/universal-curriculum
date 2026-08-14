package models

import "time"

type Unit struct {
	ID        int64
	Name      string
	Content   string
	Retired   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UnitCompletionStatus struct {
	Direct     bool
	Recognized bool
}

func (status UnitCompletionStatus) Completed() bool {
	return status.Direct || status.Recognized
}

type UnitDependency struct {
	UnitID           int64
	UnitName         string
	PrerequisiteID   int64
	PrerequisiteName string
}

type CurriculumGraph struct {
	Units        []Unit
	Dependencies []UnitDependency
}

func (graph *CurriculumGraph) Unit(unitID int64) *Unit {
	if graph == nil {
		return nil
	}
	for index := range graph.Units {
		if graph.Units[index].ID == unitID {
			return &graph.Units[index]
		}
	}
	return nil
}

func (graph *CurriculumGraph) HasDependency(unitID, prerequisiteID int64) bool {
	if graph == nil {
		return false
	}
	for _, dependency := range graph.Dependencies {
		if dependency.UnitID == unitID && dependency.PrerequisiteID == prerequisiteID {
			return true
		}
	}
	return false
}

func (graph *CurriculumGraph) Clone() *CurriculumGraph {
	if graph == nil {
		return nil
	}
	return &CurriculumGraph{
		Units:        append([]Unit(nil), graph.Units...),
		Dependencies: append([]UnitDependency(nil), graph.Dependencies...),
	}
}

// CurriculumGraphIndex is a read-only index over one graph snapshot. Build a
// new index after mutating the graph's unit or dependency slices.
type CurriculumGraphIndex struct {
	units         map[int64]*Unit
	prerequisites map[int64][]int64
	dependents    map[int64][]int64
	dependencies  map[[2]int64]bool
}

func IndexCurriculumGraph(graph *CurriculumGraph) *CurriculumGraphIndex {
	index := &CurriculumGraphIndex{
		units:         make(map[int64]*Unit),
		prerequisites: make(map[int64][]int64),
		dependents:    make(map[int64][]int64),
		dependencies:  make(map[[2]int64]bool),
	}
	if graph == nil {
		return index
	}
	for position := range graph.Units {
		unit := &graph.Units[position]
		index.units[unit.ID] = unit
	}
	for _, dependency := range graph.Dependencies {
		index.prerequisites[dependency.UnitID] = append(index.prerequisites[dependency.UnitID], dependency.PrerequisiteID)
		index.dependents[dependency.PrerequisiteID] = append(index.dependents[dependency.PrerequisiteID], dependency.UnitID)
		index.dependencies[[2]int64{dependency.UnitID, dependency.PrerequisiteID}] = true
	}
	return index
}

func (index *CurriculumGraphIndex) Unit(unitID int64) *Unit {
	if index == nil {
		return nil
	}
	return index.units[unitID]
}

func (index *CurriculumGraphIndex) HasUnit(unitID int64) bool {
	return index != nil && index.units[unitID] != nil
}

func (index *CurriculumGraphIndex) HasDependency(unitID, prerequisiteID int64) bool {
	return index != nil && index.dependencies[[2]int64{unitID, prerequisiteID}]
}

func (index *CurriculumGraphIndex) Prerequisites(unitID int64) []int64 {
	if index == nil {
		return nil
	}
	return index.prerequisites[unitID]
}

func (index *CurriculumGraphIndex) Dependents(unitID int64) []int64 {
	if index == nil {
		return nil
	}
	return index.dependents[unitID]
}

type CurriculumGraphLayout struct {
	Nodes      []CurriculumGraphNode
	Edges      []CurriculumGraphEdge
	Boundaries []CurriculumGraphBoundary
	LaneCount  int
}

type CurriculumGraphBoundary struct {
	UnitID    int64
	Direction string
	Count     int
}

type CurriculumGraphNode struct {
	Unit
	Lane float64
}

type CurriculumGraphEdge struct {
	PrerequisiteID int64
	DependentID    int64
	Lane           float64
}
