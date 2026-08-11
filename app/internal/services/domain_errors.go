package services

import "errors"

// DomainErrorCode identifies expected business failures without coupling
// adapters to the concrete sentinel or typed error that produced them.
type DomainErrorCode string

const (
	DomainErrorUnknown                    DomainErrorCode = "unknown"
	DomainErrorUnitNotFound               DomainErrorCode = "unit_not_found"
	DomainErrorUnitNameRequired           DomainErrorCode = "unit_name_required"
	DomainErrorUnitNameTooLong            DomainErrorCode = "unit_name_too_long"
	DomainErrorUnitContentRequired        DomainErrorCode = "unit_content_required"
	DomainErrorUnitIsPrerequisite         DomainErrorCode = "unit_is_prerequisite"
	DomainErrorDependencyExists           DomainErrorCode = "dependency_exists"
	DomainErrorDependencyNotFound         DomainErrorCode = "dependency_not_found"
	DomainErrorDependencyCycle            DomainErrorCode = "dependency_cycle"
	DomainErrorProposalNotFound           DomainErrorCode = "proposal_not_found"
	DomainErrorProposalTitleRequired      DomainErrorCode = "proposal_title_required"
	DomainErrorProposalTitleTooLong       DomainErrorCode = "proposal_title_too_long"
	DomainErrorProposalRationaleRequired  DomainErrorCode = "proposal_rationale_required"
	DomainErrorProposalRationaleTooLong   DomainErrorCode = "proposal_rationale_too_long"
	DomainErrorProposalEmpty              DomainErrorCode = "proposal_empty"
	DomainErrorProposalOutdated           DomainErrorCode = "proposal_outdated"
	DomainErrorProposalRebaseRequired     DomainErrorCode = "proposal_rebase_required"
	DomainErrorRebaseResolutionRequired   DomainErrorCode = "rebase_resolution_required"
	DomainErrorRecognitionSourcesRequired DomainErrorCode = "recognition_sources_required"
	DomainErrorRecognitionTargetsRequired DomainErrorCode = "recognition_targets_required"
	DomainErrorProposalInvalid            DomainErrorCode = "proposal_invalid"
	DomainErrorLearningPathNotFound       DomainErrorCode = "learning_path_not_found"
	DomainErrorLearningPathNameRequired   DomainErrorCode = "learning_path_name_required"
	DomainErrorLearningPathNameTooLong    DomainErrorCode = "learning_path_name_too_long"
	DomainErrorLearningPathUnitsRequired  DomainErrorCode = "learning_path_units_required"
)

func ClassifyDomainError(err error) DomainErrorCode {
	var prerequisite *UnitIsPrerequisiteError
	switch {
	case errors.As(err, &prerequisite):
		return DomainErrorUnitIsPrerequisite
	case errors.Is(err, ErrUnitNotFound), errors.Is(err, ErrCurriculumUnitNotFound):
		return DomainErrorUnitNotFound
	case errors.Is(err, ErrUnitNameRequired):
		return DomainErrorUnitNameRequired
	case errors.Is(err, ErrUnitNameTooLong):
		return DomainErrorUnitNameTooLong
	case errors.Is(err, ErrUnitContentRequired):
		return DomainErrorUnitContentRequired
	case errors.Is(err, ErrDependencyExists):
		return DomainErrorDependencyExists
	case errors.Is(err, ErrDependencyNotFound):
		return DomainErrorDependencyNotFound
	case errors.Is(err, ErrDependencyCycle):
		return DomainErrorDependencyCycle
	case errors.Is(err, ErrProposalNotFound):
		return DomainErrorProposalNotFound
	case errors.Is(err, ErrProposalTitleRequired):
		return DomainErrorProposalTitleRequired
	case errors.Is(err, ErrProposalTitleTooLong):
		return DomainErrorProposalTitleTooLong
	case errors.Is(err, ErrProposalRationaleRequired):
		return DomainErrorProposalRationaleRequired
	case errors.Is(err, ErrProposalRationaleTooLong):
		return DomainErrorProposalRationaleTooLong
	case errors.Is(err, ErrProposalEmpty):
		return DomainErrorProposalEmpty
	case errors.Is(err, ErrProposalOutdated):
		return DomainErrorProposalOutdated
	case errors.Is(err, ErrProposalRebaseRequired):
		return DomainErrorProposalRebaseRequired
	case errors.Is(err, ErrRebaseResolutionRequired):
		return DomainErrorRebaseResolutionRequired
	case errors.Is(err, ErrRecognitionSourcesRequired):
		return DomainErrorRecognitionSourcesRequired
	case errors.Is(err, ErrRecognitionTargetsRequired):
		return DomainErrorRecognitionTargetsRequired
	case errors.Is(err, ErrProposalInvalid):
		return DomainErrorProposalInvalid
	case errors.Is(err, ErrLearningPathNotFound):
		return DomainErrorLearningPathNotFound
	case errors.Is(err, ErrLearningPathNameRequired):
		return DomainErrorLearningPathNameRequired
	case errors.Is(err, ErrLearningPathNameTooLong):
		return DomainErrorLearningPathNameTooLong
	case errors.Is(err, ErrLearningPathUnitsRequired):
		return DomainErrorLearningPathUnitsRequired
	default:
		return DomainErrorUnknown
	}
}
