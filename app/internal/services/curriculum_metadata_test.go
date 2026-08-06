package services

import (
	"errors"
	"strings"
	"testing"
)

func TestCurriculumMetadataLengthValidationPrecedesPersistence(t *testing.T) {
	if _, err := CreateCurriculumUnit(
		nil, 1, 1, strings.Repeat("a", MaximumUnitNameLength+1), "Content",
	); !errors.Is(err, ErrUnitNameTooLong) {
		t.Fatalf("long unit name error = %v", err)
	}
	if _, err := CreateCurriculumProposal(
		nil, 1, strings.Repeat("a", MaximumProposalTitleLength+1), "Rationale",
	); !errors.Is(err, ErrProposalTitleTooLong) {
		t.Fatalf("long proposal title error = %v", err)
	}
	if _, err := CreateCurriculumProposal(
		nil, 1, "Title", strings.Repeat("a", MaximumProposalRationaleLength+1),
	); !errors.Is(err, ErrProposalRationaleTooLong) {
		t.Fatalf("long proposal rationale error = %v", err)
	}
}
