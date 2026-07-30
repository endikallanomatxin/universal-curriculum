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
	GraphView      curriculumGraphView
	GraphSearch    unitNavigationSearchView
	FocusedUnit    *models.Unit
	ContentUnit    *curriculumUnitView
	DraftProposals []models.CurriculumProposal
	Proposals      []models.CurriculumProposal
	ActiveProposal *models.CurriculumProposal
	ProposalView   string
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
	redirectToProposalView(writer, request, proposalID, "details")
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
	http.Redirect(writer, request, "/curriculum-modification", http.StatusSeeOther)
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
	http.Redirect(writer, request, "/curriculum-modification", http.StatusSeeOther)
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
	case errors.Is(err, services.ErrProposalNotFound):
		return "Select an editable draft proposal first.", http.StatusNotFound
	case errors.Is(err, services.ErrProposalTitleRequired):
		return "A proposal title is required.", http.StatusBadRequest
	case errors.Is(err, services.ErrProposalRationaleRequired):
		return "Explain the purpose of the proposal.", http.StatusBadRequest
	case errors.Is(err, services.ErrProposalEmpty):
		return "Add at least one proposed change before publishing.", http.StatusBadRequest
	case errors.Is(err, services.ErrProposalOutdated):
		return "The curriculum changed after this draft was created. Create a fresh proposal.", http.StatusConflict
	case errors.Is(err, services.ErrProposalInvalid):
		return err.Error(), http.StatusConflict
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
	completedUnitIDs, err := db.CompletedUnitIDs(server.Database, userID)
	if err != nil {
		log.Printf("load curriculum completion indicators: %v", err)
		http.Error(writer, "Unable to load progress", http.StatusInternalServerError)
		return
	}
	var activeProposal *models.CurriculumProposal
	proposalView := ""
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
			if activeProposal != nil {
				proposalView = request.URL.Query().Get("view")
				if proposalView == "" {
					proposalView = "work"
				}
				if proposalView != "work" && proposalView != "details" {
					http.Error(writer, "Invalid proposal view", http.StatusBadRequest)
					return
				}
			}
		}
	}
	workingGraph := curriculumGraphWithProposal(graph, activeProposal)
	applyCurriculumChangeLabels(activeProposal, workingGraph)
	visualGraph := curriculumGraphWithRemovedDependencies(workingGraph, graph, activeProposal)
	var focusID *int64
	if unitValue := request.URL.Query().Get("unit"); unitValue != "" {
		unitID, parseErr := parsePositiveID(unitValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		focusID = &unitID
	}
	visibleGraph, focusedUnit, boundaries, err := services.CurriculumNeighborhood(visualGraph, focusID)
	if errors.Is(err, services.ErrCurriculumUnitNotFound) {
		http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("build admin curriculum neighborhood: %v", err)
		http.Error(writer, "Unable to navigate curriculum", http.StatusInternalServerError)
		return
	}
	includeCreatedProposalUnits(visibleGraph, visualGraph, activeProposal)
	layout, err := services.BuildCurriculumGraphLayoutWithHints(visibleGraph, curriculumLayoutHints(request))
	if err != nil {
		log.Printf("layout curriculum graph: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	layout.Boundaries = boundaries
	positionIsolatedCreatedUnits(layout, activeProposal)
	proposals, err := db.ListCurriculumProposals(server.Database, 8)
	if err != nil {
		log.Printf("load curriculum proposals: %v", err)
		http.Error(writer, "Unable to load curriculum proposals", http.StatusInternalServerError)
		return
	}
	draftProposals, err := db.ListDraftCurriculumProposalsByAuthor(server.Database, userID)
	if err != nil {
		log.Printf("load draft curriculum proposals: %v", err)
		http.Error(writer, "Unable to load draft curriculum proposals", http.StatusInternalServerError)
		return
	}
	data := adminCurriculumPageData{
		userPageData: userPageData{
			User: user, CSRFToken: sessionCSRFToken(request), CurrentSection: "curriculum",
		},
		Dependencies:   graph.Dependencies,
		Graph:          layout,
		FocusedUnit:    focusedUnit,
		Proposals:      proposals,
		DraftProposals: draftProposals,
		ActiveProposal: activeProposal,
		ProposalView:   proposalView,
		Error:          message,
	}
	data.Units = curriculumUnitViews(workingGraph, layout)
	if contentValue := request.URL.Query().Get("content"); proposalView == "work" && contentValue != "" {
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
	unitURL := func(unitID int64) string {
		target := "/curriculum-modification?"
		if activeProposal != nil {
			target += "proposal=" + strconv.FormatInt(activeProposal.ID, 10) + "&view=work&"
		}
		return target + "unit=" + strconv.FormatInt(unitID, 10)
	}
	navigateURL, contentURL := curriculumUnitURLs(unitURL, data.ContentUnit != nil)
	data.GraphView = newCurriculumGraphView(
		"admin-curriculum",
		"Arrows go from each prerequisite to the units that depend on it. Select a unit to navigate, or use its document action to open the content.",
		layout,
		focusedUnit,
		nil,
		completedUnitIDs,
		true,
		navigateURL,
		contentURL,
	)
	applyProposalGraphStates(&data.GraphView, activeProposal)
	data.GraphSearch = newUnitNavigationSearchView(
		"admin-graph-search-results",
		"Find a unit in the curriculum",
		workingGraph.Units,
		navigateURL,
	)
	server.renderStatus(writer, status, "admin-curriculum.html", data)
}

func curriculumGraphWithProposal(graph *models.CurriculumGraph, proposal *models.CurriculumProposal) *models.CurriculumGraph {
	return services.CurriculumGraphWithProposal(graph, proposal)
}

func curriculumGraphWithRemovedDependencies(working, published *models.CurriculumGraph, proposal *models.CurriculumProposal) *models.CurriculumGraph {
	if working == nil || proposal == nil {
		return working
	}
	visual := &models.CurriculumGraph{
		Units:        append([]models.Unit(nil), working.Units...),
		Dependencies: append([]models.UnitDependency(nil), working.Dependencies...),
	}
	publishedDependencies := make(map[[2]int64]models.UnitDependency)
	if published != nil {
		for _, dependency := range published.Dependencies {
			publishedDependencies[[2]int64{dependency.PrerequisiteID, dependency.UnitID}] = dependency
		}
	}
	for _, change := range proposal.Changes {
		if change.Kind != "remove_dependency" || change.PrerequisiteID == nil {
			continue
		}
		key := [2]int64{*change.PrerequisiteID, change.UnitID}
		dependency, exists := publishedDependencies[key]
		if !exists {
			dependency = models.UnitDependency{PrerequisiteID: key[0], UnitID: key[1]}
		}
		if !graphHasDependency(visual, key[1], key[0]) {
			visual.Dependencies = append(visual.Dependencies, dependency)
		}
	}
	return visual
}

func graphHasDependency(graph *models.CurriculumGraph, unitID, prerequisiteID int64) bool {
	for _, dependency := range graph.Dependencies {
		if dependency.UnitID == unitID && dependency.PrerequisiteID == prerequisiteID {
			return true
		}
	}
	return false
}

func includeCreatedProposalUnits(visible, working *models.CurriculumGraph, proposal *models.CurriculumProposal) {
	if visible == nil || working == nil || proposal == nil {
		return
	}
	visibleIDs := make(map[int64]bool, len(visible.Units))
	for _, unit := range visible.Units {
		visibleIDs[unit.ID] = true
	}
	workingUnits := make(map[int64]models.Unit, len(working.Units))
	for _, unit := range working.Units {
		workingUnits[unit.ID] = unit
	}
	for _, change := range proposal.Changes {
		if change.Kind == "create_unit" && !visibleIDs[change.UnitID] {
			if unit, exists := workingUnits[change.UnitID]; exists {
				visible.Units = append(visible.Units, unit)
				visibleIDs[change.UnitID] = true
			}
		}
	}
	createdIDs := make(map[int64]bool)
	for _, change := range proposal.Changes {
		if change.Kind == "create_unit" {
			createdIDs[change.UnitID] = true
		}
	}
	visibleDependencies := make(map[[2]int64]bool, len(visible.Dependencies))
	for _, dependency := range visible.Dependencies {
		visibleDependencies[[2]int64{dependency.PrerequisiteID, dependency.UnitID}] = true
	}
	for _, dependency := range working.Dependencies {
		if !createdIDs[dependency.UnitID] && !createdIDs[dependency.PrerequisiteID] {
			continue
		}
		for _, unitID := range []int64{dependency.PrerequisiteID, dependency.UnitID} {
			if !visibleIDs[unitID] {
				if unit, exists := workingUnits[unitID]; exists {
					visible.Units = append(visible.Units, unit)
					visibleIDs[unitID] = true
				}
			}
		}
		key := [2]int64{dependency.PrerequisiteID, dependency.UnitID}
		if !visibleDependencies[key] {
			visible.Dependencies = append(visible.Dependencies, dependency)
			visibleDependencies[key] = true
		}
	}
}

func positionIsolatedCreatedUnits(layout *models.CurriculumGraphLayout, proposal *models.CurriculumProposal) {
	if layout == nil || proposal == nil {
		return
	}
	createdIDs := make(map[int64]bool)
	for _, change := range proposal.Changes {
		if change.Kind == "create_unit" {
			createdIDs[change.UnitID] = true
		}
	}
	connectedIDs := make(map[int64]bool)
	for _, edge := range layout.Edges {
		connectedIDs[edge.PrerequisiteID] = true
		connectedIDs[edge.DependentID] = true
	}
	isolated := make([]models.CurriculumGraphNode, 0)
	connected := make([]models.CurriculumGraphNode, 0, len(layout.Nodes))
	for _, node := range layout.Nodes {
		if createdIDs[node.ID] && !connectedIDs[node.ID] {
			isolated = append(isolated, node)
		} else {
			connected = append(connected, node)
		}
	}
	layout.Nodes = append(isolated, connected...)
}

func applyProposalGraphStates(view *curriculumGraphView, proposal *models.CurriculumProposal) {
	if view == nil || proposal == nil {
		return
	}
	priority := map[string]int{"content": 1, "rename": 2, "created": 3, "deleted": 4}
	states := make(map[int64]string)
	for _, change := range proposal.Changes {
		state := ""
		switch change.Kind {
		case "create_unit":
			state = "created"
		case "delete_unit":
			state = "deleted"
		case "rename_unit":
			state = "rename"
		case "update_content":
			state = "content"
		}
		if priority[state] > priority[states[change.UnitID]] {
			states[change.UnitID] = state
		}
	}
	for index := range view.Nodes {
		view.Nodes[index].ProposalState = states[view.Nodes[index].ID]
	}
	connectedIDs := make(map[int64]bool)
	for _, edge := range view.Edges {
		connectedIDs[edge.PrerequisiteID] = true
		connectedIDs[edge.DependentID] = true
	}
	for index := range view.Nodes {
		node := &view.Nodes[index]
		node.IsProposedIsolated = node.ProposalState == "created" && !connectedIDs[node.ID]
	}
	proposedDependencies := make(map[[2]int64]string)
	for _, change := range proposal.Changes {
		if change.PrerequisiteID == nil {
			continue
		}
		key := [2]int64{*change.PrerequisiteID, change.UnitID}
		switch change.Kind {
		case "add_dependency":
			proposedDependencies[key] = "created"
		case "remove_dependency":
			proposedDependencies[key] = "deleted"
		}
	}
	for index := range view.Edges {
		edge := &view.Edges[index]
		edge.ProposalState = proposedDependencies[[2]int64{edge.PrerequisiteID, edge.DependentID}]
	}
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
	redirectToProposalView(writer, request, proposalID, "work")
}

func redirectToProposalView(writer http.ResponseWriter, request *http.Request, proposalID int64, view string) {
	http.Redirect(writer, request, "/curriculum-modification?proposal="+strconv.FormatInt(proposalID, 10)+"&view="+view, http.StatusSeeOther)
}

func redirectToProposalUnit(writer http.ResponseWriter, request *http.Request, proposalID, unitID int64) {
	target := "/curriculum-modification?proposal=" + strconv.FormatInt(proposalID, 10) +
		"&view=work&unit=" + strconv.FormatInt(unitID, 10) +
		"&content=" + strconv.FormatInt(unitID, 10)
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

func redirectToProposalPanel(writer http.ResponseWriter, request *http.Request, proposalID int64, panel string, subjectID int64) {
	target := "/curriculum-modification?proposal=" + strconv.FormatInt(proposalID, 10) +
		"&view=work&" + panel + "=" + strconv.FormatInt(subjectID, 10)
	http.Redirect(writer, request, target, http.StatusSeeOther)
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
