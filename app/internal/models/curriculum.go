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
	Direct      bool
	Transferred bool
}

func (status UnitCompletionStatus) Completed() bool {
	return status.Direct || status.Transferred
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
