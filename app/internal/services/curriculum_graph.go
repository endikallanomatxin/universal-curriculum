package services

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"universal-curriculum/internal/models"
)

var ErrCurriculumUnitNotFound = errors.New("curriculum unit not found")

const (
	curriculumDirectNeighborLimit = 4
	curriculumSecondNeighborLimit = 4
	curriculumCoPrerequisiteLimit = 3
	curriculumOrderSearchLimit    = 512
)

type CurriculumGraphLayoutHints struct {
	Order map[int64]int
	Lanes map[int64]float64
}

func CurriculumNeighborhood(graph *models.CurriculumGraph, focusID *int64) (*models.CurriculumGraph, *models.Unit, []models.CurriculumGraphBoundary, error) {
	neighborhood := &models.CurriculumGraph{}
	if graph == nil {
		return neighborhood, nil, nil, nil
	}
	unitsByID := make(map[int64]models.Unit, len(graph.Units))
	incoming := make(map[int64]int, len(graph.Units))
	prerequisites := make(map[int64][]int64)
	dependents := make(map[int64][]int64)
	for _, unit := range graph.Units {
		unitsByID[unit.ID] = unit
	}
	for _, dependency := range graph.Dependencies {
		incoming[dependency.UnitID]++
		prerequisites[dependency.UnitID] = append(prerequisites[dependency.UnitID], dependency.PrerequisiteID)
		dependents[dependency.PrerequisiteID] = append(dependents[dependency.PrerequisiteID], dependency.UnitID)
	}
	included := make(map[int64]bool)
	var focus *models.Unit
	if focusID == nil {
		for _, unit := range graph.Units {
			if incoming[unit.ID] == 0 {
				included[unit.ID] = true
			}
		}
	} else {
		unit, exists := unitsByID[*focusID]
		if !exists {
			return nil, nil, nil, ErrCurriculumUnitNotFound
		}
		focus = &unit
		included[unit.ID] = true
		includeCurriculumNeighbors(included, prerequisites[unit.ID], curriculumDirectNeighborLimit)
		directDependents := includeCurriculumNeighbors(included, dependents[unit.ID], curriculumDirectNeighborLimit)
		includeSecondCurriculumNeighbors(included, directDependents, dependents, curriculumSecondNeighborLimit)

		coPrerequisites := make(map[int64]bool)
		for _, dependentID := range directDependents {
			for _, prerequisiteID := range prerequisites[dependentID] {
				if prerequisiteID != unit.ID && !included[prerequisiteID] {
					coPrerequisites[prerequisiteID] = true
				}
			}
		}
		if len(coPrerequisites) <= curriculumCoPrerequisiteLimit {
			for _, candidate := range graph.Units {
				if coPrerequisites[candidate.ID] {
					included[candidate.ID] = true
				}
			}
		}
	}
	for _, unit := range graph.Units {
		if included[unit.ID] {
			neighborhood.Units = append(neighborhood.Units, unit)
		}
	}
	for _, dependency := range graph.Dependencies {
		if included[dependency.UnitID] && included[dependency.PrerequisiteID] {
			neighborhood.Dependencies = append(neighborhood.Dependencies, dependency)
		}
	}
	return neighborhood, focus, CurriculumGraphBoundaries(graph, neighborhood), nil
}

// CurriculumProposalNeighborhood keeps a proposal's affected units visible as
// stable context. Its overview adds their immediate graph neighbours; once a
// unit is focused, only that unit contributes its navigable neighbourhood.
func CurriculumProposalNeighborhood(
	graph *models.CurriculumGraph,
	proposal *models.CurriculumProposal,
	focusID *int64,
) (*models.CurriculumGraph, *models.Unit, []models.CurriculumGraphBoundary, error) {
	if proposal == nil {
		return CurriculumNeighborhood(graph, focusID)
	}
	if graph == nil {
		return &models.CurriculumGraph{}, nil, nil, nil
	}

	included := proposalAffectedUnitIDs(proposal)
	if len(included) == 0 {
		return CurriculumNeighborhood(graph, focusID)
	}
	var focus *models.Unit
	if focusID != nil {
		neighborhood, focusedUnit, _, err := CurriculumNeighborhood(graph, focusID)
		if err != nil {
			return nil, nil, nil, err
		}
		focus = focusedUnit
		for _, unit := range neighborhood.Units {
			included[unit.ID] = true
		}
	} else {
		affected := make(map[int64]bool, len(included))
		for unitID := range included {
			affected[unitID] = true
		}
		for _, dependency := range graph.Dependencies {
			if affected[dependency.UnitID] || affected[dependency.PrerequisiteID] {
				included[dependency.UnitID] = true
				included[dependency.PrerequisiteID] = true
			}
		}
	}

	visible := curriculumSubgraph(graph, included)
	return visible, focus, CurriculumGraphBoundaries(graph, visible), nil
}

func proposalAffectedUnitIDs(proposal *models.CurriculumProposal) map[int64]bool {
	ids := make(map[int64]bool)
	if proposal == nil {
		return ids
	}
	for _, change := range proposal.Changes {
		if change.UnitID > 0 {
			ids[change.UnitID] = true
		}
		if change.PrerequisiteID != nil && *change.PrerequisiteID > 0 {
			ids[*change.PrerequisiteID] = true
		}
		if change.Recognition != nil {
			for _, unit := range change.Recognition.Sources {
				ids[unit.ID] = true
			}
			for _, unit := range change.Recognition.Targets {
				ids[unit.ID] = true
			}
		}
	}
	return ids
}

func curriculumSubgraph(graph *models.CurriculumGraph, included map[int64]bool) *models.CurriculumGraph {
	visible := &models.CurriculumGraph{}
	for _, unit := range graph.Units {
		if included[unit.ID] {
			visible.Units = append(visible.Units, unit)
		}
	}
	for _, dependency := range graph.Dependencies {
		if included[dependency.UnitID] && included[dependency.PrerequisiteID] {
			visible.Dependencies = append(visible.Dependencies, dependency)
		}
	}
	return visible
}

func CurriculumGraphBoundaries(
	graph *models.CurriculumGraph,
	visible *models.CurriculumGraph,
) []models.CurriculumGraphBoundary {
	if graph == nil || visible == nil {
		return nil
	}
	visibleIDs := make(map[int64]bool, len(visible.Units))
	for _, unit := range visible.Units {
		visibleIDs[unit.ID] = true
	}
	prerequisites := make(map[int64][]int64)
	dependents := make(map[int64][]int64)
	for _, dependency := range graph.Dependencies {
		prerequisites[dependency.UnitID] = append(prerequisites[dependency.UnitID], dependency.PrerequisiteID)
		dependents[dependency.PrerequisiteID] = append(dependents[dependency.PrerequisiteID], dependency.UnitID)
	}
	var boundaries []models.CurriculumGraphBoundary
	for _, unit := range visible.Units {
		hiddenPrerequisites := countHiddenCurriculumNeighbors(prerequisites[unit.ID], visibleIDs)
		if hiddenPrerequisites > 0 {
			boundaries = append(boundaries, models.CurriculumGraphBoundary{
				UnitID: unit.ID, Direction: "prerequisites", Count: hiddenPrerequisites,
			})
		}
		hiddenDependents := countHiddenCurriculumNeighbors(dependents[unit.ID], visibleIDs)
		if hiddenDependents > 0 {
			boundaries = append(boundaries, models.CurriculumGraphBoundary{
				UnitID: unit.ID, Direction: "dependents", Count: hiddenDependents,
			})
		}
	}
	return boundaries
}

func includeCurriculumNeighbors(included map[int64]bool, candidates []int64, limit int) []int64 {
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	selected := append([]int64(nil), candidates...)
	for _, id := range selected {
		included[id] = true
	}
	return selected
}

func includeSecondCurriculumNeighbors(included map[int64]bool, first []int64, adjacency map[int64][]int64, limit int) []int64 {
	selected := make([]int64, 0, limit)
	added := 0
	for _, firstID := range first {
		for _, candidateID := range adjacency[firstID] {
			if included[candidateID] {
				continue
			}
			included[candidateID] = true
			selected = append(selected, candidateID)
			added++
			if added == limit {
				return selected
			}
		}
	}
	return selected
}

func countHiddenCurriculumNeighbors(candidates []int64, included map[int64]bool) int {
	count := 0
	for _, candidateID := range candidates {
		if !included[candidateID] {
			count++
		}
	}
	return count
}

func BuildCurriculumGraphLayout(graph *models.CurriculumGraph) (*models.CurriculumGraphLayout, error) {
	return BuildCurriculumGraphLayoutWithHints(graph, CurriculumGraphLayoutHints{})
}

func cloneCurriculumGraphLayout(layout *models.CurriculumGraphLayout) *models.CurriculumGraphLayout {
	if layout == nil {
		return &models.CurriculumGraphLayout{}
	}
	clone := *layout
	clone.Nodes = append([]models.CurriculumGraphNode(nil), layout.Nodes...)
	clone.Edges = append([]models.CurriculumGraphEdge(nil), layout.Edges...)
	clone.Boundaries = append([]models.CurriculumGraphBoundary(nil), layout.Boundaries...)
	return &clone
}

func BuildCurriculumGraphLayoutWithHints(graph *models.CurriculumGraph, hints CurriculumGraphLayoutHints) (*models.CurriculumGraphLayout, error) {
	layout := &models.CurriculumGraphLayout{}
	if graph == nil {
		return layout, nil
	}
	for _, unit := range graph.Units {
		layout.Nodes = append(layout.Nodes, models.CurriculumGraphNode{Unit: unit})
	}
	for _, dependency := range graph.Dependencies {
		layout.Edges = append(layout.Edges, models.CurriculumGraphEdge{
			PrerequisiteID: dependency.PrerequisiteID,
			DependentID:    dependency.UnitID,
		})
	}
	canonical := cloneCurriculumGraphLayout(layout)
	if err := topologicallyOrderCurriculum(canonical, nil); err != nil {
		return nil, err
	}
	if err := topologicallyOrderCurriculum(layout, hints.Order); err != nil {
		return nil, err
	}
	improveCurriculumNodeOrder(layout, hints.Order, canonical.Nodes)
	assignCurriculumGraphLanes(layout, hints.Lanes)
	return layout, nil
}

type curriculumOrderScore struct {
	Crossings int
	EdgeSpan  int
	Movement  int
}

type curriculumOrderCandidate struct {
	nodes []models.CurriculumGraphNode
	score curriculumOrderScore
	key   string
}

func improveCurriculumNodeOrder(
	graph *models.CurriculumGraphLayout,
	previousOrder map[int64]int,
	additionalSeeds ...[]models.CurriculumGraphNode,
) {
	if graph == nil || len(graph.Nodes) < 2 || len(graph.Edges) == 0 {
		return
	}
	directDependencies := make(map[[2]int64]bool, len(graph.Edges))
	for _, edge := range graph.Edges {
		directDependencies[[2]int64{edge.PrerequisiteID, edge.DependentID}] = true
	}
	pending := make([]curriculumOrderCandidate, 0, 1+len(additionalSeeds))
	seen := make(map[string]bool)
	addSeed := func(nodes []models.CurriculumGraphNode) {
		nodes = append([]models.CurriculumGraphNode(nil), nodes...)
		key := curriculumNodeOrderKey(nodes)
		if seen[key] {
			return
		}
		seen[key] = true
		pending = append(pending, curriculumOrderCandidate{
			nodes: nodes,
			score: scoreCurriculumNodes(nodes, graph.Edges, previousOrder),
			key:   key,
		})
	}
	addSeed(graph.Nodes)
	for _, seed := range additionalSeeds {
		addSeed(seed)
	}
	exploredCandidates := make([]curriculumOrderCandidate, 0, curriculumOrderSearchLimit)
	sortCandidates := func() {
		sort.SliceStable(pending, func(i, j int) bool {
			if pending[i].score != pending[j].score {
				return pending[i].score.betterThan(pending[j].score)
			}
			return pending[i].key < pending[j].key
		})
	}
	for explored := 0; explored < curriculumOrderSearchLimit && len(pending) > 0; explored++ {
		sortCandidates()
		current := pending[0]
		pending = pending[1:]
		exploredCandidates = append(exploredCandidates, current)
		for index := 0; index+1 < len(current.nodes); index++ {
			left, right := current.nodes[index].ID, current.nodes[index+1].ID
			if directDependencies[[2]int64{left, right}] {
				continue
			}
			nodes := append([]models.CurriculumGraphNode(nil), current.nodes...)
			nodes[index], nodes[index+1] = nodes[index+1], nodes[index]
			key := curriculumNodeOrderKey(nodes)
			if seen[key] {
				continue
			}
			seen[key] = true
			pending = append(pending, curriculumOrderCandidate{
				nodes: nodes,
				score: scoreCurriculumNodes(nodes, graph.Edges, previousOrder),
				key:   key,
			})
		}
		if len(pending) > curriculumOrderSearchLimit {
			sortCandidates()
			pending = pending[:curriculumOrderSearchLimit]
		}
	}
	graph.Nodes = preferredCurriculumOrder(exploredCandidates, len(graph.Edges)).nodes
}

func preferredCurriculumOrder(candidates []curriculumOrderCandidate, edgeCount int) curriculumOrderCandidate {
	bestCrossings := candidates[0].score.Crossings
	bestSpan := candidates[0].score.EdgeSpan
	for _, candidate := range candidates[1:] {
		if candidate.score.Crossings < bestCrossings {
			bestCrossings = candidate.score.Crossings
			bestSpan = candidate.score.EdgeSpan
		} else if candidate.score.Crossings == bestCrossings && candidate.score.EdgeSpan < bestSpan {
			bestSpan = candidate.score.EdgeSpan
		}
	}
	spanTolerance := max(1, edgeCount/8)
	best := candidates[0]
	found := false
	for _, candidate := range candidates {
		if candidate.score.Crossings != bestCrossings || candidate.score.EdgeSpan > bestSpan+spanTolerance {
			continue
		}
		if !found || candidate.score.Movement < best.score.Movement ||
			candidate.score.Movement == best.score.Movement && candidate.score.EdgeSpan < best.score.EdgeSpan ||
			candidate.score.Movement == best.score.Movement && candidate.score.EdgeSpan == best.score.EdgeSpan && candidate.key < best.key {
			best = candidate
			found = true
		}
	}
	return best
}

func scoreCurriculumNodeOrder(graph *models.CurriculumGraphLayout, previousOrder map[int64]int) curriculumOrderScore {
	if graph == nil {
		return curriculumOrderScore{}
	}
	return scoreCurriculumNodes(graph.Nodes, graph.Edges, previousOrder)
}

func scoreCurriculumNodes(
	nodes []models.CurriculumGraphNode,
	edges []models.CurriculumGraphEdge,
	previousOrder map[int64]int,
) curriculumOrderScore {
	score := curriculumOrderScore{}
	positions := make(map[int64]int, len(nodes))
	for index, node := range nodes {
		positions[node.ID] = index
		if previous, exists := previousOrder[node.ID]; exists {
			score.Movement += absoluteInt(index - previous)
		}
	}
	for index, edge := range edges {
		start, startExists := positions[edge.PrerequisiteID]
		end, endExists := positions[edge.DependentID]
		if !startExists || !endExists {
			continue
		}
		score.EdgeSpan += end - start
		for _, other := range edges[index+1:] {
			otherStart, otherStartExists := positions[other.PrerequisiteID]
			otherEnd, otherEndExists := positions[other.DependentID]
			if !otherStartExists || !otherEndExists {
				continue
			}
			if intervalsInterleave(start, end, otherStart, otherEnd) {
				score.Crossings++
			}
		}
	}
	return score
}

func curriculumNodeOrderKey(nodes []models.CurriculumGraphNode) string {
	var key strings.Builder
	for index, node := range nodes {
		if index > 0 {
			key.WriteByte(',')
		}
		fmt.Fprintf(&key, "%d", node.ID)
	}
	return key.String()
}

func (score curriculumOrderScore) betterThan(other curriculumOrderScore) bool {
	if score.Crossings != other.Crossings {
		return score.Crossings < other.Crossings
	}
	if score.EdgeSpan != other.EdgeSpan {
		return score.EdgeSpan < other.EdgeSpan
	}
	return score.Movement < other.Movement
}

func intervalsInterleave(leftStart, leftEnd, rightStart, rightEnd int) bool {
	return leftStart < rightStart && rightStart < leftEnd && leftEnd < rightEnd ||
		rightStart < leftStart && leftStart < rightEnd && rightEnd < leftEnd
}

func absoluteInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func absoluteFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func topologicallyOrderCurriculum(graph *models.CurriculumGraphLayout, previousOrder map[int64]int) error {
	if graph == nil || len(graph.Nodes) < 2 {
		return nil
	}
	nodesByID := make(map[int64]models.CurriculumGraphNode, len(graph.Nodes))
	indegree := make(map[int64]int, len(graph.Nodes))
	dependents := make(map[int64][]int64, len(graph.Nodes))
	prerequisites := make(map[int64][]int64, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodesByID[node.ID] = node
		indegree[node.ID] = 0
	}
	for _, edge := range graph.Edges {
		if _, exists := nodesByID[edge.PrerequisiteID]; !exists {
			continue
		}
		if _, exists := nodesByID[edge.DependentID]; !exists {
			continue
		}
		indegree[edge.DependentID]++
		dependents[edge.PrerequisiteID] = append(dependents[edge.PrerequisiteID], edge.DependentID)
		prerequisites[edge.DependentID] = append(prerequisites[edge.DependentID], edge.PrerequisiteID)
	}

	available := make([]models.CurriculumGraphNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if indegree[node.ID] == 0 {
			available = append(available, node)
		}
	}
	sortCurriculumNodes(available)
	ordered := make([]models.CurriculumGraphNode, 0, len(graph.Nodes))
	positions := make(map[int64]int, len(graph.Nodes))
	for len(available) > 0 {
		sortCurriculumTraversalCandidates(available, ordered, positions, prerequisites, previousOrder)
		node := available[0]
		available = available[1:]
		positions[node.ID] = len(ordered)
		ordered = append(ordered, node)
		for _, dependentID := range dependents[node.ID] {
			indegree[dependentID]--
			if indegree[dependentID] == 0 {
				available = append(available, nodesByID[dependentID])
			}
		}
	}
	if len(ordered) != len(graph.Nodes) {
		return fmt.Errorf("%w: published curriculum", ErrDependencyCycle)
	}
	graph.Nodes = ordered
	return nil
}

func sortCurriculumTraversalCandidates(
	nodes []models.CurriculumGraphNode,
	ordered []models.CurriculumGraphNode,
	positions map[int64]int,
	prerequisites map[int64][]int64,
	previousOrder map[int64]int,
) {
	if len(nodes) < 2 {
		return
	}
	var previousID int64
	if len(ordered) > 0 {
		previousID = ordered[len(ordered)-1].ID
	}
	latestPrerequisite := func(nodeID int64) int {
		latest := -1
		for _, prerequisiteID := range prerequisites[nodeID] {
			if position, exists := positions[prerequisiteID]; exists && position > latest {
				latest = position
			}
		}
		return latest
	}
	dependsOnPrevious := func(nodeID int64) bool {
		for _, prerequisiteID := range prerequisites[nodeID] {
			if prerequisiteID == previousID {
				return true
			}
		}
		return false
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		leftPrevious, leftExisted := previousOrder[nodes[i].ID]
		rightPrevious, rightExisted := previousOrder[nodes[j].ID]
		if leftExisted != rightExisted {
			return leftExisted
		}
		if leftExisted && leftPrevious != rightPrevious {
			return leftPrevious < rightPrevious
		}
		leftContinues := previousID != 0 && dependsOnPrevious(nodes[i].ID)
		rightContinues := previousID != 0 && dependsOnPrevious(nodes[j].ID)
		if leftContinues != rightContinues {
			return leftContinues
		}
		leftLatest, rightLatest := latestPrerequisite(nodes[i].ID), latestPrerequisite(nodes[j].ID)
		if leftLatest != rightLatest {
			return leftLatest > rightLatest
		}
		leftName, rightName := strings.ToLower(nodes[i].Name), strings.ToLower(nodes[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func sortCurriculumNodes(nodes []models.CurriculumGraphNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left, right := strings.ToLower(nodes[i].Name), strings.ToLower(nodes[j].Name)
		if left != right {
			return left < right
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func assignCurriculumGraphLanes(graph *models.CurriculumGraphLayout, previousLanes map[int64]float64) {
	if graph == nil {
		return
	}
	fresh := cloneCurriculumGraphLayout(graph)
	assignCurriculumGraphLaneCandidate(fresh, nil)
	if len(previousLanes) == 0 {
		*graph = *fresh
		return
	}
	continuous := cloneCurriculumGraphLayout(graph)
	assignCurriculumGraphLaneCandidate(continuous, previousLanes)
	hybrid := cloneCurriculumGraphLayout(fresh)
	stabilizeCurriculumNodeLanes(hybrid, previousLanes)
	*graph = *preferredCurriculumLaneLayout(
		[]*models.CurriculumGraphLayout{fresh, continuous, hybrid}, previousLanes,
	)
}

// stabilizeCurriculumNodeLanes retains the compact fresh routing and only
// moves persistent nodes towards their previous lane when that lane is free at
// the node's row. The normal candidate scoring rejects the result if these
// local continuity improvements introduce too many bends.
func stabilizeCurriculumNodeLanes(graph *models.CurriculumGraphLayout, previousLanes map[int64]float64) {
	if graph == nil || graph.LaneCount == 0 || len(previousLanes) == 0 {
		return
	}
	nodeIndexes := make(map[int64]int, len(graph.Nodes))
	for index, node := range graph.Nodes {
		nodeIndexes[node.ID] = index
	}
	for index := range graph.Nodes {
		node := &graph.Nodes[index]
		previous, exists := previousLanes[node.ID]
		if !exists {
			continue
		}
		target := min(max(previous, 0), float64(graph.LaneCount-1))
		if absoluteFloat(target-previous) >= absoluteFloat(node.Lane-previous) ||
			curriculumLaneCrossesNodeRow(graph, nodeIndexes, index, target) {
			continue
		}
		node.Lane = target
	}
	compactCurriculumGraphLanes(graph)
}

func curriculumLaneCrossesNodeRow(
	graph *models.CurriculumGraphLayout,
	nodeIndexes map[int64]int,
	nodeIndex int,
	lane float64,
) bool {
	for _, edge := range graph.Edges {
		start := nodeIndexes[edge.PrerequisiteID]
		end := nodeIndexes[edge.DependentID]
		if start >= nodeIndex || nodeIndex >= end {
			continue
		}
		edgeLane := edge.Lane
		if graph.Nodes[start].Lane == graph.Nodes[end].Lane {
			edgeLane = graph.Nodes[start].Lane
		}
		if edgeLane == lane {
			return true
		}
	}
	return false
}

func assignCurriculumGraphLaneCandidate(graph *models.CurriculumGraphLayout, previousLanes map[int64]float64) {
	if graph == nil || len(graph.Edges) == 0 {
		return
	}
	nodeIndexes := make(map[int64]int, len(graph.Nodes))
	for index, node := range graph.Nodes {
		nodeIndexes[node.ID] = index
	}
	sort.SliceStable(graph.Edges, func(i, j int) bool {
		fromI, fromJ := nodeIndexes[graph.Edges[i].PrerequisiteID], nodeIndexes[graph.Edges[j].PrerequisiteID]
		if fromI != fromJ {
			return fromI < fromJ
		}
		toI, toJ := nodeIndexes[graph.Edges[i].DependentID], nodeIndexes[graph.Edges[j].DependentID]
		if toI != toJ {
			return toI < toJ
		}
		return graph.Edges[i].PrerequisiteID < graph.Edges[j].PrerequisiteID
	})

	laneEnds := make([]int, 0)
	for index := range graph.Edges {
		edge := &graph.Edges[index]
		start, startExists := nodeIndexes[edge.PrerequisiteID]
		end, endExists := nodeIndexes[edge.DependentID]
		if !startExists || !endExists {
			continue
		}
		lane := -1
		for candidate, laneEnd := range laneEnds {
			if laneEnd < start {
				lane = candidate
				break
			}
		}
		if lane == -1 {
			lane = len(laneEnds)
			laneEnds = append(laneEnds, -1)
		}
		edge.Lane = float64(lane)
		laneEnds[lane] = end
	}
	assignCurriculumNodeLanes(graph, nodeIndexes)
	if len(previousLanes) > 0 {
		for index := range graph.Nodes {
			if lane, exists := previousLanes[graph.Nodes[index].ID]; exists {
				graph.Nodes[index].Lane = lane
			}
		}
		avoidCurriculumNodeLaneCollisions(graph, nodeIndexes)
	}
	compactCurriculumGraphLanes(graph)
}

type curriculumLaneScore struct {
	Width    int
	Bends    float64
	Movement float64
}

func preferredCurriculumLaneLayout(
	candidates []*models.CurriculumGraphLayout,
	previousLanes map[int64]float64,
) *models.CurriculumGraphLayout {
	scores := make([]curriculumLaneScore, len(candidates))
	bestWidth := 0
	bestBends := 0.0
	for index, candidate := range candidates {
		scores[index] = scoreCurriculumLanes(candidate, previousLanes)
		if index == 0 || scores[index].Width < bestWidth {
			bestWidth = scores[index].Width
			bestBends = scores[index].Bends
		} else if scores[index].Width == bestWidth && scores[index].Bends < bestBends {
			bestBends = scores[index].Bends
		}
	}
	bendTolerance := float64(max(1, len(candidates[0].Edges)/8))
	bestIndex := -1
	for index, score := range scores {
		if score.Width != bestWidth || score.Bends > bestBends+bendTolerance {
			continue
		}
		if bestIndex == -1 || score.Movement < scores[bestIndex].Movement ||
			score.Movement == scores[bestIndex].Movement && score.Bends < scores[bestIndex].Bends {
			bestIndex = index
		}
	}
	return candidates[bestIndex]
}

func scoreCurriculumLanes(
	graph *models.CurriculumGraphLayout,
	previousLanes map[int64]float64,
) curriculumLaneScore {
	score := curriculumLaneScore{Width: graph.LaneCount}
	nodeLanes := make(map[int64]float64, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeLanes[node.ID] = node.Lane
		if previous, exists := previousLanes[node.ID]; exists {
			score.Movement += absoluteFloat(node.Lane - previous)
		}
	}
	for _, edge := range graph.Edges {
		score.Bends += absoluteFloat(nodeLanes[edge.PrerequisiteID]-edge.Lane) +
			absoluteFloat(nodeLanes[edge.DependentID]-edge.Lane)
	}
	return score
}

func assignCurriculumNodeLanes(graph *models.CurriculumGraphLayout, nodeIndexes map[int64]int) {
	incoming := make(map[int64][]models.CurriculumGraphEdge, len(graph.Nodes))
	outgoing := make(map[int64][]models.CurriculumGraphEdge, len(graph.Nodes))
	for _, edge := range graph.Edges {
		incoming[edge.DependentID] = append(incoming[edge.DependentID], edge)
		outgoing[edge.PrerequisiteID] = append(outgoing[edge.PrerequisiteID], edge)
	}
	for index := range graph.Nodes {
		node := &graph.Nodes[index]
		edges := incoming[node.ID]
		preferredLane := node.Lane
		if len(edges) == 1 {
			edge := edges[0]
			source := &graph.Nodes[nodeIndexes[edge.PrerequisiteID]]
			sourceEdges := outgoing[source.ID]
			if len(sourceEdges) == 1 || isPrimaryCurriculumBranch(edge, sourceEdges, nodeIndexes) {
				preferredLane = source.Lane
			} else {
				preferredLane = edge.Lane
			}
		} else if len(edges) > 1 {
			chosen := edges[0]
			for _, edge := range edges[1:] {
				chosenIndex, edgeIndex := nodeIndexes[chosen.PrerequisiteID], nodeIndexes[edge.PrerequisiteID]
				if edgeIndex > chosenIndex || edgeIndex == chosenIndex && edge.Lane < chosen.Lane {
					chosen = edge
				}
			}
			preferredLane = graph.Nodes[nodeIndexes[chosen.PrerequisiteID]].Lane
		} else if edges := outgoing[node.ID]; len(edges) > 0 {
			chosen := edges[0]
			for _, edge := range edges[1:] {
				chosenIndex, edgeIndex := nodeIndexes[chosen.DependentID], nodeIndexes[edge.DependentID]
				if edgeIndex > chosenIndex || edgeIndex == chosenIndex && edge.Lane < chosen.Lane {
					chosen = edge
				}
			}
			preferredLane = chosen.Lane
		}
		node.Lane = preferredLane
	}
	avoidCurriculumNodeLaneCollisions(graph, nodeIndexes)
}

func avoidCurriculumNodeLaneCollisions(graph *models.CurriculumGraphLayout, nodeIndexes map[int64]int) {
	incoming := make(map[int64][]models.CurriculumGraphEdge, len(graph.Nodes))
	outgoing := make(map[int64][]models.CurriculumGraphEdge, len(graph.Nodes))
	for _, edge := range graph.Edges {
		incoming[edge.DependentID] = append(incoming[edge.DependentID], edge)
		outgoing[edge.PrerequisiteID] = append(outgoing[edge.PrerequisiteID], edge)
	}
	for pass := 0; pass < len(graph.Nodes); pass++ {
		changed := false
		for nodeIndex := range graph.Nodes {
			occupied := make(map[float64]bool)
			for _, edge := range graph.Edges {
				start := nodeIndexes[edge.PrerequisiteID]
				end := nodeIndexes[edge.DependentID]
				if start >= nodeIndex || nodeIndex >= end {
					continue
				}
				lane := edge.Lane
				if graph.Nodes[start].Lane == graph.Nodes[end].Lane {
					lane = graph.Nodes[start].Lane
				}
				occupied[lane] = true
			}
			lane := graph.Nodes[nodeIndex].Lane
			if !occupied[lane] {
				continue
			}
			newLane := curriculumLaneBetweenNeighbours(lane, graph)
			graph.Nodes[nodeIndex].Lane = newLane
			moveCurriculumStraightDescendants(graph, nodeIndexes, incoming, outgoing, nodeIndex, lane, newLane)
			changed = true
		}
		if !changed {
			return
		}
	}
}

func moveCurriculumStraightDescendants(
	graph *models.CurriculumGraphLayout,
	nodeIndexes map[int64]int,
	incoming, outgoing map[int64][]models.CurriculumGraphEdge,
	nodeIndex int,
	oldLane, newLane float64,
) {
	node := graph.Nodes[nodeIndex]
	edges := outgoing[node.ID]
	for _, edge := range edges {
		dependentIndex := nodeIndexes[edge.DependentID]
		dependent := &graph.Nodes[dependentIndex]
		if len(incoming[dependent.ID]) != 1 || dependent.Lane != oldLane {
			continue
		}
		if len(edges) != 1 && !isPrimaryCurriculumBranch(edge, edges, nodeIndexes) {
			continue
		}
		dependent.Lane = newLane
		moveCurriculumStraightDescendants(graph, nodeIndexes, incoming, outgoing, dependentIndex, oldLane, newLane)
	}
}

func curriculumLaneBetweenNeighbours(lane float64, graph *models.CurriculumGraphLayout) float64 {
	positions := make(map[float64]bool, len(graph.Nodes)+len(graph.Edges))
	for _, node := range graph.Nodes {
		positions[node.Lane] = true
	}
	for _, edge := range graph.Edges {
		positions[edge.Lane] = true
	}
	ordered := sortedLanes(positions)
	for index, position := range ordered {
		if position != lane {
			continue
		}
		if index > 0 {
			return (ordered[index-1] + lane) / 2
		}
		if index+1 < len(ordered) {
			return (lane + ordered[index+1]) / 2
		}
		return lane + 0.5
	}
	return lane
}

func isPrimaryCurriculumBranch(edge models.CurriculumGraphEdge, outgoing []models.CurriculumGraphEdge, nodeIndexes map[int64]int) bool {
	primary := outgoing[0]
	for _, candidate := range outgoing[1:] {
		primaryIndex, candidateIndex := nodeIndexes[primary.DependentID], nodeIndexes[candidate.DependentID]
		if candidateIndex > primaryIndex || candidateIndex == primaryIndex && candidate.Lane < primary.Lane {
			primary = candidate
		}
	}
	return primary.PrerequisiteID == edge.PrerequisiteID && primary.DependentID == edge.DependentID
}

func compactCurriculumGraphLanes(graph *models.CurriculumGraphLayout) {
	nodeLaneUsed := make(map[float64]bool)
	edgeLaneUsed := make(map[float64]bool)
	nodeLanes := make(map[int64]float64, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodeLaneUsed[node.Lane] = true
		nodeLanes[node.ID] = node.Lane
	}
	for _, edge := range graph.Edges {
		if nodeLanes[edge.PrerequisiteID] != nodeLanes[edge.DependentID] {
			edgeLaneUsed[edge.Lane] = true
		}
	}
	nodeLanesInUse := sortedLanes(nodeLaneUsed)
	edgeOnlyLanes := make([]float64, 0, len(edgeLaneUsed))
	for lane := range edgeLaneUsed {
		if !nodeLaneUsed[lane] {
			edgeOnlyLanes = append(edgeOnlyLanes, lane)
		}
	}
	sort.Float64s(edgeOnlyLanes)
	orderedLanes := append(nodeLanesInUse, edgeOnlyLanes...)
	compactLane := make(map[float64]float64, len(orderedLanes))
	for lane, originalLane := range orderedLanes {
		compactLane[originalLane] = float64(lane)
	}
	for index := range graph.Edges {
		lane, exists := compactLane[graph.Edges[index].Lane]
		if !exists {
			lane = compactLane[nodeLanes[graph.Edges[index].PrerequisiteID]]
		}
		graph.Edges[index].Lane = lane
	}
	for index := range graph.Nodes {
		graph.Nodes[index].Lane = compactLane[graph.Nodes[index].Lane]
	}
	graph.LaneCount = len(orderedLanes)
}

func sortedLanes(lanes map[float64]bool) []float64 {
	result := make([]float64, 0, len(lanes))
	for lane := range lanes {
		result = append(result, lane)
	}
	sort.Float64s(result)
	return result
}
