package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type administrationPageData struct {
	userPageData
	Users          []models.User
	Invitations    []models.ContributorInvitation
	Proposals      []models.CurriculumProposal
	SelectedUserID int64
	Error          string
}

func (server *Server) administration(writer http.ResponseWriter, request *http.Request) {
	data, err := server.loadAdministrationPage(request)
	if err != nil {
		log.Printf("load administration: %v", err)
		http.Error(writer, "Unable to load administration", http.StatusInternalServerError)
		return
	}
	server.render(writer, "administration.html", data)
}

func (server *Server) loadAdministrationPage(request *http.Request) (administrationPageData, error) {
	page, err := server.loadUserPageData(request, "administration", false)
	if err != nil {
		return administrationPageData{}, err
	}
	users, err := db.ListUsers(server.Database)
	if err != nil {
		return administrationPageData{}, err
	}
	invitations, err := db.ListContributorInvitations(server.Database)
	if err != nil {
		return administrationPageData{}, err
	}
	proposals, _, err := db.ListCurriculumProposalsForUser(server.Database, page.User.ID, true, "", 100, 0)
	if err != nil {
		return administrationPageData{}, err
	}
	selected, _ := strconv.ParseInt(request.URL.Query().Get("user"), 10, 64)
	if selected > 0 {
		filtered := proposals[:0]
		for _, proposal := range proposals {
			if proposal.HasAuthor(selected) {
				filtered = append(filtered, proposal)
			}
		}
		proposals = filtered
	}
	return administrationPageData{userPageData: page, Users: users, Invitations: invitations, Proposals: proposals, SelectedUserID: selected}, nil
}

func (server *Server) createContributorInvitation(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	administratorID, _ := services.SessionUserID(request)
	_, err := services.InviteContributor(request.Context(), server.Database, server.EmailSender, server.Config.AppBaseURL, request.FormValue("email"), administratorID)
	if err != nil {
		log.Printf("invite contributor: %v", err)
		http.Error(writer, "Unable to send contributor invitation", http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, "/admin", http.StatusSeeOther)
}

func (server *Server) revokeContributorInvitation(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	id, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid invitation", http.StatusBadRequest)
		return
	}
	ok, err := db.RevokeContributorInvitation(server.Database, id)
	if err != nil {
		http.Error(writer, "Unable to revoke invitation", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(writer, "Invitation not found", http.StatusNotFound)
		return
	}
	http.Redirect(writer, request, "/admin", http.StatusSeeOther)
}

func (server *Server) acceptCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	id, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal", http.StatusBadRequest)
		return
	}
	administratorID, _ := services.SessionUserID(request)
	if _, err := services.AcceptCurriculumProposal(server.Database, administratorID, id); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin", http.StatusSeeOther)
}

func (server *Server) rejectCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseCurriculumMutation(writer, request) {
		return
	}
	id, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal", http.StatusBadRequest)
		return
	}
	reason := strings.TrimSpace(request.FormValue("reason"))
	administratorID, _ := services.SessionUserID(request)
	if err := services.RejectCurriculumProposal(server.Database, administratorID, id, reason); err != nil {
		if errors.Is(err, services.ErrProposalRationaleRequired) {
			http.Error(writer, "A rejection reason is required", http.StatusBadRequest)
			return
		}
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin", http.StatusSeeOther)
}
