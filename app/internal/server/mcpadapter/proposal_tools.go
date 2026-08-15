package mcpadapter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/services"
)

type listProposalsInput struct {
	Status string `json:"status,omitempty" jsonschema:"Optional status filter: draft, submitted, accepted or rejected."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum results from 1 to 100; defaults to 25."`
	Offset int    `json:"offset,omitempty" jsonschema:"Zero-based result offset."`
}

type proposalsOutput struct {
	Proposals []proposal `json:"proposals"`
	Total     int        `json:"total"`
	Offset    int        `json:"offset"`
	HasMore   bool       `json:"has_more"`
}

type proposalIDInput struct {
	ProposalID int64 `json:"proposal_id" jsonschema:"Stable curriculum proposal ID."`
}

type createProposalInput struct {
	Title     string `json:"title" jsonschema:"Concise proposal title, at most 200 characters."`
	Rationale string `json:"rationale" jsonschema:"Why this curriculum change is useful, at most 1000 characters."`
}

type updateProposalInput struct {
	ProposalID int64  `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	Title      string `json:"title" jsonschema:"Complete replacement title, at most 200 characters."`
	Rationale  string `json:"rationale" jsonschema:"Complete replacement rationale, at most 1000 characters."`
}

type createProposalUnitInput struct {
	ProposalID int64  `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	Name       string `json:"name" jsonschema:"Published-facing unit name, at most 200 characters."`
	Content    string `json:"content" jsonschema:"Final learner-facing microlesson for this concept, complete given its genuine prerequisites rather than an outline or teaching plan. Supports Markdown and LaTeX using $...$ inline or $$...$$ for display math."`
}

type updateProposalUnitInput struct {
	ProposalID int64  `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	UnitID     int64  `json:"unit_id" jsonschema:"Existing or proposal-created unit ID."`
	Name       string `json:"name" jsonschema:"Complete replacement unit name, at most 200 characters."`
	Content    string `json:"content" jsonschema:"Complete replacement with final learner-facing content that teaches the concept given its genuine prerequisites, not an outline or teaching plan. Supports Markdown and LaTeX using $...$ inline or $$...$$ for display math."`
}

type proposalUnitIDInput struct {
	ProposalID int64 `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	UnitID     int64 `json:"unit_id" jsonschema:"Existing or proposal-created unit ID."`
}

type setDependencyInput struct {
	ProposalID     int64 `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	UnitID         int64 `json:"unit_id" jsonschema:"Dependent unit ID."`
	PrerequisiteID int64 `json:"prerequisite_id" jsonschema:"Unit that must be completed first."`
	Present        bool  `json:"present" jsonschema:"True ensures the dependency exists; false ensures it does not."`
}

type dependencyState struct {
	UnitID         int64 `json:"unit_id"`
	PrerequisiteID int64 `json:"prerequisite_id"`
	Present        bool  `json:"present"`
}

type addRecognitionInput struct {
	ProposalID    int64   `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	SourceUnitIDs []int64 `json:"source_unit_ids" jsonschema:"Published units whose prior completion supplies the evidence."`
	TargetUnitIDs []int64 `json:"target_unit_ids" jsonschema:"Units granted completion when all sources were completed before publication."`
}

type changeIDInput struct {
	ProposalID int64 `json:"proposal_id" jsonschema:"Editable draft proposal ID."`
	ChangeID   int64 `json:"change_id" jsonschema:"Proposal change ID returned by get_proposal."`
}

type rebaseResolution struct {
	ChangeID int64  `json:"change_id" jsonschema:"Conflicting proposal change ID."`
	Choice   string `json:"choice" jsonschema:"keep, drop, or edit. edit is valid only for content changes."`
	Content  string `json:"content,omitempty" jsonschema:"Required replacement content when choice is edit."`
}

type resolveRebaseInput struct {
	ProposalID  int64              `json:"proposal_id" jsonschema:"Editable stale draft proposal ID."`
	Resolutions []rebaseResolution `json:"resolutions" jsonschema:"One resolution for every conflict returned by get_proposal_rebase."`
}

type submitProposalInput struct {
	ProposalID    int64  `json:"proposal_id" jsonschema:"Draft proposal ID inspected immediately before publication."`
	ExpectedTitle string `json:"expected_title" jsonschema:"Current proposal title, used to guard against publishing the wrong draft."`
	Confirmed     bool   `json:"confirmed" jsonschema:"Must be true only after the user explicitly requests submission."`
}

func (application *adapter) addProposalTools(server *mcp.Server) {
	addTool(server, "list_proposals", "List curriculum proposals", "Lists proposals visible to the contributor; another user's drafts and rejected proposals remain private unless the contributor is an administrator.", readOnly("List curriculum proposals"), application.listProposals)
	addTool(server, "get_proposal", "Get curriculum proposal", "Returns proposal metadata and ordered changes. A draft is visible only to its author.", readOnly("Get curriculum proposal"), application.getProposal)
	addTool(server, "create_proposal", "Create curriculum proposal", "Creates an empty draft through which curriculum changes can be prepared. Not safe to retry after an ambiguous transport failure.", mutation("Create curriculum proposal", false, false), application.createProposal)
	addTool(server, "update_proposal", "Update proposal metadata", "Replaces the title and rationale of an authored draft.", mutation("Update proposal metadata", true, false), application.updateProposal)
	addTool(server, "delete_proposal", "Delete curriculum proposal", "Deletes an authored draft and all of its unaccepted changes.", mutation("Delete curriculum proposal", true, true), application.deleteProposal)
	addTool(server, "create_proposal_unit", "Create unit in proposal", "Proposes one new unit. First search for existing and overlapping knowledge, then apply the curriculum-units, dependencies, and writing-content documentation.", mutation("Create unit in proposal", false, false), application.createProposalUnit)
	addTool(server, "update_proposal_unit", "Update unit in proposal", "Converges the name and learner-facing content on the proposed final state after applying the canonical curriculum writing guidance. For a unit created in this proposal, it updates the existing creation rather than adding edit history.", mutation("Update unit in proposal", true, false), application.updateProposalUnit)
	addTool(server, "delete_proposal_unit", "Delete unit in proposal", "Proposes deletion of a unit, or discards a unit created in the same draft.", mutation("Delete unit in proposal", true, true), application.deleteProposalUnit)
	addTool(server, "set_proposal_dependency", "Set proposal dependency", "Idempotently ensures a prerequisite relationship is present or absent; cycles are rejected.", mutation("Set proposal dependency", true, true), application.setProposalDependency)
	addTool(server, "add_proposal_recognition", "Add proposal recognition", "Ensures an identical recognition exists. Recognitions preserve progress across curriculum changes and do not constitute ordinary public curriculum content.", mutation("Add proposal recognition", true, false), application.addProposalRecognition)
	addTool(server, "delete_proposal_change", "Delete proposal change", "Deletes one draft change by its stable change ID.", mutation("Delete proposal change", true, true), application.deleteProposalChange)
	addTool(server, "get_proposal_rebase", "Inspect proposal rebase", "Reports whether an authored draft is current, automatically rebasable, or needs review and identifies conflicts.", readOnly("Inspect proposal rebase"), application.getProposalRebase)
	addTool(server, "resolve_proposal_rebase", "Resolve proposal rebase", "Advances an automatic rebase with no resolutions, or applies one keep, drop or edit decision for every reported conflict. Inspect the plan first.", mutation("Resolve proposal rebase", true, true), application.resolveProposalRebase)
	addTool(server, "submit_proposal", "Submit curriculum proposal", "Submits an authored draft for administrator review. Call only after an explicit user request; confirmed and the current title are required.", mutation("Submit curriculum proposal", true, true), application.submitProposal)
}

func (application *adapter) addProposalDecisionTools(server *mcp.Server) {
	addTool(server, "accept_proposal", "Accept curriculum proposal", "Accepts a submitted proposal into the shared curriculum.", mutation("Accept curriculum proposal", true, true), application.acceptProposal)
	addTool(server, "reject_proposal", "Reject curriculum proposal", "Rejects and preserves a submitted proposal.", mutation("Reject curriculum proposal", true, true), application.rejectProposal)
}

func (application *adapter) listProposals(_ context.Context, request *mcp.CallToolRequest, input listProposalsInput) (*mcp.CallToolResult, toolOutput[proposalsOutput], error) {
	if result, output, err, user := requireContributor[proposalsOutput](request); user == nil {
		return result, output, err
	} else {
		return application.listProposalsForUser(user.ID, user.IsAdmin, input)
	}
}

func (application *adapter) listProposalsForUser(userID int64, isAdmin bool, input listProposalsInput) (*mcp.CallToolResult, toolOutput[proposalsOutput], error) {
	if input.Limit == 0 {
		input.Limit = 25
	}
	models, total, err := db.ListCurriculumProposalsForUser(application.database, userID, isAdmin, input.Status, input.Limit, input.Offset)
	if err != nil {
		return internalFailure[proposalsOutput]("list proposals", err)
	}
	result := proposalsOutput{Proposals: make([]proposal, 0, len(models)), Total: total, Offset: input.Offset, HasMore: input.Offset+len(models) < total}
	for _, model := range models {
		result.Proposals = append(result.Proposals, newProposal(model, false))
	}
	return ok(result)
}

func (application *adapter) getProposal(_ context.Context, request *mcp.CallToolRequest, input proposalIDInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	if result, output, err, user := requireContributor[proposal](request); user == nil {
		return result, output, err
	} else {
		model, err := services.GetVisibleCurriculumProposal(application.database, user.ID, user.IsAdmin, input.ProposalID)
		if err != nil {
			return curriculumFailure[proposal]("get proposal", err)
		}
		return ok(newProposal(*model, true))
	}
}

func (application *adapter) createProposal(_ context.Context, request *mcp.CallToolRequest, input createProposalInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	if result, output, err, user := requireContributor[proposal](request); user == nil {
		return result, output, err
	} else {
		created, err := services.CreateCurriculumProposal(application.database, user.ID, input.Title, input.Rationale)
		if err != nil {
			return curriculumFailure[proposal]("create proposal", err)
		}
		return application.reloadProposal(created.ID)
	}
}

func (application *adapter) updateProposal(_ context.Context, request *mcp.CallToolRequest, input updateProposalInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	if result, output, err, user := requireContributor[proposal](request); user == nil {
		return result, output, err
	} else {
		err := services.UpdateCurriculumProposal(application.database, user.ID, input.ProposalID, input.Title, input.Rationale)
		if err != nil {
			return curriculumFailure[proposal]("update proposal", err)
		}
		return application.reloadProposal(input.ProposalID)
	}
}

func (application *adapter) deleteProposal(_ context.Context, request *mcp.CallToolRequest, input proposalIDInput) (*mcp.CallToolResult, toolOutput[deleteOutput], error) {
	result, output, authErr, user := requireContributor[deleteOutput](request)
	if user == nil {
		return result, output, authErr
	}
	err := services.DeleteCurriculumProposal(application.database, user.ID, input.ProposalID)
	if err != nil {
		return curriculumFailure[deleteOutput]("delete proposal", err)
	}
	return ok(deleteOutput{Deleted: true})
}

func (application *adapter) createProposalUnit(_ context.Context, request *mcp.CallToolRequest, input createProposalUnitInput) (*mcp.CallToolResult, toolOutput[unit], error) {
	result, output, authErr, user := requireContributor[unit](request)
	if user == nil {
		return result, output, authErr
	}
	created, err := services.CreateCurriculumUnit(application.database, user.ID, input.ProposalID, input.Name, input.Content)
	if err != nil {
		return curriculumFailure[unit]("create proposal unit", err)
	}
	return application.proposalUnit(input.ProposalID, created.ID)
}

func (application *adapter) updateProposalUnit(_ context.Context, request *mcp.CallToolRequest, input updateProposalUnitInput) (*mcp.CallToolResult, toolOutput[unit], error) {
	result, output, authErr, user := requireContributor[unit](request)
	if user == nil {
		return result, output, authErr
	}
	err := services.UpdateCurriculumUnitAndContent(application.database, user.ID, input.ProposalID, input.UnitID, input.Name, input.Content)
	if err != nil {
		return curriculumFailure[unit]("update proposal unit", err)
	}
	return application.proposalUnit(input.ProposalID, input.UnitID)
}

func (application *adapter) deleteProposalUnit(_ context.Context, request *mcp.CallToolRequest, input proposalUnitIDInput) (*mcp.CallToolResult, toolOutput[deleteOutput], error) {
	result, output, authErr, user := requireContributor[deleteOutput](request)
	if user == nil {
		return result, output, authErr
	}
	err := services.DeleteCurriculumUnit(application.database, user.ID, input.ProposalID, input.UnitID)
	if err != nil {
		return curriculumFailure[deleteOutput]("delete proposal unit", err)
	}
	return ok(deleteOutput{Deleted: true})
}

func (application *adapter) setProposalDependency(_ context.Context, request *mcp.CallToolRequest, input setDependencyInput) (*mcp.CallToolResult, toolOutput[dependencyState], error) {
	result, output, authErr, user := requireContributor[dependencyState](request)
	if user == nil {
		return result, output, authErr
	}
	err := services.SetUnitDependency(application.database, user.ID, input.ProposalID, input.UnitID, input.PrerequisiteID, input.Present)
	if err != nil {
		return curriculumFailure[dependencyState]("set proposal dependency", err)
	}
	return ok(dependencyState{UnitID: input.UnitID, PrerequisiteID: input.PrerequisiteID, Present: input.Present})
}

func (application *adapter) addProposalRecognition(_ context.Context, request *mcp.CallToolRequest, input addRecognitionInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	result, output, authErr, user := requireContributor[proposal](request)
	if user == nil {
		return result, output, authErr
	}
	err := services.EnsureCurriculumRecognition(application.database, user.ID, input.ProposalID, input.SourceUnitIDs, input.TargetUnitIDs)
	if err != nil {
		return curriculumFailure[proposal]("add proposal recognition", err)
	}
	return application.reloadProposal(input.ProposalID)
}

func (application *adapter) deleteProposalChange(_ context.Context, request *mcp.CallToolRequest, input changeIDInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	result, output, authErr, user := requireContributor[proposal](request)
	if user == nil {
		return result, output, authErr
	}
	err := services.DeleteCurriculumProposalChange(application.database, user.ID, input.ProposalID, input.ChangeID)
	if err != nil {
		return curriculumFailure[proposal]("delete proposal change", err)
	}
	return application.reloadProposal(input.ProposalID)
}

func (application *adapter) getProposalRebase(_ context.Context, request *mcp.CallToolRequest, input proposalIDInput) (*mcp.CallToolResult, toolOutput[rebasePlan], error) {
	result, output, authErr, user := requireContributor[rebasePlan](request)
	if user == nil {
		return result, output, authErr
	}
	model, err := services.GetEditableCurriculumProposal(application.database, user.ID, input.ProposalID)
	if err != nil {
		return curriculumFailure[rebasePlan]("get proposal rebase", err)
	}
	plan, err := services.PlanCurriculumProposalRebase(application.database, model)
	if err != nil {
		return curriculumFailure[rebasePlan]("get proposal rebase", err)
	}
	return ok(newRebasePlan(plan))
}

func (application *adapter) resolveProposalRebase(_ context.Context, request *mcp.CallToolRequest, input resolveRebaseInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	result, output, authErr, user := requireContributor[proposal](request)
	if user == nil {
		return result, output, authErr
	}
	model, err := services.GetEditableCurriculumProposal(application.database, user.ID, input.ProposalID)
	if err != nil {
		return curriculumFailure[proposal]("inspect proposal rebase", err)
	}
	plan, err := services.PlanCurriculumProposalRebase(application.database, model)
	if err != nil {
		return curriculumFailure[proposal]("inspect proposal rebase", err)
	}
	if plan.NeedsReview() && len(input.Resolutions) == 0 {
		return failed[proposal]("validation_failed", "Every reported rebase conflict requires a resolution.", map[string]string{"resolutions": "must not be empty when review is required"})
	}
	resolutions := make(map[int64]services.CurriculumProposalRebaseResolution, len(input.Resolutions))
	for _, resolution := range input.Resolutions {
		if resolution.ChangeID <= 0 || resolutions[resolution.ChangeID].Choice != "" || !slices.Contains([]string{"keep", "drop", "edit"}, resolution.Choice) {
			return failed[proposal]("validation_failed", "Resolutions require unique positive change IDs and a keep, drop or edit choice.", nil)
		}
		resolutions[resolution.ChangeID] = services.CurriculumProposalRebaseResolution{Choice: resolution.Choice, Content: resolution.Content}
	}
	err = services.ResolveCurriculumProposalRebase(application.database, user.ID, input.ProposalID, resolutions)
	if err != nil {
		return curriculumFailure[proposal]("resolve proposal rebase", err)
	}
	return application.reloadProposal(input.ProposalID)
}

func (application *adapter) submitProposal(_ context.Context, request *mcp.CallToolRequest, input submitProposalInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	result, output, authErr, user := requireContributor[proposal](request)
	if user == nil {
		return result, output, authErr
	}
	if !input.Confirmed {
		return failed[proposal]("confirmation_required", "Submission requires confirmed=true after an explicit user request.", map[string]string{"confirmed": "must be true"})
	}
	model, err := services.GetEditableCurriculumProposal(application.database, user.ID, input.ProposalID)
	if err != nil {
		return curriculumFailure[proposal]("inspect proposal before submission", err)
	}
	if strings.TrimSpace(input.ExpectedTitle) != model.Title {
		return failed[proposal]("confirmation_mismatch", "expected_title does not match the current proposal title. Inspect the proposal again.", map[string]string{"expected_title": "does not match"})
	}
	if err := services.SubmitCurriculumProposal(application.database, user.ID, input.ProposalID); err != nil {
		return curriculumFailure[proposal]("submit proposal", err)
	}
	return application.reloadProposal(input.ProposalID)
}

func (application *adapter) acceptProposal(_ context.Context, request *mcp.CallToolRequest, input proposalIDInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	result, output, authErr, user := requireAdmin[proposal](request)
	if user == nil {
		return result, output, authErr
	}
	if _, err := services.AcceptCurriculumProposal(application.database, input.ProposalID); err != nil {
		return curriculumFailure[proposal]("accept proposal", err)
	}
	return application.reloadProposal(input.ProposalID)
}

func (application *adapter) rejectProposal(_ context.Context, request *mcp.CallToolRequest, input proposalIDInput) (*mcp.CallToolResult, toolOutput[proposal], error) {
	result, output, authErr, user := requireAdmin[proposal](request)
	if user == nil {
		return result, output, authErr
	}
	if err := services.RejectCurriculumProposal(application.database, input.ProposalID); err != nil {
		return curriculumFailure[proposal]("reject proposal", err)
	}
	return application.reloadProposal(input.ProposalID)
}

func (application *adapter) reloadProposal(id int64) (*mcp.CallToolResult, toolOutput[proposal], error) {
	model, err := db.GetCurriculumProposal(application.database, id)
	if err != nil {
		return internalFailure[proposal]("reload proposal", err)
	}
	if model == nil {
		return failed[proposal]("proposal_not_found", "The proposal was not found.", nil)
	}
	return ok(newProposal(*model, true))
}

func (application *adapter) proposalUnit(proposalID, unitID int64) (*mcp.CallToolResult, toolOutput[unit], error) {
	model, err := db.GetCurriculumProposal(application.database, proposalID)
	if err != nil {
		return internalFailure[unit]("reload proposal unit", err)
	}
	if model == nil {
		return failed[unit]("proposal_not_found", "The proposal was not found.", nil)
	}
	base, err := services.CurriculumGraphAtProposal(application.database, model.BaseProposalID)
	if err != nil {
		return internalFailure[unit]("load proposal base", err)
	}
	working := curriculumRepresentation(services.CurriculumGraphWithProposal(base, model), nil)
	for _, item := range working.Units {
		if item.ID == unitID {
			return ok(item)
		}
	}
	return failed[unit]("unit_not_found", "The proposed unit was not found.", nil)
}

func newRebasePlan(model *services.CurriculumProposalRebasePlan) rebasePlan {
	result := rebasePlan{Status: model.Status, AcceptedProposalIDs: []int64{}, Conflicts: make([]rebaseConflict, 0, len(model.Conflicts))}
	for _, accepted := range model.AcceptedProposals {
		result.AcceptedProposalIDs = append(result.AcceptedProposalIDs, accepted.ID)
	}
	for _, conflict := range model.Conflicts {
		item := rebaseConflict{ChangeID: conflict.Change.ID, Kind: conflict.Change.Kind, AcceptedProposalIDs: []int64{}}
		if conflict.Change.UnitID > 0 {
			id := conflict.Change.UnitID
			item.UnitID = &id
		}
		for _, work := range conflict.AcceptedWork {
			if !slices.Contains(item.AcceptedProposalIDs, work.Proposal.ID) {
				item.AcceptedProposalIDs = append(item.AcceptedProposalIDs, work.Proposal.ID)
			}
		}
		result.Conflicts = append(result.Conflicts, item)
	}
	return result
}

func curriculumFailure[T any](operation string, err error) (*mcp.CallToolResult, toolOutput[T], error) {
	var prerequisite *services.UnitIsPrerequisiteError
	switch services.ClassifyDomainError(err) {
	case services.DomainErrorProposalNotFound:
		return failed[T]("proposal_not_found", "The editable proposal was not found.", nil)
	case services.DomainErrorUnitNotFound:
		return failed[T]("unit_not_found", "A curriculum unit was not found.", nil)
	case services.DomainErrorProposalTitleRequired:
		return failed[T]("validation_failed", "The proposal title is required.", map[string]string{"title": "is required"})
	case services.DomainErrorProposalTitleTooLong:
		return failed[T]("validation_failed", "The proposal title is too long.", map[string]string{"title": "must not exceed 200 characters"})
	case services.DomainErrorProposalRationaleRequired:
		return failed[T]("validation_failed", "The proposal rationale is required.", map[string]string{"rationale": "is required"})
	case services.DomainErrorProposalRationaleTooLong:
		return failed[T]("validation_failed", "The proposal rationale is too long.", map[string]string{"rationale": "must not exceed 1000 characters"})
	case services.DomainErrorUnitNameRequired:
		return failed[T]("validation_failed", "The unit name is required.", map[string]string{"name": "is required"})
	case services.DomainErrorUnitNameTooLong:
		return failed[T]("validation_failed", "The unit name is too long.", map[string]string{"name": "must not exceed 200 characters"})
	case services.DomainErrorUnitContentRequired:
		return failed[T]("validation_failed", "Unit content is required.", map[string]string{"content": "is required"})
	case services.DomainErrorRecognitionSourcesRequired:
		return failed[T]("validation_failed", "Recognition sources are required.", map[string]string{"source_unit_ids": "must not be empty"})
	case services.DomainErrorRecognitionTargetsRequired:
		return failed[T]("validation_failed", "Recognition targets are required.", map[string]string{"target_unit_ids": "must not be empty"})
	case services.DomainErrorDependencyCycle:
		return failed[T]("conflict", "The dependency would create a cycle.", nil)
	case services.DomainErrorProposalEmpty:
		return failed[T]("conflict", "The proposal has no changes to publish.", nil)
	case services.DomainErrorProposalOutdated, services.DomainErrorProposalRebaseRequired:
		return failed[T]("rebase_required", "The proposal must be inspected and rebased before this operation.", nil)
	case services.DomainErrorRebaseResolutionRequired:
		return failed[T]("conflict", "Every rebase conflict requires a valid resolution.", nil)
	case services.DomainErrorUnitIsPrerequisite:
		errors.As(err, &prerequisite)
		return failed[T]("conflict", fmt.Sprintf("The unit is still required by: %s.", strings.Join(prerequisite.DependentNames, ", ")), nil)
	default:
		return internalFailure[T](operation, err)
	}
}

func logRebaseFailures(proposalID int64, err error) {
	log.Printf("MCP rebase drafts after publishing proposal %d: %v", proposalID, err)
}
