package db

import (
	"testing"

	"universal-curriculum/internal/models"
)

func TestApplyRecognitionsRequiresEverySourceAndFollowsChains(t *testing.T) {
	statuses := map[int64]models.UnitCompletionStatus{
		1: {Direct: true},
		2: {Direct: true},
	}
	recognitions := []models.Recognition{
		{
			Sources: []models.Unit{{ID: 1}, {ID: 2}},
			Targets: []models.Unit{{ID: 3}, {ID: 4}},
		},
		{
			Sources: []models.Unit{{ID: 3}},
			Targets: []models.Unit{{ID: 5}},
		},
		{
			Sources: []models.Unit{{ID: 1}, {ID: 9}},
			Targets: []models.Unit{{ID: 6}},
		},
	}

	applyRecognitions(statuses, recognitions)

	for _, unitID := range []int64{3, 4, 5} {
		status := statuses[unitID]
		if !status.Recognized || !status.Completed() {
			t.Errorf("unit %d was not recognized: %#v", unitID, status)
		}
	}
	if statuses[6].Completed() {
		t.Fatal("recognition was applied without every source")
	}
	if !statuses[1].Direct || statuses[1].Recognized {
		t.Fatalf("direct completion was rewritten: %#v", statuses[1])
	}
}
