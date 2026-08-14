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
	ShowUsers     bool
	Users         []models.User
	Invitations   []models.ContributorInvitation
	SelectedUser  *models.User
	UserProposals []models.CurriculumProposal
	Error         string
}

func (server *Server) administration(writer http.ResponseWriter, request *http.Request) {
	data, err := server.loadAdministrationPage(request)
	if err != nil {
		if errors.Is(err, services.ErrProposalNotFound) {
			http.Error(writer, "User not found", http.StatusNotFound)
			return
		}
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
	data := administrationPageData{userPageData: page}
	if request.URL.Path == "/admin" {
		return data, nil
	}
	data.ShowUsers = true
	users, err := db.ListUsers(server.Database)
	if err != nil {
		return administrationPageData{}, err
	}
	data.Users = users
	data.Invitations, err = db.ListContributorInvitations(server.Database)
	if err != nil {
		return administrationPageData{}, err
	}
	if value := request.PathValue("id"); value != "" {
		selectedID, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || selectedID <= 0 {
			return administrationPageData{}, services.ErrProposalNotFound
		}
		for index := range users {
			if users[index].ID == selectedID {
				data.SelectedUser = &users[index]
				break
			}
		}
		if data.SelectedUser == nil {
			return administrationPageData{}, services.ErrProposalNotFound
		}
		proposals, _, listErr := db.ListCurriculumProposalsForUser(server.Database, page.User.ID, true, "", 100, 0)
		if listErr != nil {
			return administrationPageData{}, listErr
		}
		for _, proposal := range proposals {
			if proposal.HasAuthor(selectedID) {
				data.UserProposals = append(data.UserProposals, proposal)
			}
		}
	}
	return data, nil
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
	http.Redirect(writer, request, "/admin/users", http.StatusSeeOther)
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
	http.Redirect(writer, request, "/admin/users", http.StatusSeeOther)
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
	http.Redirect(writer, request, administrationUserURL(request), http.StatusSeeOther)
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
	http.Redirect(writer, request, administrationUserURL(request), http.StatusSeeOther)
}

func administrationUserURL(request *http.Request) string {
	if userID, err := strconv.ParseInt(request.FormValue("user_id"), 10, 64); err == nil && userID > 0 {
		return "/admin/users/" + strconv.FormatInt(userID, 10)
	}
	return "/admin/users"
}
