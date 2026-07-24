package server

import (
	"strconv"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

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
