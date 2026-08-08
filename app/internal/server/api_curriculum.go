package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

type apiUnitSummary struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Retired bool   `json:"retired"`
}

type apiUnit struct {
	apiUnitSummary
	Content         string    `json:"content"`
	PrerequisiteIDs []int64   `json:"prerequisite_ids"`
	DependentIDs    []int64   `json:"dependent_ids"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type apiDependency struct {
	UnitID         int64 `json:"unit_id"`
	PrerequisiteID int64 `json:"prerequisite_id"`
}

func (server *Server) apiGetCurriculum(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		log.Printf("API get curriculum: %v", err)
		writeAPIInternalError(writer)
		return
	}
	proposalID, err := db.GetCurrentCurriculumProposalID(server.Database)
	if err != nil {
		log.Printf("API get curriculum projection: %v", err)
		writeAPIInternalError(writer)
		return
	}
	units, dependencies := apiCurriculumResources(graph)
	writeAPIJSON(writer, http.StatusOK, struct {
		ProposalID   *int64          `json:"proposal_id"`
		Units        []apiUnit       `json:"units"`
		Dependencies []apiDependency `json:"dependencies"`
	}{proposalID, units, dependencies})
}

func (server *Server) apiListUnits(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request, "query", "limit", "offset"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	limit, offset, err := apiPagination(request)
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("query"))
	if len([]rune(query)) > 200 {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", "query must not exceed 200 characters", nil)
		return
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		log.Printf("API list units: %v", err)
		writeAPIInternalError(writer)
		return
	}
	query = strings.ToLower(query)
	units := make([]apiUnitSummary, 0, len(graph.Units))
	for _, unit := range graph.Units {
		if query != "" && !strings.Contains(strings.ToLower(unit.Name), query) &&
			!strings.Contains(strings.ToLower(unit.Content), query) {
			continue
		}
		units = append(units, apiUnitSummary{ID: unit.ID, Name: unit.Name, Retired: unit.Retired})
	}
	total := len(units)
	pageUnits := make([]apiUnitSummary, 0)
	if offset < total {
		end := min(offset+limit, total)
		pageUnits = units[offset:end]
	}
	writeAPIJSON(writer, http.StatusOK, struct {
		Units []apiUnitSummary `json:"units"`
		Page  apiPage          `json:"page"`
	}{pageUnits, apiPageFor(limit, offset, len(pageUnits), total)})
}

func (server *Server) apiGetUnit(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	unitID, err := apiPathID(request, "unitId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "unitId must be a positive integer.", nil)
		return
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		log.Printf("API get unit: %v", err)
		writeAPIInternalError(writer)
		return
	}
	units, _ := apiCurriculumResources(graph)
	for _, unit := range units {
		if unit.ID == unitID {
			writeAPIJSON(writer, http.StatusOK, unit)
			return
		}
	}
	writeAPIError(writer, http.StatusNotFound, "unit_not_found", "The curriculum unit was not found.", nil)
}

func (server *Server) apiListAcceptedProposals(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request, "limit", "offset"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	limit, offset, err := apiPagination(request)
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	proposals, total, err := db.ListAcceptedCurriculumProposals(server.Database, limit, offset)
	if err != nil {
		log.Printf("API list accepted proposals: %v", err)
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

func (server *Server) apiGetAcceptedProposal(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	proposalID, err := apiPathID(request, "proposalId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "proposalId must be a positive integer.", nil)
		return
	}
	proposal, err := db.GetCurriculumProposal(server.Database, proposalID)
	if err != nil {
		log.Printf("API get accepted proposal: %v", err)
		writeAPIInternalError(writer)
		return
	}
	if proposal == nil || proposal.Status != "accepted" {
		writeAPIError(writer, http.StatusNotFound, "proposal_not_found", "The accepted proposal was not found.", nil)
		return
	}
	writeAPIJSON(writer, http.StatusOK, newAPIProposal(*proposal, true))
}

func apiCurriculumResources(graph *models.CurriculumGraph) ([]apiUnit, []apiDependency) {
	units := make([]apiUnit, 0, len(graph.Units))
	indexes := make(map[int64]int, len(graph.Units))
	for _, unit := range graph.Units {
		indexes[unit.ID] = len(units)
		units = append(units, apiUnit{
			apiUnitSummary: apiUnitSummary{ID: unit.ID, Name: unit.Name, Retired: unit.Retired},
			Content:        unit.Content, PrerequisiteIDs: []int64{}, DependentIDs: []int64{},
			CreatedAt: unit.CreatedAt, UpdatedAt: unit.UpdatedAt,
		})
	}
	dependencies := make([]apiDependency, 0, len(graph.Dependencies))
	for _, dependency := range graph.Dependencies {
		dependencies = append(dependencies, apiDependency{
			UnitID: dependency.UnitID, PrerequisiteID: dependency.PrerequisiteID,
		})
		if index, ok := indexes[dependency.UnitID]; ok {
			units[index].PrerequisiteIDs = append(units[index].PrerequisiteIDs, dependency.PrerequisiteID)
		}
		if index, ok := indexes[dependency.PrerequisiteID]; ok {
			units[index].DependentIDs = append(units[index].DependentIDs, dependency.UnitID)
		}
	}
	return units, dependencies
}
