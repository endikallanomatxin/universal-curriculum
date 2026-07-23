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

type learnPageData struct {
	userPageData
	Graph             *models.CurriculumGraphLayout
	FocusedUnit       *models.Unit
	Paths             []models.LearningPath
	SelectedPath      *models.LearningPath
	AllUnits          []models.Unit
	TargetUnitIDs     map[int64]bool
	ExploreAll        bool
	ShowGraph         bool
	TotalUnits        int
	TotalDependencies int
}

func (server *Server) learn(writer http.ResponseWriter, request *http.Request) {
	page, err := server.loadUserPageData(request, "learn", false)
	if err != nil {
		log.Printf("load learn user: %v", err)
		http.Error(writer, "Unable to load user", http.StatusInternalServerError)
		return
	}
	curriculum, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		log.Printf("load learn curriculum: %v", err)
		http.Error(writer, "Unable to load curriculum", http.StatusInternalServerError)
		return
	}
	data := learnPageData{
		userPageData:      page,
		AllUnits:          curriculum.Units,
		TotalUnits:        len(curriculum.Units),
		TotalDependencies: len(curriculum.Dependencies),
	}
	userID, authenticated := services.SessionUserID(request)
	if authenticated {
		data.Paths, err = db.ListLearningPaths(server.Database, userID)
		if err != nil {
			log.Printf("list learning paths: %v", err)
			http.Error(writer, "Unable to load learning paths", http.StatusInternalServerError)
			return
		}
	}

	pathValue := request.URL.Query().Get("path")
	var visibleGraph *models.CurriculumGraph
	switch {
	case pathValue == "":
		server.render(writer, "learn.html", data)
		return
	case pathValue == "all":
		data.ExploreAll, data.ShowGraph = true, true
		visibleGraph = curriculum
	default:
		pathID, parseErr := strconv.ParseInt(pathValue, 10, 64)
		if parseErr != nil || pathID <= 0 || !authenticated {
			http.Error(writer, "Learning path not found", http.StatusNotFound)
			return
		}
		data.SelectedPath, err = db.GetLearningPath(server.Database, userID, pathID)
		if err != nil {
			log.Printf("get learning path: %v", err)
			http.Error(writer, "Unable to load learning path", http.StatusInternalServerError)
			return
		}
		if data.SelectedPath == nil {
			http.Error(writer, "Learning path not found", http.StatusNotFound)
			return
		}
		targetIDs := make([]int64, 0, len(data.SelectedPath.Units))
		data.TargetUnitIDs = make(map[int64]bool, len(data.SelectedPath.Units))
		for _, unit := range data.SelectedPath.Units {
			targetIDs = append(targetIDs, unit.ID)
			data.TargetUnitIDs[unit.ID] = true
		}
		visibleGraph = services.CurriculumPathSubgraph(curriculum, targetIDs)
		data.ShowGraph = true
	}

	var focusID *int64
	if rawID := request.URL.Query().Get("unit"); rawID != "" {
		id, parseErr := strconv.ParseInt(rawID, 10, 64)
		if parseErr != nil || id <= 0 {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		focusID = &id
	}
	var boundaries []models.CurriculumGraphBoundary
	if focusID != nil || data.ExploreAll {
		visibleGraph, data.FocusedUnit, boundaries, err = services.CurriculumNeighborhood(visibleGraph, focusID)
		if errors.Is(err, services.ErrCurriculumUnitNotFound) {
			http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("build curriculum neighborhood: %v", err)
			http.Error(writer, "Unable to navigate curriculum", http.StatusInternalServerError)
			return
		}
	}
	data.Graph, err = services.BuildCurriculumGraphLayoutWithHints(visibleGraph, curriculumLayoutHints(request))
	if err != nil {
		log.Printf("lay out learn curriculum: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	data.Graph.Boundaries = boundaries
	server.render(writer, "learn.html", data)
}

func (server *Server) createLearningPath(writer http.ResponseWriter, request *http.Request) {
	if !server.parseLearningPathMutation(writer, request) {
		return
	}
	userID, _ := services.SessionUserID(request)
	path, err := services.CreateLearningPath(
		server.Database, userID, request.FormValue("name"), request.FormValue("description"),
		parseLearningPathUnitIDs(request.Form["unit_ids"]),
	)
	if err != nil {
		http.Error(writer, learningPathError(err), http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, "/learn?path="+strconv.FormatInt(path.ID, 10), http.StatusSeeOther)
}

func (server *Server) updateLearningPath(writer http.ResponseWriter, request *http.Request) {
	if !server.parseLearningPathMutation(writer, request) {
		return
	}
	pathID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid learning path", http.StatusBadRequest)
		return
	}
	userID, _ := services.SessionUserID(request)
	err = services.UpdateLearningPath(
		server.Database, userID, pathID, request.FormValue("name"), request.FormValue("description"),
		parseLearningPathUnitIDs(request.Form["unit_ids"]),
	)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, services.ErrLearningPathNotFound) {
			status = http.StatusNotFound
		}
		http.Error(writer, learningPathError(err), status)
		return
	}
	http.Redirect(writer, request, "/learn?path="+strconv.FormatInt(pathID, 10), http.StatusSeeOther)
}

func (server *Server) deleteLearningPath(writer http.ResponseWriter, request *http.Request) {
	if !server.parseLearningPathMutation(writer, request) {
		return
	}
	pathID, err := parsePositiveID(request.PathValue("id"))
	if err != nil {
		http.Error(writer, "Invalid learning path", http.StatusBadRequest)
		return
	}
	userID, _ := services.SessionUserID(request)
	ok, err := db.DeleteLearningPath(server.Database, userID, pathID)
	if err != nil {
		log.Printf("delete learning path: %v", err)
		http.Error(writer, "Unable to delete learning path", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(writer, "Learning path not found", http.StatusNotFound)
		return
	}
	http.Redirect(writer, request, "/learn", http.StatusSeeOther)
}

func (server *Server) parseLearningPathMutation(writer http.ResponseWriter, request *http.Request) bool {
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

func parseLearningPathUnitIDs(values []string) []int64 {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func learningPathError(err error) string {
	switch {
	case errors.Is(err, services.ErrLearningPathNameRequired):
		return "A learning path name is required."
	case errors.Is(err, services.ErrLearningPathUnitsRequired):
		return "Select at least one target unit."
	case errors.Is(err, services.ErrUnitNotFound):
		return "A selected unit no longer exists."
	case errors.Is(err, services.ErrLearningPathNotFound):
		return "Learning path not found."
	default:
		return "Unable to save the learning path."
	}
}

func curriculumLayoutHints(request *http.Request) services.CurriculumGraphLayoutHints {
	hints := services.CurriculumGraphLayoutHints{
		Order: make(map[int64]int),
		Lanes: make(map[int64]float64),
	}
	for index, rawID := range strings.Split(request.URL.Query().Get("layout_order"), ",") {
		if index >= 200 || rawID == "" {
			break
		}
		if id, err := strconv.ParseInt(rawID, 10, 64); err == nil && id > 0 {
			hints.Order[id] = index
		}
	}
	for index, entry := range strings.Split(request.URL.Query().Get("layout_lanes"), ",") {
		if index >= 200 || entry == "" {
			break
		}
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			continue
		}
		id, idErr := strconv.ParseInt(parts[0], 10, 64)
		lane, laneErr := strconv.ParseFloat(parts[1], 64)
		if idErr == nil && laneErr == nil && id > 0 && lane >= 0 && lane < 200 {
			hints.Lanes[id] = lane
		}
	}
	return hints
}
