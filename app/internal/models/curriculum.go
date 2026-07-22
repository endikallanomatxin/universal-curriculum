package models

import "time"

type Unit struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	Nodes     []CurriculumGraphNode
	Edges     []CurriculumGraphEdge
	LaneCount int
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
