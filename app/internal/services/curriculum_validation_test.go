package services

import (
	"errors"
	"testing"

	"universal-curriculum/internal/models"
)

func TestValidateCurriculumProposalAcceptsCoherentOrderedChanges(t *testing.T) {
	base := validationTestGraph()
	prerequisiteID := int64(1)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{
			ID: 10, Position: 1, Kind: "create_unit", UnitID: 10,
			UnitName: "Geometry", UnitContent: "Learn shapes.",
		},
		{
			ID: 11, Position: 2, Kind: "rename_unit", UnitID: 1,
			PreviousUnitName: "Foundations", UnitName: "Mathematical foundations",
		},
		{
			ID: 12, Position: 3, Kind: "update_content", UnitID: 2,
			PreviousUnitContent: "Learn variables.", UnitContent: "Learn variables and equations.",
		},
		{
			ID: 13, Position: 4, Kind: "add_dependency", UnitID: 10,
			PrerequisiteID: &prerequisiteID,
		},
	}}

	if err := validateCurriculumProposal(base, proposal); err != nil {
		t.Fatalf("validate coherent proposal: %v", err)
	}
}

func TestValidateCurriculumProposalAcceptsExplicitPrerequisiteResolutionBeforeDeletion(t *testing.T) {
	base := validationTestGraph()
	prerequisiteID := int64(1)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{ID: 10, Position: 1, Kind: "remove_dependency", UnitID: 2, PrerequisiteID: &prerequisiteID},
		{
			ID: 11, Position: 2, Kind: "delete_unit", UnitID: 1,
			UnitName: "Foundations", UnitContent: "Learn the basics.",
		},
	}}

	if err := validateCurriculumProposal(base, proposal); err != nil {
		t.Fatalf("validate coherent deletion: %v", err)
	}
}

func TestValidateCurriculumProposalRejectsIncoherentChanges(t *testing.T) {
	prerequisiteOne := int64(1)
	prerequisiteTwo := int64(2)
	tests := []struct {
		name    string
		changes []models.CurriculumProposalChange
	}{
		{
			name: "missing unit",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Position: 1, Kind: "rename_unit", UnitID: 99,
				PreviousUnitName: "Missing", UnitName: "Still missing",
			}},
		},
		{
			name: "stale previous value",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Position: 1, Kind: "rename_unit", UnitID: 1,
				PreviousUnitName: "Old foundations", UnitName: "New foundations",
			}},
		},
		{
			name: "no effect",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Position: 1, Kind: "update_content", UnitID: 1,
				PreviousUnitContent: "Learn the basics.", UnitContent: "Learn the basics.",
			}},
		},
		{
			name: "created then deleted",
			changes: []models.CurriculumProposalChange{
				{
					ID: 10, Position: 1, Kind: "create_unit", UnitID: 10,
					UnitName: "Temporary", UnitContent: "Temporary content.",
				},
				{
					ID: 11, Position: 2, Kind: "delete_unit", UnitID: 10,
					UnitName: "Temporary", UnitContent: "Temporary content.",
				},
			},
		},
		{
			name: "dependency cycle",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Position: 1, Kind: "add_dependency", UnitID: 1,
				PrerequisiteID: &prerequisiteTwo,
			}},
		},
		{
			name: "dependency changed twice",
			changes: []models.CurriculumProposalChange{
				{ID: 10, Position: 1, Kind: "remove_dependency", UnitID: 2, PrerequisiteID: &prerequisiteOne},
				{ID: 11, Position: 2, Kind: "add_dependency", UnitID: 2, PrerequisiteID: &prerequisiteOne},
			},
		},
		{
			name: "delete prerequisite still in use",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Position: 1, Kind: "delete_unit", UnitID: 1,
				UnitName: "Foundations", UnitContent: "Learn the basics.",
			}},
		},
		{
			name: "change superseded by deletion",
			changes: []models.CurriculumProposalChange{
				{
					ID: 10, Position: 1, Kind: "rename_unit", UnitID: 3,
					PreviousUnitName: "Geometry", UnitName: "Euclidean geometry",
				},
				{
					ID: 11, Position: 2, Kind: "delete_unit", UnitID: 3,
					UnitName: "Euclidean geometry", UnitContent: "Learn shapes.",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCurriculumProposal(
				validationTestGraph(),
				&models.CurriculumProposal{Changes: test.changes},
			)
			if !errors.Is(err, ErrProposalInvalid) {
				t.Fatalf("validation error = %v, want %v", err, ErrProposalInvalid)
			}
		})
	}
}

func validationTestGraph() *models.CurriculumGraph {
	return &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations", Content: "Learn the basics."},
			{ID: 2, Name: "Algebra", Content: "Learn variables."},
			{ID: 3, Name: "Geometry", Content: "Learn shapes."},
		},
		Dependencies: []models.UnitDependency{{
			UnitID: 2, PrerequisiteID: 1,
		}},
	}
}
