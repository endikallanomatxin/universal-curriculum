package services

import (
	"errors"
	"slices"
	"testing"

	"universal-curriculum/internal/models"
)

func TestCurriculumProposalOverviewShowsAffectedUnitsAndImmediateContext(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Earlier"},
			{ID: 2, Name: "Prerequisite"},
			{ID: 3, Name: "Changed"},
			{ID: 4, Name: "Dependent"},
			{ID: 5, Name: "Later"},
			{ID: 6, Name: "Other changed"},
		},
		Dependencies: []models.UnitDependency{
			{PrerequisiteID: 1, UnitID: 2},
			{PrerequisiteID: 2, UnitID: 3},
			{PrerequisiteID: 3, UnitID: 4},
			{PrerequisiteID: 4, UnitID: 5},
		},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "update_content", UnitID: 3},
		{Kind: "rename_unit", UnitID: 6},
	}}

	visible, focus, boundaries, err := CurriculumProposalNeighborhood(graph, proposal, nil)
	if err != nil {
		t.Fatal(err)
	}
	if focus != nil {
		t.Fatalf("overview focus = %#v, want nil", focus)
	}
	assertCurriculumUnitIDs(t, visible, []int64{2, 3, 4, 6})
	if len(boundaries) != 2 {
		t.Fatalf("overview boundaries = %#v, want the earlier and later context", boundaries)
	}
}

func TestFocusedProposalGraphKeepsAffectedUnitsWithoutTheirNeighbours(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Focus"},
			{ID: 2, Name: "Focus dependent"},
			{ID: 3, Name: "Affected prerequisite"},
			{ID: 4, Name: "Affected"},
			{ID: 5, Name: "Affected dependent"},
		},
		Dependencies: []models.UnitDependency{
			{PrerequisiteID: 1, UnitID: 2},
			{PrerequisiteID: 3, UnitID: 4},
			{PrerequisiteID: 4, UnitID: 5},
		},
	}
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{{
		Kind: "update_content", UnitID: 4,
	}}}
	focusID := int64(1)

	visible, focus, _, err := CurriculumProposalNeighborhood(graph, proposal, &focusID)
	if err != nil {
		t.Fatal(err)
	}
	if focus == nil || focus.ID != focusID {
		t.Fatalf("focused unit = %#v", focus)
	}
	assertCurriculumUnitIDs(t, visible, []int64{1, 2, 4})
}

func TestProposalAffectedUnitsIncludeDependencyAndRecognitionMembers(t *testing.T) {
	prerequisiteID := int64(2)
	proposal := &models.CurriculumProposal{Changes: []models.CurriculumProposalChange{
		{Kind: "add_dependency", UnitID: 3, PrerequisiteID: &prerequisiteID},
		{Kind: "recognition", Recognition: &models.Recognition{
			Sources: []models.Unit{{ID: 4}, {ID: 5}},
			Targets: []models.Unit{{ID: 6}, {ID: 7}},
		}},
	}}

	ids := proposalAffectedUnitIDs(proposal)
	for _, id := range []int64{2, 3, 4, 5, 6, 7} {
		if !ids[id] {
			t.Errorf("unit %d is not marked as affected", id)
		}
	}
}

func assertCurriculumUnitIDs(t *testing.T, graph *models.CurriculumGraph, want []int64) {
	t.Helper()
	got := make([]int64, 0, len(graph.Units))
	for _, unit := range graph.Units {
		got = append(got, unit.ID)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("visible unit IDs = %v, want %v", got, want)
	}
}

func TestBuildCurriculumGraphLayoutOrdersDependenciesFirst(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 3, Name: "Calculus"},
			{ID: 1, Name: "Foundations"},
			{ID: 4, Name: "Probability"},
			{ID: 2, Name: "Algebra"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 2},
			{UnitID: 4, PrerequisiteID: 1},
		},
	}
	layout, err := BuildCurriculumGraphLayout(graph)
	if err != nil {
		t.Fatal(err)
	}
	positions := make(map[int64]int)
	for index, node := range layout.Nodes {
		positions[node.ID] = index
	}
	for _, edge := range layout.Edges {
		if positions[edge.PrerequisiteID] >= positions[edge.DependentID] {
			t.Fatalf("dependency is not before dependent: %#v", edge)
		}
	}
	if layout.LaneCount < 1 {
		t.Fatalf("expected graph lanes, got %d", layout.LaneCount)
	}
}

func TestBuildCurriculumGraphLayoutKeepsConnectedBranchesTogether(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Alpha root"},
			{ID: 2, Name: "Zebra child"},
			{ID: 3, Name: "Zebra grandchild"},
			{ID: 4, Name: "Beta unrelated root"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 2},
		},
	}

	layout, err := BuildCurriculumGraphLayout(graph)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int64, 0, len(layout.Nodes))
	for _, node := range layout.Nodes {
		got = append(got, node.ID)
	}
	want := []int64{1, 2, 3, 4}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("node order = %v, want %v", got, want)
		}
	}
}

func TestBuildCurriculumGraphLayoutUsesPreviousOrderAsAStartingPoint(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Root"},
			{ID: 2, Name: "Alpha child"},
			{ID: 3, Name: "Beta child"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 1},
		},
	}

	layout, err := BuildCurriculumGraphLayoutWithHints(graph, CurriculumGraphLayoutHints{
		Order: map[int64]int{1: 0, 3: 1, 2: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := []int64{layout.Nodes[0].ID, layout.Nodes[1].ID, layout.Nodes[2].ID}
	want := []int64{1, 3, 2}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("node order = %v, want %v", got, want)
		}
	}
}

func TestPreferredCurriculumOrderPreservesContinuityWithinQualityBand(t *testing.T) {
	nodes := []models.CurriculumGraphNode{{Unit: models.Unit{ID: 1}}}
	candidates := []curriculumOrderCandidate{
		{nodes: nodes, score: curriculumOrderScore{Crossings: 0, EdgeSpan: 10, Movement: 20}, key: "structural"},
		{nodes: nodes, score: curriculumOrderScore{Crossings: 0, EdgeSpan: 11, Movement: 2}, key: "continuous"},
		{nodes: nodes, score: curriculumOrderScore{Crossings: 1, EdgeSpan: 5, Movement: 0}, key: "crossing"},
	}

	preferred := preferredCurriculumOrder(candidates, 8)

	if preferred.key != "continuous" {
		t.Fatalf("preferred order = %q, want continuity inside the structural tolerance", preferred.key)
	}
}

func TestPreferredCurriculumOrderRejectsMovementOutsideQualityBand(t *testing.T) {
	nodes := []models.CurriculumGraphNode{{Unit: models.Unit{ID: 1}}}
	candidates := []curriculumOrderCandidate{
		{nodes: nodes, score: curriculumOrderScore{Crossings: 0, EdgeSpan: 10, Movement: 20}, key: "structural"},
		{nodes: nodes, score: curriculumOrderScore{Crossings: 0, EdgeSpan: 12, Movement: 0}, key: "continuous"},
	}

	preferred := preferredCurriculumOrder(candidates, 8)

	if preferred.key != "structural" {
		t.Fatalf("preferred order = %q, want the structurally better order", preferred.key)
	}
}

func TestPreferredCurriculumLanesRejectWiderContinuousLayout(t *testing.T) {
	fresh := &models.CurriculumGraphLayout{
		LaneCount: 2,
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}, Lane: 0},
			{Unit: models.Unit{ID: 2}, Lane: 1},
		},
	}
	continuous := cloneCurriculumGraphLayout(fresh)
	continuous.LaneCount = 3
	continuous.Nodes[0].Lane = 2
	previous := map[int64]float64{1: 2, 2: 1}

	preferred := preferredCurriculumLaneLayout(
		[]*models.CurriculumGraphLayout{fresh, continuous}, previous, nil,
	)

	if preferred != fresh {
		t.Fatal("wider continuous lane layout was preferred over the fresh layout")
	}
}

func TestPreferredCurriculumLanesKeepTheFocusedUnitStable(t *testing.T) {
	anchorMoved := &models.CurriculumGraphLayout{
		LaneCount: 2,
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}, Lane: 1},
			{Unit: models.Unit{ID: 2}, Lane: 0},
		},
	}
	otherMoved := &models.CurriculumGraphLayout{
		LaneCount: 2,
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}, Lane: 0},
			{Unit: models.Unit{ID: 2}, Lane: 1},
		},
	}
	previous := map[int64]float64{1: 0, 2: 0}

	preferred := preferredCurriculumLaneLayout(
		[]*models.CurriculumGraphLayout{anchorMoved, otherMoved},
		previous,
		map[int64]int{1: 4},
	)

	if preferred != otherMoved {
		t.Fatal("equivalent lane layout moved the focused unit instead of another unit")
	}
}

func TestCurriculumGraphMovementWeightsIncludeImmediateNeighbors(t *testing.T) {
	anchorID := int64(2)
	graph := &models.CurriculumGraphLayout{
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}},
			{Unit: models.Unit{ID: 2}},
			{Unit: models.Unit{ID: 3}},
			{Unit: models.Unit{ID: 4}},
		},
		Edges: []models.CurriculumGraphEdge{
			{PrerequisiteID: 1, DependentID: 2},
			{PrerequisiteID: 2, DependentID: 3},
		},
	}

	weights := curriculumGraphMovementWeights(graph, &anchorID)

	if weights[2] != 4 || weights[1] != 2 || weights[3] != 2 || curriculumMovementWeight(4, weights) != 1 {
		t.Fatalf("movement weights = %#v, want anchor 4, neighbors 2 and other units 1", weights)
	}
}

func TestStabilizeCurriculumNodeLanesMovesOnlyToClearPreviousLanes(t *testing.T) {
	layout := &models.CurriculumGraphLayout{
		LaneCount: 3,
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}, Lane: 0},
			{Unit: models.Unit{ID: 2}, Lane: 0},
			{Unit: models.Unit{ID: 3}, Lane: 2},
			{Unit: models.Unit{ID: 4}, Lane: 2},
		},
		Edges: []models.CurriculumGraphEdge{{PrerequisiteID: 1, DependentID: 4, Lane: 1}},
	}

	stabilizeCurriculumNodeLanes(layout, map[int64]float64{2: 1, 3: 0})

	if layout.Nodes[1].Lane == 1 {
		t.Fatal("persistent node moved onto a dependency crossing its row")
	}
	if layout.Nodes[2].Lane != 0 {
		t.Fatalf("clear persistent node lane = %g, want previous lane 0", layout.Nodes[2].Lane)
	}
}

func TestBuildCurriculumGraphLayoutImprovesCrossingPreviousOrder(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "First root"},
			{ID: 2, Name: "Second root"},
			{ID: 3, Name: "First result"},
			{ID: 4, Name: "Second result"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 3, PrerequisiteID: 1},
			{UnitID: 4, PrerequisiteID: 2},
		},
	}
	hints := CurriculumGraphLayoutHints{
		Order: map[int64]int{1: 0, 2: 1, 3: 2, 4: 3},
	}

	layout, err := BuildCurriculumGraphLayoutWithHints(graph, hints)
	if err != nil {
		t.Fatal(err)
	}
	score := scoreCurriculumNodeOrder(layout, hints.Order)
	if score.Crossings != 0 {
		t.Fatalf("avoidable crossings = %d, layout = %#v", score.Crossings, layout.Nodes)
	}
	positions := make(map[int64]int, len(layout.Nodes))
	for index, node := range layout.Nodes {
		positions[node.ID] = index
	}
	for _, edge := range layout.Edges {
		if positions[edge.PrerequisiteID] >= positions[edge.DependentID] {
			t.Fatalf("optimized order violated dependency %#v", edge)
		}
	}
}

func TestBuildCurriculumGraphLayoutOptimizesDenseNeighborhood(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Mathematical reasoning"},
			{ID: 2, Name: "Algebraic foundations"},
			{ID: 3, Name: "Propositional logic"},
			{ID: 4, Name: "Functions and relations"},
			{ID: 5, Name: "Discrete structures"},
			{ID: 13, Name: "Bash"},
			{ID: 15, Name: "Operating systems"},
			{ID: 27, Name: "Continuous delivery"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 13, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 1},
			{UnitID: 5, PrerequisiteID: 2},
			{UnitID: 4, PrerequisiteID: 2},
			{UnitID: 27, PrerequisiteID: 13},
			{UnitID: 15, PrerequisiteID: 13},
			{UnitID: 5, PrerequisiteID: 13},
			{UnitID: 5, PrerequisiteID: 3},
			{UnitID: 4, PrerequisiteID: 3},
		},
	}

	hints := CurriculumGraphLayoutHints{
		Order: map[int64]int{1: 0, 2: 1, 13: 2, 27: 3, 15: 4, 3: 5, 5: 6, 4: 7},
	}
	layout, err := BuildCurriculumGraphLayoutWithHints(graph, hints)
	if err != nil {
		t.Fatal(err)
	}
	score := scoreCurriculumNodeOrder(layout, hints.Order)
	if score.Crossings > 5 || score.EdgeSpan > 27 {
		t.Fatalf("dense layout score = %#v, nodes = %#v", score, layout.Nodes)
	}
	assertNoCurriculumNodeLaneCollisions(t, layout)

	repeated, err := BuildCurriculumGraphLayoutWithHints(graph, hints)
	if err != nil {
		t.Fatal(err)
	}
	for index := range layout.Nodes {
		if layout.Nodes[index].ID != repeated.Nodes[index].ID ||
			layout.Nodes[index].Lane != repeated.Nodes[index].Lane {
			t.Fatalf("layout is not deterministic: first=%#v repeated=%#v", layout.Nodes, repeated.Nodes)
		}
	}
}

func TestBuildCurriculumGraphLayoutRejectsCycles(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{{ID: 1, Name: "One"}, {ID: 2, Name: "Two"}},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 1, PrerequisiteID: 2},
		},
	}
	if _, err := BuildCurriculumGraphLayout(graph); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle returned %v", err)
	}
}

func TestAssignCurriculumNodeLanesAvoidsEdgesCrossingTheNodeRow(t *testing.T) {
	layout := &models.CurriculumGraphLayout{
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1}, Lane: 0},
			{Unit: models.Unit{ID: 2}, Lane: 1},
			{Unit: models.Unit{ID: 3}},
			{Unit: models.Unit{ID: 4}},
			{Unit: models.Unit{ID: 5}},
		},
		Edges: []models.CurriculumGraphEdge{
			{PrerequisiteID: 1, DependentID: 5, Lane: 1},
			{PrerequisiteID: 2, DependentID: 3, Lane: 0},
		},
	}
	nodeIndexes := map[int64]int{1: 0, 2: 1, 3: 2, 4: 3, 5: 4}

	assignCurriculumNodeLanes(layout, nodeIndexes)

	if layout.Nodes[2].Lane == 1 {
		t.Fatal("dependent unit was placed on a lane crossing its row")
	}
}

func TestBuildCurriculumGraphLayoutKeepsUnitsOffRenderedDependencyLanes(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 5, Name: "Foundations of Logic"},
			{ID: 6, Name: "Set Theory"},
			{ID: 7, Name: "Functions and Relations"},
			{ID: 8, Name: "Linear Algebra"},
			{ID: 9, Name: "Limits"},
			{ID: 10, Name: "Differential Calculus"},
			{ID: 11, Name: "Probability Basics"},
			{ID: 12, Name: "Introduction to Programming"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 12, PrerequisiteID: 5},
			{UnitID: 6, PrerequisiteID: 5},
			{UnitID: 7, PrerequisiteID: 6},
			{UnitID: 11, PrerequisiteID: 6},
			{UnitID: 9, PrerequisiteID: 7},
			{UnitID: 8, PrerequisiteID: 7},
			{UnitID: 10, PrerequisiteID: 9},
		},
	}

	layout, err := BuildCurriculumGraphLayout(graph)
	if err != nil {
		t.Fatal(err)
	}
	assertNoCurriculumNodeLaneCollisions(t, layout)
	lanes := make(map[int64]float64, len(layout.Nodes))
	for _, node := range layout.Nodes {
		lanes[node.ID] = node.Lane
	}
	if lanes[9] != lanes[10] {
		t.Fatalf("straight descendants were split across lanes: Limits=%g Differential Calculus=%g", lanes[9], lanes[10])
	}
}

func assertNoCurriculumNodeLaneCollisions(t *testing.T, layout *models.CurriculumGraphLayout) {
	t.Helper()
	indexes := make(map[int64]int, len(layout.Nodes))
	for index, node := range layout.Nodes {
		indexes[node.ID] = index
	}
	for nodeIndex, node := range layout.Nodes {
		for _, edge := range layout.Edges {
			start, end := indexes[edge.PrerequisiteID], indexes[edge.DependentID]
			if start >= nodeIndex || nodeIndex >= end {
				continue
			}
			lane := edge.Lane
			if layout.Nodes[start].Lane == layout.Nodes[end].Lane {
				lane = layout.Nodes[start].Lane
			}
			if node.Lane == lane {
				t.Fatalf("unit %d occupies lane %g used by dependency %d -> %d", node.ID, lane, edge.PrerequisiteID, edge.DependentID)
			}
		}
	}
}

func TestCurriculumNeighborhood(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations"},
			{ID: 2, Name: "Algebra"},
			{ID: 3, Name: "Calculus"},
			{ID: 4, Name: "Geometry"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 2},
		},
	}

	entries, focus, boundaries, err := CurriculumNeighborhood(graph, nil)
	if err != nil {
		t.Fatal(err)
	}
	if focus != nil || len(entries.Units) != 2 || entries.Units[0].ID != 1 || entries.Units[1].ID != 4 {
		t.Fatalf("unexpected entry points: focus=%#v graph=%#v", focus, entries)
	}
	if len(boundaries) != 1 || boundaries[0].UnitID != 1 || boundaries[0].Direction != "dependents" {
		t.Fatalf("unexpected entry boundaries: %#v", boundaries)
	}

	algebraID := int64(2)
	neighborhood, focus, boundaries, err := CurriculumNeighborhood(graph, &algebraID)
	if err != nil {
		t.Fatal(err)
	}
	if focus == nil || focus.ID != algebraID {
		t.Fatalf("focus = %#v", focus)
	}
	if len(neighborhood.Units) != 3 || len(neighborhood.Dependencies) != 2 {
		t.Fatalf("unexpected algebra neighborhood: %#v", neighborhood)
	}
	if len(boundaries) != 0 {
		t.Fatalf("unexpected algebra boundaries: %#v", boundaries)
	}

	missingID := int64(99)
	if _, _, _, err := CurriculumNeighborhood(graph, &missingID); !errors.Is(err, ErrCurriculumUnitNotFound) {
		t.Fatalf("missing unit returned %v", err)
	}
}

func TestCurriculumNeighborhoodTruncatesWideBranches(t *testing.T) {
	graph := &models.CurriculumGraph{Units: []models.Unit{{ID: 1, Name: "Focus"}}}
	for id := int64(2); id <= 8; id++ {
		graph.Units = append(graph.Units, models.Unit{ID: id, Name: "Dependent"})
		graph.Dependencies = append(graph.Dependencies, models.UnitDependency{
			UnitID: id, PrerequisiteID: 1,
		})
	}

	focusID := int64(1)
	neighborhood, _, boundaries, err := CurriculumNeighborhood(graph, &focusID)
	if err != nil {
		t.Fatal(err)
	}
	if len(neighborhood.Units) != 1+curriculumDirectNeighborLimit {
		t.Fatalf("visible units = %d", len(neighborhood.Units))
	}
	if len(boundaries) != 1 || boundaries[0].UnitID != focusID ||
		boundaries[0].Direction != "dependents" || boundaries[0].Count != 3 {
		t.Fatalf("unexpected truncation boundary: %#v", boundaries)
	}
}

func TestCurriculumNeighborhoodIncludesForwardCoPrerequisitesButNotUpstreamSiblings(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Shared foundation"},
			{ID: 2, Name: "Focus"},
			{ID: 3, Name: "Upstream sibling"},
			{ID: 4, Name: "Other requirement"},
			{ID: 5, Name: "Immediate next unit"},
		},
		Dependencies: []models.UnitDependency{
			{UnitID: 2, PrerequisiteID: 1},
			{UnitID: 3, PrerequisiteID: 1},
			{UnitID: 5, PrerequisiteID: 2},
			{UnitID: 5, PrerequisiteID: 4},
		},
	}

	focusID := int64(2)
	neighborhood, _, _, err := CurriculumNeighborhood(graph, &focusID)
	if err != nil {
		t.Fatal(err)
	}
	visible := make(map[int64]bool)
	for _, unit := range neighborhood.Units {
		visible[unit.ID] = true
	}
	for _, expected := range []int64{1, 2, 4, 5} {
		if !visible[expected] {
			t.Fatalf("expected unit %d in neighborhood: %#v", expected, neighborhood.Units)
		}
	}
	if visible[3] {
		t.Fatalf("upstream sibling leaked into neighborhood: %#v", neighborhood.Units)
	}
}

func TestCurriculumNeighborhoodUsesOneUpstreamAndTwoDownstreamLevels(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Third upstream"},
			{ID: 2, Name: "Second upstream"},
			{ID: 3, Name: "First upstream"},
			{ID: 4, Name: "Focus"},
			{ID: 5, Name: "First downstream"},
			{ID: 6, Name: "Second downstream"},
			{ID: 7, Name: "Third downstream"},
			{ID: 8, Name: "Fourth downstream"},
		},
	}
	for id := int64(2); id <= 8; id++ {
		graph.Dependencies = append(graph.Dependencies, models.UnitDependency{
			UnitID: id, PrerequisiteID: id - 1,
		})
	}

	focusID := int64(4)
	neighborhood, _, boundaries, err := CurriculumNeighborhood(graph, &focusID)
	if err != nil {
		t.Fatal(err)
	}
	visible := make(map[int64]bool)
	for _, unit := range neighborhood.Units {
		visible[unit.ID] = true
	}
	for _, expected := range []int64{3, 4, 5, 6} {
		if !visible[expected] {
			t.Fatalf("expected unit %d in neighborhood: %#v", expected, neighborhood.Units)
		}
	}
	if visible[1] || visible[2] || visible[7] || visible[8] {
		t.Fatalf("neighborhood exceeded its directional horizon: %#v", neighborhood.Units)
	}
	if len(boundaries) != 2 {
		t.Fatalf("expected boundaries at both horizons, got %#v", boundaries)
	}
}
