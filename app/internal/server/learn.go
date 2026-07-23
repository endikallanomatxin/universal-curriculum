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
	var focusID *int64
	if rawID := request.URL.Query().Get("unit"); rawID != "" {
		id, parseErr := strconv.ParseInt(rawID, 10, 64)
		if parseErr != nil || id <= 0 {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		focusID = &id
	}
	neighborhood, focusedUnit, boundaries, err := services.CurriculumNeighborhood(curriculum, focusID)
	if errors.Is(err, services.ErrCurriculumUnitNotFound) {
		http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("build curriculum neighborhood: %v", err)
		http.Error(writer, "Unable to navigate curriculum", http.StatusInternalServerError)
		return
	}
	graph, err := services.BuildCurriculumGraphLayoutWithHints(neighborhood, curriculumLayoutHints(request))
	if err != nil {
		log.Printf("lay out learn curriculum: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	graph.Boundaries = boundaries
	server.render(writer, "learn.html", learnPageData{
		userPageData:      page,
		Graph:             graph,
		FocusedUnit:       focusedUnit,
		TotalUnits:        len(curriculum.Units),
		TotalDependencies: len(curriculum.Dependencies),
	})
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
