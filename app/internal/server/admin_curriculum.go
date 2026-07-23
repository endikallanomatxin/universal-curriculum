package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type curriculumUnitView struct {
	models.Unit
	Prerequisites []models.Unit
	Dependents    []models.Unit
	Lane          float64
}

type adminCurriculumPageData struct {
	userPageData
	Units          []curriculumUnitView
	Dependencies   []models.UnitDependency
	Graph          *models.CurriculumGraphLayout
	FocusedUnit    *models.Unit
	ContentUnit    *curriculumUnitView
	Proposals      []models.CurriculumProposal
	ActiveProposal *models.CurriculumProposal
	Error          string
}

func (server *Server) adminCurriculum(writer http.ResponseWriter, request *http.Request) {
	server.renderAdminCurriculum(writer, request, http.StatusOK, "")
}

func (server *Server) createCurriculumUnit(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
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
	if !server.parseAdminMutation(writer, request) {
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
	if !server.parseAdminMutation(writer, request) {
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
	if !server.parseAdminMutation(writer, request) {
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
	if !server.parseAdminMutation(writer, request) {
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
	if !server.parseAdminMutation(writer, request) {
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

func (server *Server) createCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
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
	if !server.parseAdminMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.UpdateCurriculumProposal(server.Database, authorID, proposalID, request.FormValue("title"), request.FormValue("rationale")); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	redirectToProposal(writer, request, proposalID)
}

func (server *Server) deleteCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
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
	http.Redirect(writer, request, "/admin/curriculum", http.StatusSeeOther)
}

func (server *Server) publishCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.PublishCurriculumProposal(server.Database, authorID, proposalID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin/curriculum", http.StatusSeeOther)
}

func (server *Server) deleteCurriculumProposalChange(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
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

func (server *Server) revertCurriculumProposal(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
		return
	}
	proposalID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid proposal ID", http.StatusBadRequest)
		return
	}
	authorID, _ := services.SessionUserID(request)
	if err := services.RevertCurriculumProposal(server.Database, authorID, proposalID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin/curriculum", http.StatusSeeOther)
}

func (server *Server) parseAdminMutation(writer http.ResponseWriter, request *http.Request) bool {
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
	server.renderAdminCurriculum(writer, request, status, message)
}

func curriculumErrorResponse(err error) (string, int) {
	var prerequisiteError *services.UnitIsPrerequisiteError
	switch {
	case errors.As(err, &prerequisiteError):
		return "Remove the dependencies from " + joinNames(prerequisiteError.DependentNames) + " before deleting this unit.", http.StatusConflict
	case errors.Is(err, services.ErrUnitNameRequired):
		return "A unit name is required.", http.StatusBadRequest
	case errors.Is(err, services.ErrUnitContentRequired):
		return "Unit content cannot be empty.", http.StatusBadRequest
	case errors.Is(err, services.ErrUnitNotFound):
		return "The selected unit no longer exists.", http.StatusNotFound
	case errors.Is(err, services.ErrDependencyExists):
		return "That dependency already exists.", http.StatusConflict
	case errors.Is(err, services.ErrDependencyNotFound):
		return "That dependency no longer exists.", http.StatusNotFound
	case errors.Is(err, services.ErrDependencyCycle):
		return "That dependency would create a cycle.", http.StatusConflict
	case errors.Is(err, services.ErrNoProposalToRevert):
		return "There is no published proposal to revert.", http.StatusConflict
	case errors.Is(err, services.ErrProposalNotFound):
		return "Select an editable draft proposal first.", http.StatusNotFound
	case errors.Is(err, services.ErrProposalTitleRequired):
		return "A proposal title is required.", http.StatusBadRequest
	case errors.Is(err, services.ErrProposalRationaleRequired):
		return "Explain the purpose of the proposal.", http.StatusBadRequest
	case errors.Is(err, services.ErrProposalEmpty):
		return "Add at least one proposed change before publishing.", http.StatusBadRequest
	case errors.Is(err, services.ErrProposalOutdated):
		return "This draft was based on an older curriculum version. Create a fresh proposal.", http.StatusConflict
	default:
		return "Unable to modify the curriculum.", http.StatusInternalServerError
	}
}

func (server *Server) renderAdminCurriculum(writer http.ResponseWriter, request *http.Request, status int, message string) {
	userID, _ := services.SessionUserID(request)
	user, err := db.GetUserByID(server.Database, userID)
	if err != nil || user == nil {
		log.Printf("load curriculum administrator: %v", err)
		http.Error(writer, "Unable to load administrator", http.StatusInternalServerError)
		return
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		log.Printf("load curriculum graph: %v", err)
		http.Error(writer, "Unable to load curriculum", http.StatusInternalServerError)
		return
	}
	var focusID *int64
	if unitValue := request.URL.Query().Get("unit"); unitValue != "" {
		unitID, parseErr := parsePositiveID(unitValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		focusID = &unitID
	}
	visibleGraph, focusedUnit, boundaries, err := services.CurriculumNeighborhood(graph, focusID)
	if errors.Is(err, services.ErrCurriculumUnitNotFound) {
		http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("build admin curriculum neighborhood: %v", err)
		http.Error(writer, "Unable to navigate curriculum", http.StatusInternalServerError)
		return
	}
	layout, err := services.BuildCurriculumGraphLayoutWithHints(visibleGraph, curriculumLayoutHints(request))
	if err != nil {
		log.Printf("layout curriculum graph: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	layout.Boundaries = boundaries
	proposals, err := db.ListCurriculumProposals(server.Database, 8)
	if err != nil {
		log.Printf("load curriculum proposals: %v", err)
		http.Error(writer, "Unable to load curriculum proposals", http.StatusInternalServerError)
		return
	}
	for index := range proposals {
		if proposals[index].Status == "accepted" && proposals[index].AuthorID != nil {
			proposals[index].CanRevert = true
			break
		}
	}
	var activeProposal *models.CurriculumProposal
	if proposalValue := request.URL.Query().Get("proposal"); proposalValue != "" {
		if proposalID, parseErr := parsePositiveID(proposalValue); parseErr == nil {
			activeProposal, err = db.GetCurriculumProposal(server.Database, proposalID)
			if err != nil {
				log.Printf("load active curriculum proposal: %v", err)
				http.Error(writer, "Unable to load curriculum proposal", http.StatusInternalServerError)
				return
			}
			if activeProposal != nil && (activeProposal.Status != "draft" || activeProposal.AuthorID == nil || *activeProposal.AuthorID != userID) {
				activeProposal = nil
			}
		}
	}
	applyCurriculumChangeLabels(activeProposal, graph)
	data := adminCurriculumPageData{
		userPageData: userPageData{
			User: user, CSRFToken: sessionCSRFToken(request), CurrentSection: "curriculum",
		},
		Dependencies:   graph.Dependencies,
		Graph:          layout,
		FocusedUnit:    focusedUnit,
		Proposals:      proposals,
		ActiveProposal: activeProposal,
		Error:          message,
	}
	data.Units = curriculumUnitViews(graph, layout)
	applyProposedDependencies(data.Units, activeProposal)
	if contentValue := request.URL.Query().Get("content"); contentValue != "" {
		contentID, parseErr := parsePositiveID(contentValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		for index := range data.Units {
			if data.Units[index].ID == contentID {
				data.ContentUnit = &data.Units[index]
				break
			}
		}
		if data.ContentUnit == nil {
			http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
			return
		}
	}
	server.renderStatus(writer, status, "admin-curriculum.html", data)
}

func applyCurriculumChangeLabels(proposal *models.CurriculumProposal, graph *models.CurriculumGraph) {
	if proposal == nil || graph == nil {
		return
	}
	names := make(map[int64]string, len(graph.Units))
	for _, unit := range graph.Units {
		names[unit.ID] = unit.Name
	}
	for index := range proposal.Changes {
		if proposal.Changes[index].UnitName == "" {
			proposal.Changes[index].UnitName = names[proposal.Changes[index].UnitID]
		}
	}
}

func redirectToProposal(writer http.ResponseWriter, request *http.Request, proposalID int64) {
	http.Redirect(writer, request, "/admin/curriculum?proposal="+strconv.FormatInt(proposalID, 10), http.StatusSeeOther)
}

func redirectToProposalUnit(writer http.ResponseWriter, request *http.Request, proposalID, unitID int64) {
	target := "/admin/curriculum?proposal=" + strconv.FormatInt(proposalID, 10) +
		"&unit=" + strconv.FormatInt(unitID, 10) +
		"&content=" + strconv.FormatInt(unitID, 10)
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func redirectToProposalPanel(writer http.ResponseWriter, request *http.Request, proposalID int64, panel string, subjectID int64) {
	target := "/admin/curriculum?proposal=" + strconv.FormatInt(proposalID, 10) +
		"&" + panel + "=" + strconv.FormatInt(subjectID, 10)
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func applyProposedDependencies(views []curriculumUnitView, proposal *models.CurriculumProposal) {
	if proposal == nil {
		return
	}
	byID := make(map[int64]*curriculumUnitView, len(views))
	units := make(map[int64]models.Unit, len(views))
	for index := range views {
		byID[views[index].ID] = &views[index]
		units[views[index].ID] = views[index].Unit
	}
	for _, change := range proposal.Changes {
		if change.PrerequisiteID == nil {
			continue
		}
		view := byID[change.UnitID]
		prerequisite, exists := units[*change.PrerequisiteID]
		if view == nil || !exists {
			continue
		}
		switch change.Kind {
		case "add_dependency":
			alreadyPresent := false
			for _, current := range view.Prerequisites {
				if current.ID == prerequisite.ID {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				view.Prerequisites = append(view.Prerequisites, prerequisite)
			}
		case "remove_dependency":
			filtered := view.Prerequisites[:0]
			for _, current := range view.Prerequisites {
				if current.ID != prerequisite.ID {
					filtered = append(filtered, current)
				}
			}
			view.Prerequisites = filtered
		}
	}
}

func curriculumUnitViews(graph *models.CurriculumGraph, layout *models.CurriculumGraphLayout) []curriculumUnitView {
	if graph == nil {
		return nil
	}
	unitsByID := make(map[int64]models.Unit, len(graph.Units))
	viewsByID := make(map[int64]*curriculumUnitView, len(graph.Units))
	views := make([]curriculumUnitView, len(graph.Units))
	for index, unit := range graph.Units {
		unitsByID[unit.ID] = unit
		views[index].Unit = unit
		viewsByID[unit.ID] = &views[index]
	}
	for _, dependency := range graph.Dependencies {
		dependent := viewsByID[dependency.UnitID]
		prerequisite := viewsByID[dependency.PrerequisiteID]
		if dependent == nil || prerequisite == nil {
			continue
		}
		dependent.Prerequisites = append(dependent.Prerequisites, unitsByID[dependency.PrerequisiteID])
		prerequisite.Dependents = append(prerequisite.Dependents, unitsByID[dependency.UnitID])
	}
	if layout != nil && len(layout.Nodes) == len(views) {
		ordered := make([]curriculumUnitView, 0, len(views))
		for _, node := range layout.Nodes {
			view := viewsByID[node.ID]
			if view == nil {
				continue
			}
			view.Lane = node.Lane
			ordered = append(ordered, *view)
		}
		views = ordered
	}
	return views
}

func parsePositiveID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid positive ID")
	}
	return id, nil
}

func sessionCSRFToken(request *http.Request) string {
	token, _ := services.SessionCSRFToken(request)
	return token
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
