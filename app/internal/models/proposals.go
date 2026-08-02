package models

import "time"

type CurriculumProposal struct {
	ID             int64
	AuthorIDs      []int64
	AuthorName     string
	Title          string
	Rationale      string
	Status         string
	BaseProposalID *int64
	CreatedAt      time.Time
	AcceptedAt     *time.Time
	Changes        []CurriculumProposalChange
	ChangeCount    int
}

func (proposal CurriculumProposal) HasAuthor(userID int64) bool {
	for _, authorID := range proposal.AuthorIDs {
		if authorID == userID {
			return true
		}
	}
	return false
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
	PrerequisiteName    string
	Recognition         *Recognition
}

type Recognition struct {
	Rationale string
	Sources   []Unit
	Targets   []Unit
}
