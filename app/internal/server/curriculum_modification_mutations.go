package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"universal-curriculum/internal/services"
)

func (server *Server) createCurriculumUnit(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	authorID, _ := services.SessionUserID(request)
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	_, err = services.CreateCurriculumUnit(
		server.Database,
		authorID,
		proposalID,
		request.FormValue("name"),
		request.FormValue("content"),
	)
	if err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) updateCurriculumUnit(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	unitID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid unit ID", http.StatusBadRequest)
		return
	}
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.UpdateCurriculumUnit(server.Database, authorID, proposalID, unitID, request.FormValue("name")); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposalUnit(writer, request, proposalID, unitID)
}

func (server *Server) updateCurriculumUnitContent(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	unitID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid unit ID", http.StatusBadRequest)
		return
	}
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.UpdateCurriculumUnitContent(
		server.Database, authorID, proposalID, unitID, request.FormValue("content"),
	); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposalUnit(writer, request, proposalID, unitID)
}

func (server *Server) deleteCurriculumUnit(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	unitID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid unit ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	if err := services.DeleteCurriculumUnit(server.Database, authorID, proposalID, unitID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) createUnitDependency(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	unitID, unitErr := parsePositiveID(request.FormValue("unit_id"))
	prerequisiteID, prerequisiteErr := parsePositiveID(request.FormValue("prerequisite_id"))
	if unitErr != nil || prerequisiteErr != nil {
		http.Error(writer, "Invalid dependency", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	if err := services.AddUnitDependency(server.Database, authorID, proposalID, unitID, prerequisiteID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposalPanel(writer, request, proposalID, "edit_dependencies", unitID)
}

func (server *Server) deleteUnitDependency(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	unitID, unitErr := parsePositiveID(request.FormValue("unit_id"))
	prerequisiteID, prerequisiteErr := parsePositiveID(request.FormValue("prerequisite_id"))
	if unitErr != nil || prerequisiteErr != nil {
		http.Error(writer, "Invalid dependency", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	if err := services.RemoveUnitDependency(server.Database, authorID, proposalID, unitID, prerequisiteID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposalPanel(writer, request, proposalID, "edit_dependencies", unitID)
}

func (server *Server) createCurriculumRecognition(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.FormValue("proposal_id"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, services.ErrProposalNotFound)
		return
	}
	authorID, _ := services.SessionUserID(request)
	err = services.AddCurriculumRecognition(
		server.Database,
		authorID,
		proposalID,
		parseLearningPathUnitIDs(request.Form["source_unit_ids"]),
		parseLearningPathUnitIDs(request.Form["target_unit_ids"]),
	)
	if err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) createCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	authorID, _ := services.SessionUserID(request)
	proposal, err := services.CreateCurriculumProposal(server.Database, authorID, request.FormValue("title"), request.FormValue("rationale"))
	if err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposal.ID)
}

func (server *Server) updateCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.UpdateCurriculumProposal(server.Database, authorID, proposalID, request.FormValue("title"), request.FormValue("rationale")); err != nil {
		if request.Header.Get("HX-Request") == "true" {
			message, _ := curriculumErrorResponse(err)
			server.render(writer, "proposal-metadata-save-status", proposalMetadataSaveStatusView{Error: message})
			return
		}
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	if request.Header.Get("HX-Request") == "true" {
		server.render(writer, "proposal-metadata-save-status", proposalMetadataSaveStatusView{})
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) deleteCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.DeleteCurriculumProposal(server.Database, authorID, proposalID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/curriculum-modification", http.StatusSeeOther)
}

func (server *Server) submitCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.SubmitCurriculumProposal(server.Database, authorID, proposalID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/curriculum-modification", http.StatusSeeOther)
}

func (server *Server) rebaseCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	resolutions := make(map[int64]services.CurriculumProposalRebaseResolution)
	for key, values := range request.Form {
		if !strings.HasPrefix(key, "resolution_") || len(values) == 0 {
			continue
		}
		changeID, parseErr := parsePositiveID(strings.TrimPrefix(key, "resolution_"))
		if parseErr == nil {
			resolutions[changeID] = services.CurriculumProposalRebaseResolution{
				Choice:  values[0],
				Content: request.FormValue(fmt.Sprintf("resolution_content_%d", changeID)),
			}
		}
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.ResolveCurriculumProposalRebase(
		server.Database, authorID, proposalID, resolutions,
	); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) deleteCurriculumProposalChange(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	proposalID, proposalErr := parsePositiveID(request.PathValue("id"))
	changeID, changeErr := parsePositiveID(request.PathValue("changeID"))
	if proposalErr != nil || changeErr != nil {
		http.Error(writer, "Invalid proposal change", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.DeleteCurriculumProposalChange(server.Database, authorID, proposalID, changeID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) parseCurriculumMutation(writer http.ResponseWriter, request *http.Request) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "Invalid form", http.StatusBadRequest)
		return false
	}
	if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return false
	}
	return true
}

func (server *Server) renderCurriculumMutationError(writer http.ResponseWriter, request *http.Request, err error) {
	message, status := curriculumErrorResponse(err)
	if status == http.StatusInternalServerError {
		log.Printf("modify curriculum: %v", err)
	}
	server.renderCurriculumModification(writer, request, status, message)
}

func curriculumErrorResponse(err error) (string, int) {
	var prerequisiteError *services.UnitIsPrerequisiteError
	switch services.ClassifyDomainError(err) {
	case services.DomainErrorUnitIsPrerequisite:
		errors.As(err, &prerequisiteError)
		return "Remove the dependencies from " + joinNames(prerequisiteError.DependentNames) + " before deleting this unit.", http.StatusConflict
	case services.DomainErrorUnitNameRequired:
		return "A unit name is required.", http.StatusBadRequest
	case services.DomainErrorUnitNameTooLong:
		return "A unit name cannot exceed 200 characters.", http.StatusBadRequest
	case services.DomainErrorUnitContentRequired:
		return "Unit content cannot be empty.", http.StatusBadRequest
	case services.DomainErrorUnitNotFound:
		return "The selected unit no longer exists.", http.StatusNotFound
	case services.DomainErrorDependencyExists:
		return "That dependency already exists.", http.StatusConflict
	case services.DomainErrorDependencyNotFound:
		return "That dependency no longer exists.", http.StatusNotFound
	case services.DomainErrorDependencyCycle:
		return "That dependency would create a cycle.", http.StatusConflict
	case services.DomainErrorProposalNotFound:
		return "Select an editable draft proposal first.", http.StatusNotFound
	case services.DomainErrorProposalTitleRequired:
		return "A proposal title is required.", http.StatusBadRequest
	case services.DomainErrorProposalTitleTooLong:
		return "A proposal title cannot exceed 200 characters.", http.StatusBadRequest
	case services.DomainErrorProposalRationaleRequired:
		return "Explain the purpose of the proposal.", http.StatusBadRequest
	case services.DomainErrorProposalRationaleTooLong:
		return "A proposal rationale cannot exceed 1000 characters.", http.StatusBadRequest
	case services.DomainErrorProposalEmpty:
		return "Add at least one proposed change before publishing.", http.StatusBadRequest
	case services.DomainErrorProposalOutdated:
		return "The proposal base could not be reconciled with the accepted curriculum history.", http.StatusConflict
	case services.DomainErrorProposalRebaseRequired:
		return "Review the proposal changes that overlap with newer accepted work before continuing.", http.StatusConflict
	case services.DomainErrorRebaseResolutionRequired:
		return "Choose a valid resolution for every conflicting change.", http.StatusBadRequest
	case services.DomainErrorRecognitionSourcesRequired:
		return "Select at least one source unit.", http.StatusBadRequest
	case services.DomainErrorRecognitionTargetsRequired:
		return "Select at least one target unit.", http.StatusBadRequest
	case services.DomainErrorProposalInvalid:
		return err.Error(), http.StatusConflict
	default:
		return "Unable to modify the curriculum.", http.StatusInternalServerError
	}
}

func redirectToProposal(writer http.ResponseWriter, request *http.Request, proposalID int64) {
	http.Redirect(writer, request, "/curriculum-modification?proposal="+strconv.FormatInt(proposalID, 10), http.StatusSeeOther)
}

func redirectToProposalUnit(writer http.ResponseWriter, request *http.Request, proposalID, unitID int64) {
	target := "/curriculum-modification?proposal=" + strconv.FormatInt(proposalID, 10) +
		"&unit=" + strconv.FormatInt(unitID, 10) +
		"&content=" + strconv.FormatInt(unitID, 10)
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func redirectToProposalPanel(writer http.ResponseWriter, request *http.Request, proposalID int64, panel string, subjectID int64) {
	target := "/curriculum-modification?proposal=" + strconv.FormatInt(proposalID, 10) +
		"&" + panel + "=" + strconv.FormatInt(subjectID, 10)
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func joinNames(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	result := ""
	for index, name := range names {
		switch {
		case index == 0:
			result = name
		case index == len(names)-1:
			result += " and " + name
		default:
			result += ", " + name
		}
	}
	return result
}
