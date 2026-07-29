package models

import "time"

type CurriculumProposal struct {
	ID                int64
	AuthorID          *int64
	AuthorName        string
	Title             string
	Rationale         string
	Status            string
	BaseProposalID    *int64
	RevertsProposalID *int64
	CreatedAt         time.Time
	AcceptedAt        *time.Time
	Changes           []CurriculumProposalChange
	ChangeCount       int
	CanRevert         bool
}

type CurriculumProposalChange struct {
	ID                  int64
	ProposalID          int64
	Position            int
	Kind                string
	UnitID              int64
	UnitName            string
	PreviousUnitName    string
	UnitContent         string
	PreviousUnitContent string
	PrerequisiteID      *int64
}
