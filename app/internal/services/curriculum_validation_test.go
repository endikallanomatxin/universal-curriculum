package services

import (
	"errors"
	"testing"

	"universal-curriculum/internal/models"
)

func TestPopulateCurriculumProposalPreviousStateReplaysBase(t *testing.T) {
	base := &models.CurriculumGraph{Units: []models.Unit{{
		ID: 1, Name: "Foundations", Content: "Original notes.",
	}}}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "rename_unit", UnitID: 1, UnitName: "Mathematical foundations"},
		{Kind: "update_content", UnitID: 1, UnitContent: "Revised notes."},
		{Kind: "delete_unit", UnitID: 1},
	}}

	PopulateCurriculumProposalPreviousState(base, proposal)

	if proposal.Changes[0].PreviousUnitName != "Foundations" ||
		proposal.Changes[1].PreviousUnitContent != "Original notes." ||
		proposal.Changes[2].UnitName != "Mathematical foundations" ||
		proposal.Changes[2].UnitContent != "Revised notes." {
		t.Fatalf("derived proposal state = %#v", proposal.Changes)
	}
}

func TestValidateCurriculumProposalAcceptsCoherentUnorderedChanges(t *testing.T) {
	base := validationTestGraph()
	prerequisiteID := int64(1)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{
			ID: 13, Kind: "add_dependency", UnitID: 10,
			PrerequisiteID: &prerequisiteID,
		},
		{
			ID: 10, Kind: "create_unit", UnitID: 10,
			UnitName: "Geometry", UnitContent: "Learn shapes.",
		},
		{
			ID: 11, Kind: "rename_unit", UnitID: 1,
			PreviousUnitName: "Foundations", UnitName: "Mathematical foundations",
		},
		{
			ID: 12, Kind: "update_content", UnitID: 2,
			PreviousUnitContent: "Learn variables.", UnitContent: "Learn variables and equations.",
		},
	}}

	if err := validateCurriculumProposal(base, proposal); err != nil {
		t.Fatalf("validate coherent proposal: %v", err)
	}
}

func TestValidateCurriculumProposalOrdersPrerequisiteResolutionBeforeDeletion(t *testing.T) {
	base := validationTestGraph()
	prerequisiteID := int64(1)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{
			ID: 11, Kind: "delete_unit", UnitID: 1,
			UnitName: "Foundations", UnitContent: "Learn the basics.",
		},
		{ID: 10, Kind: "remove_dependency", UnitID: 2, PrerequisiteID: &prerequisiteID},
	}}

	if err := validateCurriculumProposal(base, proposal); err != nil {
		t.Fatalf("validate coherent deletion: %v", err)
	}
}

func TestValidateCurriculumProposalAcceptsRecognitionAcrossResultingState(t *testing.T) {
	base := validationTestGraph()
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{
			ID: 10, Kind: "create_unit", UnitID: 10,
			UnitName: "Modern geometry", UnitContent: "Learn modern geometry.",
		},
		{
			ID: 11, Kind: "recognition",
			Recognition: &models.Recognition{
				Sources: []models.Unit{{ID: 1}, {ID: 3}},
				Targets: []models.Unit{{ID: 10}},
			},
		},
	}}

	if err := validateCurriculumProposal(base, proposal); err != nil {
		t.Fatalf("validate recognition: %v", err)
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
			name: "duplicate change identity",
			changes: []models.CurriculumProposalChange{
				{ID: 10, Kind: "rename_unit", UnitID: 1, UnitName: "Core foundations"},
				{ID: 10, Kind: "update_content", UnitID: 2, UnitContent: "Expanded variables."},
			},
		},
		{
			name: "missing unit",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Kind: "rename_unit", UnitID: 99,
				PreviousUnitName: "Missing", UnitName: "Still missing",
			}},
		},
		{
			name: "no effect",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Kind: "update_content", UnitID: 1,
				PreviousUnitContent: "Learn the basics.", UnitContent: "Learn the basics.",
			}},
		},
		{
			name: "created then deleted",
			changes: []models.CurriculumProposalChange{
				{
					ID: 10, Kind: "create_unit", UnitID: 10,
					UnitName: "Temporary", UnitContent: "Temporary content.",
				},
				{
					ID: 11, Kind: "delete_unit", UnitID: 10,
					UnitName: "Temporary", UnitContent: "Temporary content.",
				},
			},
		},
		{
			name: "dependency cycle",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Kind: "add_dependency", UnitID: 1,
				PrerequisiteID: &prerequisiteTwo,
			}},
		},
		{
			name: "dependency changed twice",
			changes: []models.CurriculumProposalChange{
				{ID: 10, Kind: "remove_dependency", UnitID: 2, PrerequisiteID: &prerequisiteOne},
				{ID: 11, Kind: "add_dependency", UnitID: 2, PrerequisiteID: &prerequisiteOne},
			},
		},
		{
			name: "delete prerequisite still in use",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Kind: "delete_unit", UnitID: 1,
				UnitName: "Foundations", UnitContent: "Learn the basics.",
			}},
		},
		{
			name: "change superseded by deletion",
			changes: []models.CurriculumProposalChange{
				{
					ID: 10, Kind: "rename_unit", UnitID: 3,
					PreviousUnitName: "Geometry", UnitName: "Euclidean geometry",
				},
				{
					ID: 11, Kind: "delete_unit", UnitID: 3,
					UnitName: "Euclidean geometry", UnitContent: "Learn shapes.",
				},
			},
		},
		{
			name: "recognition source outside base",
			changes: []models.CurriculumProposalChange{
				{
					ID: 10, Kind: "create_unit", UnitID: 10,
					UnitName: "New", UnitContent: "New content.",
				},
				{
					ID: 11, Kind: "recognition",
					Recognition: &models.Recognition{
						Sources: []models.Unit{{ID: 10}},
						Targets: []models.Unit{{ID: 1}},
					},
				},
			},
		},
		{
			name: "recognition target outside result",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Kind: "recognition",
				Recognition: &models.Recognition{
					Sources: []models.Unit{{ID: 1}},
					Targets: []models.Unit{{ID: 99}},
				},
			}},
		},
		{
			name: "recognition duplicate source",
			changes: []models.CurriculumProposalChange{{
				ID: 10, Kind: "recognition",
				Recognition: &models.Recognition{
					Sources: []models.Unit{{ID: 1}, {ID: 1}},
					Targets: []models.Unit{{ID: 2}},
				},
			}},
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

func TestCurriculumRecognitionCoverageOnlyWarnsForUnmappedStructuralChanges(t *testing.T) {
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{ID: 10, Kind: "create_unit", UnitID: 10, UnitName: "Covered creation"},
		{ID: 11, Kind: "create_unit", UnitID: 11, UnitName: "Novel creation"},
		{ID: 12, Kind: "delete_unit", UnitID: 1, UnitName: "Covered retirement"},
		{ID: 13, Kind: "delete_unit", UnitID: 2, UnitName: "Unmapped retirement"},
		{
			ID: 14, Kind: "recognition",
			Recognition: &models.Recognition{
				Sources: []models.Unit{{ID: 1}},
				Targets: []models.Unit{{ID: 10}},
			},
		},
	}}

	warning := CurriculumRecognitionCoverage(proposal)

	if len(warning.CreatedWithoutSource) != 1 || warning.CreatedWithoutSource[0].ID != 11 {
		t.Fatalf("unmapped creations = %#v", warning.CreatedWithoutSource)
	}
	if len(warning.DeletedWithoutTarget) != 1 || warning.DeletedWithoutTarget[0].ID != 2 {
		t.Fatalf("unmapped deletions = %#v", warning.DeletedWithoutTarget)
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
