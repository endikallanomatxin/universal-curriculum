package server

import (
	"errors"
	"log"
	"net/http"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type apiLearningPath struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	TargetUnits []apiUnitSummary `json:"target_units"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type apiLearningPathInput struct {
	Name          string  `json:"name"`
	TargetUnitIDs []int64 `json:"target_unit_ids"`
}

type apiProgress struct {
	UnitID     int64 `json:"unit_id"`
	Direct     bool  `json:"direct"`
	Recognized bool  `json:"recognized"`
	Completed  bool  `json:"completed"`
}

type apiRecommendation struct {
	LearningPathID   int64                `json:"learning_path_id"`
	LearningPathName string               `json:"learning_path_name"`
	Units            []apiRecommendedUnit `json:"units"`
}

type apiRecommendedUnit struct {
	apiUnitSummary
	Reason string `json:"reason"`
}

func (server *Server) apiListLearningPaths(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	user := apiUser(request)
	paths, err := db.ListLearningPaths(server.Database, user.ID)
	if err != nil {
		log.Printf("API list learning paths: %v", err)
		writeAPIInternalError(writer)
		return
	}
	resources := make([]apiLearningPath, 0, len(paths))
	for _, path := range paths {
		resources = append(resources, newAPILearningPath(path))
	}
	writeAPIJSON(writer, http.StatusOK, struct {
		LearningPaths []apiLearningPath `json:"learning_paths"`
	}{resources})
}

func (server *Server) apiCreateLearningPath(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	var input apiLearningPathInput
	if err := decodeAPIJSON(writer, request, &input); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return
	}
	if err := validateAPIIDs(input.TargetUnitIDs, "target_unit_ids"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", err.Error(), nil)
		return
	}
	user := apiUser(request)
	path, err := services.CreateLearningPath(server.Database, user.ID, input.Name, input.TargetUnitIDs)
	if err != nil {
		server.writeAPILearningPathError(writer, err)
		return
	}
	path, err = db.GetLearningPath(server.Database, user.ID, path.ID)
	if err != nil {
		log.Printf("API reload created learning path: %v", err)
		writeAPIInternalError(writer)
		return
	}
	writeAPIJSON(writer, http.StatusCreated, newAPILearningPath(*path))
}

func (server *Server) apiGetLearningPath(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	pathID, ok := apiLearningPathID(writer, request)
	if !ok {
		return
	}
	path, err := db.GetLearningPath(server.Database, apiUser(request).ID, pathID)
	if err != nil {
		log.Printf("API get learning path: %v", err)
		writeAPIInternalError(writer)
		return
	}
	if path == nil {
		writeAPIError(writer, http.StatusNotFound, "learning_path_not_found", "The learning path was not found.", nil)
		return
	}
	writeAPIJSON(writer, http.StatusOK, newAPILearningPath(*path))
}

func (server *Server) apiUpdateLearningPath(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	pathID, ok := apiLearningPathID(writer, request)
	if !ok {
		return
	}
	var input apiLearningPathInput
	if err := decodeAPIJSON(writer, request, &input); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return
	}
	if err := validateAPIIDs(input.TargetUnitIDs, "target_unit_ids"); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", err.Error(), nil)
		return
	}
	user := apiUser(request)
	if err := services.UpdateLearningPath(server.Database, user.ID, pathID, input.Name, input.TargetUnitIDs); err != nil {
		server.writeAPILearningPathError(writer, err)
		return
	}
	path, err := db.GetLearningPath(server.Database, user.ID, pathID)
	if err != nil {
		log.Printf("API reload updated learning path: %v", err)
		writeAPIInternalError(writer)
		return
	}
	writeAPIJSON(writer, http.StatusOK, newAPILearningPath(*path))
}

func (server *Server) apiDeleteLearningPath(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	pathID, ok := apiLearningPathID(writer, request)
	if !ok {
		return
	}
	deleted, err := db.DeleteLearningPath(server.Database, apiUser(request).ID, pathID)
	if err != nil {
		log.Printf("API delete learning path: %v", err)
		writeAPIInternalError(writer)
		return
	}
	if !deleted {
		writeAPIError(writer, http.StatusNotFound, "learning_path_not_found", "The learning path was not found.", nil)
		return
	}
	writeAPIJSON(writer, http.StatusNoContent, nil)
}

func (server *Server) apiListRecommendations(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	user := apiUser(request)
	recommendationModels, err := services.LearningRecommendations(server.Database, user.ID)
	if err != nil {
		log.Printf("API load learning recommendations: %v", err)
		writeAPIInternalError(writer)
		return
	}
	recommendations := make([]apiRecommendation, 0, len(recommendationModels))
	for _, model := range recommendationModels {
		recommendation := apiRecommendation{
			LearningPathID: model.LearningPathID, LearningPathName: model.LearningPathName,
			Units: make([]apiRecommendedUnit, 0, len(model.Units)),
		}
		for _, unit := range model.Units {
			recommendation.Units = append(recommendation.Units, apiRecommendedUnit{
				apiUnitSummary: apiUnitSummary{ID: unit.ID, Name: unit.Name, Retired: unit.Retired},
				Reason:         unit.Reason,
			})
		}
		recommendations = append(recommendations, recommendation)
	}
	writeAPIJSON(writer, http.StatusOK, struct {
		Recommendations []apiRecommendation `json:"recommendations"`
	}{recommendations})
}

func (server *Server) apiGetProgress(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	statuses, err := db.UnitCompletionStatuses(server.Database, apiUser(request).ID)
	if err != nil {
		log.Printf("API get progress: %v", err)
		writeAPIInternalError(writer)
		return
	}
	progress := make([]apiProgress, 0, len(statuses))
	for unitID, status := range statuses {
		progress = append(progress, newAPIProgress(unitID, status))
	}
	sortAPIProgress(progress)
	writeAPIJSON(writer, http.StatusOK, struct {
		Progress []apiProgress `json:"progress"`
	}{progress})
}

func (server *Server) apiSetProgress(writer http.ResponseWriter, request *http.Request) {
	if err := validateAPIQuery(request); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_query", err.Error(), nil)
		return
	}
	unitID, err := apiPathID(request, "unitId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "unitId must be a positive integer.", nil)
		return
	}
	var input struct {
		Completed *bool `json:"completed"`
	}
	if err := decodeAPIJSON(writer, request, &input); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_json", err.Error(), nil)
		return
	}
	if input.Completed == nil {
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "completed is required", map[string]string{"completed": "is required"})
		return
	}
	user := apiUser(request)
	status, err := services.SetUnitProgress(server.Database, user.ID, unitID, *input.Completed)
	if errors.Is(err, services.ErrUnitNotFound) {
		writeAPIError(writer, http.StatusNotFound, "unit_not_found", "The curriculum unit was not found.", nil)
		return
	}
	if err != nil {
		log.Printf("API set progress: %v", err)
		writeAPIInternalError(writer)
		return
	}
	writeAPIJSON(writer, http.StatusOK, newAPIProgress(unitID, status))
}

func newAPILearningPath(path models.LearningPath) apiLearningPath {
	resource := apiLearningPath{
		ID: path.ID, Name: path.Name, TargetUnits: make([]apiUnitSummary, 0, len(path.Units)),
		CreatedAt: path.CreatedAt, UpdatedAt: path.UpdatedAt,
	}
	for _, unit := range path.Units {
		resource.TargetUnits = append(resource.TargetUnits, apiUnitSummary{
			ID: unit.ID, Name: unit.Name, Retired: unit.Retired,
		})
	}
	return resource
}

func newAPIProgress(unitID int64, status models.UnitCompletionStatus) apiProgress {
	return apiProgress{
		UnitID: unitID, Direct: status.Direct, Recognized: status.Recognized,
		Completed: status.Completed(),
	}
}

func apiLearningPathID(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	pathID, err := apiPathID(request, "pathId")
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid_id", "pathId must be a positive integer.", nil)
		return 0, false
	}
	return pathID, true
}

func validateAPIIDs(ids []int64, field string) error {
	if len(ids) == 0 {
		return errors.New(field + " must contain at least one unit ID")
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return errors.New(field + " must contain only positive integers")
		}
		if seen[id] {
			return errors.New(field + " must not contain duplicate IDs")
		}
		seen[id] = true
	}
	return nil
}

func sortAPIProgress(progress []apiProgress) {
	for index := 1; index < len(progress); index++ {
		for current := index; current > 0 && progress[current].UnitID < progress[current-1].UnitID; current-- {
			progress[current], progress[current-1] = progress[current-1], progress[current]
		}
	}
}

func (server *Server) writeAPILearningPathError(writer http.ResponseWriter, err error) {
	switch services.ClassifyDomainError(err) {
	case services.DomainErrorLearningPathNotFound:
		writeAPIError(writer, http.StatusNotFound, "learning_path_not_found", "The learning path was not found.", nil)
	case services.DomainErrorLearningPathNameRequired:
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "The request is invalid.", map[string]string{"name": "is required"})
	case services.DomainErrorLearningPathNameTooLong:
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "The request is invalid.", map[string]string{"name": "must not exceed 200 characters"})
	case services.DomainErrorLearningPathUnitsRequired:
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "The request is invalid.", map[string]string{"target_unit_ids": "must contain at least one current unit"})
	case services.DomainErrorUnitNotFound:
		writeAPIError(writer, http.StatusBadRequest, "validation_failed", "The request is invalid.", map[string]string{"target_unit_ids": "contains a unit that does not exist"})
	default:
		log.Printf("API save learning path: %v", err)
		writeAPIInternalError(writer)
	}
}
