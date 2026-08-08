package server

import (
	"html/template"
	"net/http"
	"strconv"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/guidance"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func (server *Server) about(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "about.html", "about", false)
}

func (server *Server) aboutCase(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "case.html", "about", false)
}

func (server *Server) aboutProposal(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "proposal.html", "about", false)
}

type documentationPageData struct {
	userPageData
	Pages    []guidance.Page
	Page     *guidance.Page
	Rendered template.HTML
	Title    string
}

func (server *Server) documentation(writer http.ResponseWriter, request *http.Request) {
	data, err := server.loadUserPageData(request, "about", false)
	if err != nil {
		http.Error(writer, "Load user", http.StatusInternalServerError)
		return
	}
	view := documentationPageData{userPageData: data, Pages: guidance.Pages()}
	if slug := request.PathValue("slug"); slug != "" {
		page, ok := guidance.Find(slug)
		if !ok {
			http.NotFound(writer, request)
			return
		}
		view.Page = &page
		view.Rendered = services.RenderUnitContent(page.Content)
		view.Title = page.Title + " · Universal Curriculum"
		server.render(writer, "documentation-page.html", view)
		return
	}
	server.render(writer, "documentation.html", view)
}

func (server *Server) license(writer http.ResponseWriter, request *http.Request) {
	server.renderUserPage(writer, request, "license.html", "about", false)
}

type homeLearningUnitRecommendation struct {
	models.Unit
	URL string
}

type homeLearningPathRecommendation struct {
	ID           int64
	Name         string
	URL          string
	PendingCount int
	NextUnits    []homeLearningUnitRecommendation
}

func (server *Server) homeLearningRecommendations(userID int64) ([]homeLearningPathRecommendation, error) {
	paths, err := db.ListLearningPaths(server.Database, userID)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		return nil, err
	}
	completedUnitIDs, err := db.CompletedUnitIDs(server.Database, userID)
	if err != nil {
		return nil, err
	}
	return newHomeLearningRecommendations(paths, graph, completedUnitIDs), nil
}

func newHomeLearningRecommendations(
	paths []models.LearningPath,
	graph *models.CurriculumGraph,
	completedUnitIDs map[int64]bool,
) []homeLearningPathRecommendation {
	recommendations := make([]homeLearningPathRecommendation, 0, len(paths))
	for _, path := range paths {
		targetIDs := make([]int64, 0, len(path.Units))
		for _, unit := range path.Units {
			targetIDs = append(targetIDs, unit.ID)
		}
		nextUnits, pendingCount := services.AvailableLearningPathUnits(graph, targetIDs, completedUnitIDs)
		if pendingCount == 0 || len(nextUnits) == 0 {
			continue
		}
		pathID := strconv.FormatInt(path.ID, 10)
		recommendation := homeLearningPathRecommendation{
			ID:           path.ID,
			Name:         path.Name,
			URL:          "/learn?path=" + pathID,
			PendingCount: pendingCount,
			NextUnits:    make([]homeLearningUnitRecommendation, 0, len(nextUnits)),
		}
		for _, unit := range nextUnits {
			unitID := strconv.FormatInt(unit.ID, 10)
			recommendation.NextUnits = append(recommendation.NextUnits, homeLearningUnitRecommendation{
				Unit: unit,
				URL:  "/learn?path=" + pathID + "&unit=" + unitID + "&content=" + unitID,
			})
		}
		recommendations = append(recommendations, recommendation)
	}
	return recommendations
}
