package server

import (
	"universal-curriculum/internal/models"
)

type curriculumGraphNodeView struct {
	models.CurriculumGraphNode
	URL       string
	IsCurrent bool
	IsTarget  bool
}

type curriculumGraphView struct {
	IDPrefix    string
	Description string
	Layout      *models.CurriculumGraphLayout
	Nodes       []curriculumGraphNodeView
}

type unitNavigationOptionView struct {
	Name string
	URL  string
}

type unitNavigationSearchView struct {
	ID          string
	Label       string
	Placeholder string
	Options     []unitNavigationOptionView
}

func newCurriculumGraphView(
	idPrefix string,
	description string,
	layout *models.CurriculumGraphLayout,
	focusedUnit *models.Unit,
	targetUnitIDs map[int64]bool,
	unitURL func(int64) string,
) curriculumGraphView {
	view := curriculumGraphView{IDPrefix: idPrefix, Description: description, Layout: layout}
	if layout == nil {
		return view
	}
	currentID := int64(0)
	if focusedUnit != nil {
		currentID = focusedUnit.ID
	}
	view.Nodes = make([]curriculumGraphNodeView, 0, len(layout.Nodes))
	for _, node := range layout.Nodes {
		view.Nodes = append(view.Nodes, curriculumGraphNodeView{
			CurriculumGraphNode: node,
			URL:                 unitURL(node.ID),
			IsCurrent:           node.ID == currentID,
			IsTarget:            targetUnitIDs[node.ID],
		})
	}
	return view
}

func newUnitNavigationSearchView(
	id string,
	label string,
	units []models.Unit,
	unitURL func(int64) string,
) unitNavigationSearchView {
	view := unitNavigationSearchView{
		ID: id, Label: label, Placeholder: "Find a unit",
		Options: make([]unitNavigationOptionView, 0, len(units)),
	}
	for _, unit := range units {
		view.Options = append(view.Options, unitNavigationOptionView{Name: unit.Name, URL: unitURL(unit.ID)})
	}
	return view
}
