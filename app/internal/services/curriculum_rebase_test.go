package services

import (
	"slices"
	"testing"

	"universal-curriculum/internal/models"
)

func TestCurriculumProposalRebaseFootprintIncludesRelationshipEndpoints(t *testing.T) {
	prerequisiteID := int64(2)
	tests := []struct {
		name   string
		change models.CurriculumProposalChange
		want   []int64
	}{
		{
			name:   "unit change",
			change: models.CurriculumProposalChange{Kind: "update_content", UnitID: 1},
			want:   []int64{1},
		},
		{
			name: "dependency",
			change: models.CurriculumProposalChange{
				Kind: "add_dependency", UnitID: 1, PrerequisiteID: &prerequisiteID,
			},
			want: []int64{1, 2},
		},
		{
			name: "recognition",
			change: models.CurriculumProposalChange{
				Kind: "recognition",
				Recognition: &models.Recognition{
					Sources: []models.Unit{{ID: 1}, {ID: 2}},
					Targets: []models.Unit{{ID: 2}, {ID: 3}},
				},
			},
			want: []int64{1, 2, 3},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := curriculumChangeUnitIDs(test.change)
			if !slices.Equal(got, test.want) {
				t.Fatalf("rebase footprint = %v, want %v", got, test.want)
			}
		})
	}
}
