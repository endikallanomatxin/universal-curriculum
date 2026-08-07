package services

import (
	"database/sql"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

// LearningRecommendations applies the platform's curriculum and recorded
// progress rules for every learning path owned by the user.
func LearningRecommendations(database *sql.DB, userID int64) ([]models.LearningRecommendation, error) {
	paths, err := db.ListLearningPaths(database, userID)
	if err != nil {
		return nil, err
	}
	graph, err := db.GetCurriculumGraph(database)
	if err != nil {
		return nil, err
	}
	completed, err := db.CompletedUnitIDs(database, userID)
	if err != nil {
		return nil, err
	}
	recommendations := make([]models.LearningRecommendation, 0, len(paths))
	for _, path := range paths {
		targetIDs := make([]int64, 0, len(path.Units))
		for _, unit := range path.Units {
			targetIDs = append(targetIDs, unit.ID)
		}
		available, pending := AvailableLearningPathUnits(graph, targetIDs, completed)
		if pending == 0 || len(available) == 0 {
			continue
		}
		recommendation := models.LearningRecommendation{
			LearningPathID: path.ID, LearningPathName: path.Name,
			Units: make([]models.RecommendedUnit, 0, len(available)),
		}
		for _, unit := range available {
			recommendation.Units = append(recommendation.Units, models.RecommendedUnit{
				Unit: unit, Reason: "all prerequisites are completed",
			})
		}
		recommendations = append(recommendations, recommendation)
	}
	return recommendations, nil
}

// SetUnitProgress records or removes a direct completion and returns the
// resulting authoritative status, including recognition-derived completion.
func SetUnitProgress(
	database *sql.DB, userID, unitID int64, completed bool,
) (models.UnitCompletionStatus, error) {
	unit, err := db.GetUnit(database, unitID)
	if err != nil {
		return models.UnitCompletionStatus{}, err
	}
	if unit == nil {
		return models.UnitCompletionStatus{}, ErrUnitNotFound
	}
	if err := db.SetUnitCompleted(database, userID, unitID, completed); err != nil {
		return models.UnitCompletionStatus{}, err
	}
	statuses, err := db.UnitCompletionStatuses(database, userID)
	if err != nil {
		return models.UnitCompletionStatus{}, err
	}
	return statuses[unitID], nil
}
