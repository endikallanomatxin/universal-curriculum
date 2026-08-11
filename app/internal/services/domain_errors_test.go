package services

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassifyDomainError(t *testing.T) {
	tests := []struct {
		err  error
		want DomainErrorCode
	}{
		{ErrUnitNotFound, DomainErrorUnitNotFound},
		{ErrCurriculumUnitNotFound, DomainErrorUnitNotFound},
		{ErrUnitNameRequired, DomainErrorUnitNameRequired},
		{ErrUnitNameTooLong, DomainErrorUnitNameTooLong},
		{ErrUnitContentRequired, DomainErrorUnitContentRequired},
		{&UnitIsPrerequisiteError{DependentNames: []string{"Algebra"}}, DomainErrorUnitIsPrerequisite},
		{ErrDependencyExists, DomainErrorDependencyExists},
		{ErrDependencyNotFound, DomainErrorDependencyNotFound},
		{ErrDependencyCycle, DomainErrorDependencyCycle},
		{ErrProposalNotFound, DomainErrorProposalNotFound},
		{ErrProposalTitleRequired, DomainErrorProposalTitleRequired},
		{ErrProposalTitleTooLong, DomainErrorProposalTitleTooLong},
		{ErrProposalRationaleRequired, DomainErrorProposalRationaleRequired},
		{ErrProposalRationaleTooLong, DomainErrorProposalRationaleTooLong},
		{ErrProposalEmpty, DomainErrorProposalEmpty},
		{ErrProposalOutdated, DomainErrorProposalOutdated},
		{ErrProposalRebaseRequired, DomainErrorProposalRebaseRequired},
		{ErrRebaseResolutionRequired, DomainErrorRebaseResolutionRequired},
		{ErrRecognitionSourcesRequired, DomainErrorRecognitionSourcesRequired},
		{ErrRecognitionTargetsRequired, DomainErrorRecognitionTargetsRequired},
		{ErrProposalInvalid, DomainErrorProposalInvalid},
		{ErrLearningPathNotFound, DomainErrorLearningPathNotFound},
		{ErrLearningPathNameRequired, DomainErrorLearningPathNameRequired},
		{ErrLearningPathNameTooLong, DomainErrorLearningPathNameTooLong},
		{ErrLearningPathUnitsRequired, DomainErrorLearningPathUnitsRequired},
		{errors.New("unexpected"), DomainErrorUnknown},
	}
	for _, test := range tests {
		t.Run(string(test.want), func(t *testing.T) {
			if got := ClassifyDomainError(fmt.Errorf("operation failed: %w", test.err)); got != test.want {
				t.Fatalf("ClassifyDomainError() = %q, want %q", got, test.want)
			}
		})
	}
	if got := ClassifyDomainError(nil); got != DomainErrorUnknown {
		t.Fatalf("ClassifyDomainError(nil) = %q, want %q", got, DomainErrorUnknown)
	}
}
