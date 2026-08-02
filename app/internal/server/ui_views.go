package server

import (
	"strconv"
	"strings"

	"universal-curriculum/internal/models"
)

type curriculumGraphNodeView struct {
	models.CurriculumGraphNode
	NavigateURL        string
	ContentURL         string
	ProposalState      string
	IsProposedIsolated bool
	IsOOB              bool
	IsCurrent          bool
	IsTarget           bool
	IsCompleted        bool
	IsRecognized       bool
	HasProgress        bool
}

type unitCompletionView struct {
	UnitID     int64
	CSRFToken  string
	ReturnURL  string
	Completed  bool
	Recognized bool
}

type unitCompletionUpdateView struct {
	Completion unitCompletionView
	Anchor     curriculumGraphNodeView
}

type curriculumGraphEdgeView struct {
	models.CurriculumGraphEdge
	ProposalState string
}

type curriculumGraphView struct {
	IDPrefix    string
	Description string
	Layout      *models.CurriculumGraphLayout
	Nodes       []curriculumGraphNodeView
	Edges       []curriculumGraphEdgeView
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

func curriculumUnitURLs(
	unitURL func(int64) string,
	contentOpen bool,
) (navigateURL func(int64) string, contentURL func(int64) string) {
	contentURL = func(unitID int64) string {
		target := unitURL(unitID)
		separator := "?"
		if strings.Contains(target, "?") {
			separator = "&"
		}
		return target + separator + "content=" + strconv.FormatInt(unitID, 10)
	}
	navigateURL = unitURL
	if contentOpen {
		navigateURL = contentURL
	}
	return navigateURL, contentURL
}

func newCurriculumGraphView(
	idPrefix string,
	description string,
	layout *models.CurriculumGraphLayout,
	focusedUnit *models.Unit,
	targetUnitIDs map[int64]bool,
	completionStatuses map[int64]models.UnitCompletionStatus,
	hasProgress bool,
	navigateURL func(int64) string,
	contentURL func(int64) string,
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
		completion := completionStatuses[node.ID]
		view.Nodes = append(view.Nodes, curriculumGraphNodeView{
			CurriculumGraphNode: node,
			NavigateURL:         navigateURL(node.ID),
			ContentURL:          contentURL(node.ID),
			IsCurrent:           node.ID == currentID,
			IsTarget:            targetUnitIDs[node.ID],
			IsCompleted:         completion.Completed(),
			IsRecognized:        completion.Recognized && !completion.Direct,
			HasProgress:         hasProgress,
		})
	}
	view.Edges = make([]curriculumGraphEdgeView, 0, len(layout.Edges))
	for _, edge := range layout.Edges {
		view.Edges = append(view.Edges, curriculumGraphEdgeView{CurriculumGraphEdge: edge})
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
