package models

import "time"

type LearningPath struct {
	ID        int64
	UserID    int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Units     []Unit
	UnitCount int
}

type LearningRecommendation struct {
	LearningPathID   int64
	LearningPathName string
	Units            []RecommendedUnit
}

type RecommendedUnit struct {
	Unit
	Reason string
}
