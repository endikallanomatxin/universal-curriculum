package server

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type apiProposalInput struct {
	Title     string `json:"title"`
	Rationale string `json:"rationale"`
}

type apiUnitInput struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type apiDependencyInput struct {
	UnitID         int64 `json:"unit_id"`
	PrerequisiteID int64 `json:"prerequisite_id"`
}

type apiRecognitionInput struct {
	SourceUnitIDs []int64 `json:"source_unit_ids"`
	TargetUnitIDs []int64 `json:"target_unit_ids"`
}

type apiRebaseResolutionInput struct {
	ChangeID int64  `json:"change_id"`
	Choice   string `json:"choice"`
	Content  string `json:"content"`
}

type apiRebasePlan struct {
	Status              string              `json:"status"`
	AcceptedProposalIDs []int64             `json:"accepted_proposal_ids"`
	Conflicts           []apiRebaseConflict `json:"conflicts"`
}

type apiRebaseConflict struct {
	ChangeID            int64   `json:"change_id"`
	AcceptedProposalIDs []int64 `json:"accepted_proposal_ids"`
}

func (server *Server) apiListProposals(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request, "status", "limit", "offset"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	status := request.URL.Query().Get("status")
	if status != "" && status != "draft" && status != "submitted" && status != "accepted" && status != "rejected" {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", "status must be draft, submitted, accepted or rejected", nil)
		return
	}
	limit, offset, err := apiPagination(request)
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	proposals, total, err := db.ListCurriculumProposalsForUser(
		server.Database, apiUser(request).ID, apiUser(request).IsAdmin, status, limit, offset,
	)
	if err != nil {
		log.Printf("API list proposals: %v", err)
		writeAPIInternalError(writer)
		return
	}
	resources := make([]apiProposal, 0, len(proposals))
	for _, proposal := range proposals {
		resources = append(resources, newAPIProposal(proposal, false))
	}
	writeAPIJSON(writer, http.StatusOK, struct {
		Proposals []apiProposal `json:"proposals"`
		Page      apiPage       `json:"page"`
	}{resources, apiPageFor(limit, offset, len(resources), total)})
}

func (server *Server) apiCreateProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	var input apiProposalInput
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	proposal, err := services.CreateCurriculumProposal(
		server.Database, apiUser(request).ID, input.Title, input.Rationale,
	)
	if err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	proposal, err = db.GetCurriculumProposal(server.Database, proposal.ID)
	if err != nil {
		log.Printf("API reload created proposal: %v", err)
		writeAPIInternalError(writer)
		return
	}
	writeAPIJSON(writer, http.StatusCreated, newAPIProposal(*proposal, true))
}

func (server *Server) apiGetProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposal, ok := server.apiVisibleProposal(writer, request)
	if !ok {
		return
	}
	writeAPIJSON(writer, http.StatusOK, newAPIProposal(*proposal, true))
}

func (server *Server) apiUpdateProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	var input apiProposalInput
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	if err := services.UpdateCurriculumProposal(
		server.Database, apiUser(request).ID, proposalID, input.Title, input.Rationale,
	); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeReloadedAPIProposal(writer, proposalID, http.StatusOK)
}

func (server *Server) apiDeleteProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	if err := services.DeleteCurriculumProposal(server.Database, apiUser(request).ID, proposalID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiCreateProposalUnit(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	var input apiUnitInput
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	unit, err := services.CreateCurriculumUnit(
		server.Database, apiUser(request).ID, proposalID, input.Name, input.Content,
	)
	if err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeAPIProposalUnit(writer, proposalID, unit.ID, http.StatusCreated)
}

func (server *Server) apiUpdateProposalUnit(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	unitID, ok := apiUnitID(writer, request)
	if !ok {
		return
	}
	var input apiUnitInput
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Content) == "" {
		fields := map[string]string{}
		if strings.TrimSpace(input.Name) == "" {
			fields["name"] = "is required"
		}
		if strings.TrimSpace(input.Content) == "" {
			fields["content"] = "is required"
		}
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "The request is invalid.", fields)
		return
	}
	if err := services.UpdateCurriculumUnitAndContent(
		server.Database, apiUser(request).ID, proposalID, unitID, input.Name, input.Content,
	); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeAPIProposalUnit(writer, proposalID, unitID, http.StatusOK)
}

func (server *Server) apiDeleteProposalUnit(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	unitID, ok := apiUnitID(writer, request)
	if !ok {
		return
	}
	if err := services.DeleteCurriculumUnit(server.Database, apiUser(request).ID, proposalID, unitID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiAddProposalDependency(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	var input apiDependencyInput
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	if input.UnitID <= 0 || input.PrerequisiteID <= 0 {
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "unit_id and prerequisite_id must be positive integers", nil)
		return
	}
	if err := services.AddUnitDependency(server.Database, apiUser(request).ID, proposalID, input.UnitID, input.PrerequisiteID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiRemoveProposalDependency(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	unitID, err := apiPathID(request, "unitId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "unitId must be a positive integer", nil)
		return
	}
	prerequisiteID, err := apiPathID(request, "prerequisiteId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "prerequisiteId must be a positive integer", nil)
		return
	}
	if err := services.RemoveUnitDependency(server.Database, apiUser(request).ID, proposalID, unitID, prerequisiteID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiAddProposalRecognition(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	var input apiRecognitionInput
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	if err := validateAPIIDs(input.SourceUnitIDs, "source_unit_ids"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", err.Error(), nil)
		return
	}
	if err := validateAPIIDs(input.TargetUnitIDs, "target_unit_ids"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", err.Error(), nil)
		return
	}
	if err := services.AddCurriculumRecognition(
		server.Database, apiUser(request).ID, proposalID, input.SourceUnitIDs, input.TargetUnitIDs,
	); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiDeleteProposalChange(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	changeID, err := apiPathID(request, "changeId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "changeId must be a positive integer.", nil)
		return
	}
	if err := services.DeleteCurriculumProposalChange(
		server.Database, apiUser(request).ID, proposalID, changeID,
	); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiGetProposalRebase(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposal, ok := server.apiEditableProposal(writer, request)
	if !ok {
		return
	}
	plan, err := services.PlanCurriculumProposalRebase(server.Database, proposal)
	if err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	writeAPIJSON(writer, http.StatusOK, newAPIRebasePlan(plan))
}

func (server *Server) apiResolveProposalRebase(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	var input struct {
		Resolutions []apiRebaseResolutionInput `json:"resolutions"`
	}
	if !decodeAPIInput(writer, request, &input) {
		return
	}
	resolutions := make(map[int64]services.CurriculumProposalRebaseResolution, len(input.Resolutions))
	for _, resolution := range input.Resolutions {
		if resolution.ChangeID <= 0 || resolutions[resolution.ChangeID].Choice != "" {
			writeAPIError(writer, http.StatusBadRequest, "validation_failed", "resolutions require unique positive change IDs", nil)
			return
		}
		if resolution.Choice != "keep" && resolution.Choice != "drop" && resolution.Choice != "edit" {
			writeAPIError(writer, http.StatusBadRequest, "validation_failed", "resolution choice must be keep, drop or edit", nil)
			return
		}
		resolutions[resolution.ChangeID] = services.CurriculumProposalRebaseResolution{
			Choice: resolution.Choice, Content: resolution.Content,
		}
	}
	if err := services.ResolveCurriculumProposalRebase(
		server.Database, apiUser(request).ID, proposalID, resolutions,
	); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeReloadedAPIProposal(writer, proposalID, http.StatusOK)
}

func (server *Server) apiSubmitProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	if err := services.SubmitCurriculumProposal(server.Database, apiUser(request).ID, proposalID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeReloadedAPIProposal(writer, proposalID, http.StatusOK)
}

func (server *Server) apiAcceptProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	if _, err := services.AcceptCurriculumProposal(server.Database, proposalID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeReloadedAPIProposal(writer, proposalID, http.StatusOK)
}

func (server *Server) apiRejectProposal(writer http.ResponseWriter, request *http.Request) {
	if !apiNoQuery(writer, request) {
		return
	}
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return
	}
	if err := services.RejectCurriculumProposal(server.Database, proposalID); err != nil {
		server.writeAPICurriculumError(writer, err)
		return
	}
	server.writeReloadedAPIProposal(writer, proposalID, http.StatusOK)
}

func apiNoQuery(writer http.ResponseWriter, request *http.Request) bool {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return false
	}
	return true
}

func decodeAPIInput(writer http.ResponseWriter, request *http.Request, target any) bool {
	if err := decodeAPIJSON(writer, request, target); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return false
	}
	return true
}

func apiProposalID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	proposalID, err := apiPathID(request, "proposalId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "proposalId must be a positive integer.", nil)
		return 0, false
	}
	return proposalID, true
}

func apiUnitID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	unitID, err := apiPathID(request, "unitId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "unitId must be a positive integer.", nil)
		return 0, false
	}
	return unitID, true
}

func (server *Server) apiVisibleProposal(writer http.ResponseWriter, request *http.Request) (*models.CurriculumProposal, bool) {
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return nil, false
	}
	proposal, err := services.GetVisibleCurriculumProposal(server.Database, apiUser(request).ID, apiUser(request).IsAdmin, proposalID)
	if err != nil {
		if errors.Is(err, services.ErrProposalNotFound) {
			writeAPIError(writer, http.StatusNotFound, "proposal_not_found", "The proposal was not found.", nil)
			return nil, false
		}
		log.Printf("API get proposal: %v", err)
		writeAPIInternalError(writer)
		return nil, false
	}
	return proposal, true
}

func (server *Server) apiEditableProposal(writer http.ResponseWriter, request *http.Request) (*models.CurriculumProposal, bool) {
	proposalID, ok := apiProposalID(writer, request)
	if !ok {
		return nil, false
	}
	proposal, err := services.GetEditableCurriculumProposal(server.Database, apiUser(request).ID, proposalID)
	if errors.Is(err, services.ErrProposalNotFound) {
		writeAPIError(writer, http.StatusNotFound, "proposal_not_found", "The editable proposal was not found.", nil)
		return nil, false
	}
	if err != nil {
		log.Printf("API get editable proposal: %v", err)
		writeAPIInternalError(writer)
		return nil, false
	}
	return proposal, true
}

func (server *Server) writeReloadedAPIProposal(writer http.ResponseWriter, proposalID int64, status int) {
	proposal, err := db.GetCurriculumProposal(server.Database, proposalID)
	if err != nil {
		log.Printf("API reload proposal: %v", err)
		writeAPIInternalError(writer)
		return
	}
	if proposal == nil {
		writeAPIError(writer, http.StatusNotFound, "proposal_not_found", "The proposal was not found.", nil)
		return
	}
	writeAPIJSON(writer, status, newAPIProposal(*proposal, true))
}

func (server *Server) writeAPIProposalUnit(writer http.ResponseWriter, proposalID, unitID int64, status int) {
	proposal, err := db.GetCurriculumProposal(server.Database, proposalID)
	if err != nil {
		log.Printf("API reload proposal unit: %v", err)
		writeAPIInternalError(writer)
		return
	}
	if proposal == nil {
		writeAPIError(writer, http.StatusNotFound, "proposal_not_found", "The proposal was not found.", nil)
		return
	}
	base, err := services.CurriculumGraphAtProposal(server.Database, proposal.BaseProposalID)
	if err != nil {
		log.Printf("API load proposal unit base: %v", err)
		writeAPIInternalError(writer)
		return
	}
	working := services.CurriculumGraphWithProposal(base, proposal)
	unit := working.Unit(unitID)
	if unit == nil {
		writeAPIError(writer, http.StatusNotFound, "unit_not_found", "The proposed unit was not found.", nil)
		return
	}
	resource := apiUnitResource(*unit, working.Dependencies)
	if resource.CreatedAt.IsZero() {
		resource.CreatedAt = proposal.CreatedAt
	}
	if resource.UpdatedAt.IsZero() {
		resource.UpdatedAt = proposal.CreatedAt
	}
	writeAPIJSON(writer, status, resource)
}

func newAPIRebasePlan(plan *services.CurriculumProposalRebasePlan) apiRebasePlan {
	resource := apiRebasePlan{
		Status: plan.Status, AcceptedProposalIDs: []int64{},
		Conflicts: make([]apiRebaseConflict, 0, len(plan.Conflicts)),
	}
	for _, proposal := range plan.AcceptedProposals {
		resource.AcceptedProposalIDs = append(resource.AcceptedProposalIDs, proposal.ID)
	}
	for _, conflict := range plan.Conflicts {
		item := apiRebaseConflict{ChangeID: conflict.Change.ID, AcceptedProposalIDs: []int64{}}
		seen := make(map[int64]bool)
		for _, work := range conflict.AcceptedWork {
			if !seen[work.Proposal.ID] {
				seen[work.Proposal.ID] = true
				item.AcceptedProposalIDs = append(item.AcceptedProposalIDs, work.Proposal.ID)
			}
		}
		resource.Conflicts = append(resource.Conflicts, item)
	}
	return resource
}

func (server *Server) writeAPICurriculumError(writer http.ResponseWriter, err error) {
	message, status := curriculumErrorResponse(err)
	domainCode := services.ClassifyDomainError(err)
	code := "validation_failed"
	switch status {
	case http.StatusNotFound:
		code = "not_found"
		if domainCode == services.DomainErrorProposalNotFound {
			code = "proposal_not_found"
		} else if domainCode == services.DomainErrorUnitNotFound {
			code = "unit_not_found"
		} else if domainCode == services.DomainErrorDependencyNotFound {
			code = "dependency_not_found"
		}
	case http.StatusConflict:
		code = "conflict"
	case http.StatusInternalServerError:
		log.Printf("API modify curriculum: %v", err)
		writeAPIInternalError(writer)
		return
	}
	writeAPIError(writer, status, code, message, nil)
}
