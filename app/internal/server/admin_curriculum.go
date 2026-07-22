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
	Units        []curriculumUnitView
	Dependencies []models.UnitDependency
	Graph        *models.CurriculumGraphLayout
	Error        string
}

func (server *Server) adminCurriculum(writer http.ResponseWriter, request *http.Request) {
	server.renderAdminCurriculum(writer, request, http.StatusOK, "")
}

func (server *Server) createCurriculumUnit(writer http.ResponseWriter, request *http.Request) {
	if !server.parseAdminMutation(writer, request) {
		return
	}
	_, err := services.CreateCurriculumUnit(
		server.Database,
		request.FormValue("name"),
		request.FormValue("description"),
	)
	if err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin/curriculum", http.StatusSeeOther)
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
	if err := services.DeleteCurriculumUnit(server.Database, unitID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin/curriculum", http.StatusSeeOther)
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
	if err := services.AddUnitDependency(server.Database, unitID, prerequisiteID); err != nil {
		server.renderCurriculumMutationError(writer, request, err)
		return
	}
	http.Redirect(writer, request, "/admin/curriculum", http.StatusSeeOther)
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
	if err := services.RemoveUnitDependency(server.Database, unitID, prerequisiteID); err != nil {
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
	case errors.Is(err, services.ErrUnitDescriptionRequired):
		return "A short description is required.", http.StatusBadRequest
	case errors.Is(err, services.ErrUnitNotFound):
		return "The selected unit no longer exists.", http.StatusNotFound
	case errors.Is(err, services.ErrDependencyExists):
		return "That dependency already exists.", http.StatusConflict
	case errors.Is(err, services.ErrDependencyNotFound):
		return "That dependency no longer exists.", http.StatusNotFound
	case errors.Is(err, services.ErrDependencyCycle):
		return "That dependency would create a cycle.", http.StatusConflict
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
	layout, err := services.BuildCurriculumGraphLayout(graph)
	if err != nil {
		log.Printf("layout curriculum graph: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	data := adminCurriculumPageData{
		userPageData: userPageData{
			User: user, CSRFToken: sessionCSRFToken(request), CurrentSection: "curriculum",
		},
		Dependencies: graph.Dependencies,
		Graph:        layout,
		Error:        message,
	}
	data.Units = curriculumUnitViews(graph, layout)
	server.renderStatus(writer, status, "admin-curriculum.html", data)
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
