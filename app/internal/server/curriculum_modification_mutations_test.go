package server

import (
	"errors"
	"net/http"
	"testing"

	"universal-curriculum/internal/services"
)

func TestCurriculumErrorResponse(t *testing.T) {
	message, status := curriculumErrorResponse(&services.UnitIsPrerequisiteError{DependentNames: []string{"Algebra", "Calculus"}})
	if status != http.StatusConflict || message != "Remove the dependencies from Algebra and Calculus before deleting this unit." {
		t.Fatalf("curriculumErrorResponse() = %q, %d", message, status)
	}
	validationError := &services.ProposalValidationError{ChangeID: 12, Reason: "the dependency creates a cycle"}
	if message, status = curriculumErrorResponse(validationError); status != http.StatusConflict || message != validationError.Error() {
		t.Fatalf("proposal validation response = %q, %d", message, status)
	}
	if message, status = curriculumErrorResponse(errors.New("database unavailable")); status != http.StatusInternalServerError || message == "" {
		t.Fatalf("unexpected internal error response: %q, %d", message, status)
	}
}
