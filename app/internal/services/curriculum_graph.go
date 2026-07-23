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
	curriculumDirectNeighborLimit = 5
	curriculumSecondNeighborLimit = 5
	curriculumCoPrerequisiteLimit = 3
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
		directPrerequisites := includeCurriculumNeighbors(included, prerequisites[unit.ID], curriculumDirectNeighborLimit)
		directDependents := includeCurriculumNeighbors(included, dependents[unit.ID], curriculumDirectNeighborLimit)
		includeSecondCurriculumNeighbors(included, directPrerequisites, prerequisites, curriculumSecondNeighborLimit)
		secondDependents := includeSecondCurriculumNeighbors(included, directDependents, dependents, curriculumSecondNeighborLimit)
		includeSecondCurriculumNeighbors(included, secondDependents, dependents, curriculumSecondNeighborLimit)

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
	var boundaries []models.CurriculumGraphBoundary
	for _, unit := range neighborhood.Units {
		hiddenPrerequisites := countHiddenCurriculumNeighbors(prerequisites[unit.ID], included)
		if hiddenPrerequisites > 0 {
			boundaries = append(boundaries, models.CurriculumGraphBoundary{
				UnitID: unit.ID, Direction: "prerequisites", Count: hiddenPrerequisites,
			})
		}
		hiddenDependents := countHiddenCurriculumNeighbors(dependents[unit.ID], included)
		if hiddenDependents > 0 {
			boundaries = append(boundaries, models.CurriculumGraphBoundary{
				UnitID: unit.ID, Direction: "dependents", Count: hiddenDependents,
			})
		}
	}
	return neighborhood, focus, boundaries, nil
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
	if err := topologicallyOrderCurriculum(layout, hints.Order); err != nil {
		return nil, err
	}
	assignCurriculumGraphLanes(layout, hints.Lanes)
	return layout, nil
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
