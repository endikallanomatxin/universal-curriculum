package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"

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
	graph, err := services.BuildCurriculumGraphLayout(neighborhood)
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
