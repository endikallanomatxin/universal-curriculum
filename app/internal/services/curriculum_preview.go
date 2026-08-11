package services

import "universal-curriculum/internal/models"

func CurriculumGraphWithProposal(graph *models.CurriculumGraph, proposal *models.CurriculumProposal) *models.CurriculumGraph {
	if graph == nil || proposal == nil {
		return graph
	}
	preview := graph.Clone()
	unitIndexes := make(map[int64]int, len(preview.Units))
	for index := range preview.Units {
		unitIndexes[preview.Units[index].ID] = index
	}
	for _, change := range canonicalCurriculumProposalChanges(proposal.Changes) {
		switch change.Kind {
		case "create_unit":
			if _, exists := unitIndexes[change.UnitID]; !exists {
				unitIndexes[change.UnitID] = len(preview.Units)
				preview.Units = append(preview.Units, models.Unit{
					ID: change.UnitID, Name: change.UnitName, Content: change.UnitContent,
				})
			}
		case "rename_unit":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units[index].Name = change.UnitName
			}
		case "update_content":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units[index].Content = change.UnitContent
			}
		case "delete_unit":
			if index, exists := unitIndexes[change.UnitID]; exists {
				preview.Units = append(preview.Units[:index], preview.Units[index+1:]...)
				delete(unitIndexes, change.UnitID)
				for following := index; following < len(preview.Units); following++ {
					unitIndexes[preview.Units[following].ID] = following
				}
			}
			filtered := preview.Dependencies[:0]
			for _, dependency := range preview.Dependencies {
				if dependency.UnitID != change.UnitID && dependency.PrerequisiteID != change.UnitID {
					filtered = append(filtered, dependency)
				}
			}
			preview.Dependencies = filtered
		case "add_dependency":
			if change.PrerequisiteID != nil && !preview.HasDependency(change.UnitID, *change.PrerequisiteID) {
				dependency := models.UnitDependency{
					UnitID: change.UnitID, PrerequisiteID: *change.PrerequisiteID,
				}
				if index, exists := unitIndexes[change.UnitID]; exists {
					dependency.UnitName = preview.Units[index].Name
				}
				if index, exists := unitIndexes[*change.PrerequisiteID]; exists {
					dependency.PrerequisiteName = preview.Units[index].Name
				}
				preview.Dependencies = append(preview.Dependencies, dependency)
			}
		case "remove_dependency":
			if change.PrerequisiteID != nil {
				filtered := preview.Dependencies[:0]
				for _, dependency := range preview.Dependencies {
					if dependency.UnitID != change.UnitID || dependency.PrerequisiteID != *change.PrerequisiteID {
						filtered = append(filtered, dependency)
					}
				}
				preview.Dependencies = filtered
			}
		}
	}
	return preview
}

func curriculumDependencyCreatesCycle(graph *models.CurriculumGraph, unitID, prerequisiteID int64) bool {
	index := models.IndexCurriculumGraph(graph)
	pending := []int64{unitID}
	visited := make(map[int64]bool)
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == prerequisiteID {
			return true
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		pending = append(pending, index.Dependents(current)...)
	}
	return false
}
