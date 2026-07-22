package server

import (
	"log"
	"net/http"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type learnPageData struct {
	userPageData
	Graph *models.CurriculumGraphLayout
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
	graph, err := services.BuildCurriculumGraphLayout(curriculum)
	if err != nil {
		log.Printf("lay out learn curriculum: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	server.render(writer, "learn.html", learnPageData{userPageData: page, Graph: graph})
}
