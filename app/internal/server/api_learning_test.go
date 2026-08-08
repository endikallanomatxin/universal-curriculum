package server

import "testing"

func TestValidateAPIIDsRejectsMalformedCollections(t *testing.T) {
	for _, ids := range [][]int64{nil, {}, {0}, {-1}, {1, 1}} {
		if err := validateAPIIDs(ids, "unit_ids"); err == nil {
			t.Fatalf("validateAPIIDs(%v) accepted invalid IDs", ids)
		}
	}
	if err := validateAPIIDs([]int64{3, 1, 2}, "unit_ids"); err != nil {
		t.Fatalf("validateAPIIDs(valid) = %v", err)
	}
}

func TestAPIProgressIsSortedByUnitID(t *testing.T) {
	progress := []apiProgress{{UnitID: 9}, {UnitID: 2}, {UnitID: 5}}
	sortAPIProgress(progress)
	if progress[0].UnitID != 2 || progress[1].UnitID != 5 || progress[2].UnitID != 9 {
		t.Fatalf("sorted progress = %#v", progress)
	}
}
