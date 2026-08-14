package server

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

type curriculumUnitView struct {
	models.Unit
	Prerequisites   []models.Unit
	Dependents      []models.Unit
	Lane            float64
	HasContentDiff  bool
	PreviousContent string
	Historical      bool
}

type curriculumModificationPageData struct {
	userPageData
	Units                   []curriculumUnitView
	Dependencies            []models.UnitDependency
	Graph                   *models.CurriculumGraphLayout
	GraphView               curriculumGraphView
	GraphSearch             unitNavigationSearchView
	FocusedUnit             *models.Unit
	ContentUnit             *curriculumUnitView
	DraftProposals          []curriculumDraftProposalView
	ReviewedProposals       []models.CurriculumProposal
	ReviewedProposalTotal   int
	ReviewedProposalMore    bool
	ReviewedProposalLimit   int
	ReviewedProposalNext    int
	ActiveProposals         []models.CurriculumProposal
	ActiveProposalTotal     int
	ActiveProposalMore      bool
	ActiveProposalLimit     int
	ActiveProposalNext      int
	ActiveProposal          *models.CurriculumProposal
	ProposalRebase          *services.CurriculumProposalRebasePlan
	RebaseTimeline          *curriculumRebaseTimelineView
	ReviewedProposal        *models.CurriculumProposal
	ProposalHistory         []curriculumProposalHistoryView
	RootDraftProposals      []curriculumDraftProposalView
	ShowProposalHistory     bool
	ProposalHistoryMore     bool
	ProposalHistoryLimit    int
	ProposalHistoryNext     int
	CanEditProposal         bool
	ViewingAcceptedProposal bool
	GraphURL                string
	UnitContentCloseURL     string
	RecognitionSources      []models.Unit
	RecognitionTargets      []models.Unit
	PublishWarning          string
	Error                   string
}

type curriculumRebaseTimelineView struct {
	DraftID    int64
	BaseID     int64
	BaseTitle  string
	DraftTitle string
	Items      []curriculumRebaseTimelineItemView
	Edges      []curriculumRebaseTimelineEdgeView
}

type curriculumRebaseTimelineItemView struct {
	ID        int64
	Title     string
	Ellipsis  bool
	Current   bool
	Conflicts bool
}

type curriculumRebaseTimelineEdgeView struct {
	Source string
	Target string
}

type curriculumDraftProposalView struct {
	models.CurriculumProposal
	RebaseStatus  string
	ChangeSummary []curriculumProposalChangeCountView
}

type curriculumProposalChangeCountView struct {
	Kind  string
	Count int
	Label string
}

type curriculumProposalHistoryView struct {
	models.CurriculumProposal
	Drafts []curriculumDraftProposalView
	IsHead bool
}

type proposalMetadataSaveStatusView struct {
	Error string
}

func curriculumProposalChangeSummary(counts map[string]int) []curriculumProposalChangeCountView {
	categories := []struct {
		kind     string
		singular string
		plural   string
		kinds    []string
	}{
		{kind: "created", singular: "unit creation", plural: "unit creations", kinds: []string{"create_unit"}},
		{kind: "deleted", singular: "unit deletion", plural: "unit deletions", kinds: []string{"delete_unit"}},
		{kind: "renamed", singular: "unit rename", plural: "unit renames", kinds: []string{"rename_unit"}},
		{kind: "content", singular: "content update", plural: "content updates", kinds: []string{"update_content"}},
		{kind: "dependency", singular: "dependency change", plural: "dependency changes", kinds: []string{"add_dependency", "remove_dependency"}},
		{kind: "recognition", singular: "recognition", plural: "recognitions", kinds: []string{"recognition"}},
	}
	var summary []curriculumProposalChangeCountView
	for _, category := range categories {
		count := 0
		for _, kind := range category.kinds {
			count += counts[kind]
		}
		if count > 0 {
			label := category.plural
			if count == 1 {
				label = category.singular
			}
			summary = append(summary, curriculumProposalChangeCountView{
				Kind: category.kind, Count: count, Label: label,
			})
		}
	}
	return summary
}

func applyUnitContentDiff(unit *curriculumUnitView, proposal *models.CurriculumProposal) {
	if unit == nil || proposal == nil {
		return
	}
	for _, change := range proposal.Changes {
		if change.Kind == "update_content" && change.UnitID == unit.ID {
			unit.HasContentDiff = true
			unit.PreviousContent = change.PreviousUnitContent
			return
		}
	}
}

func curriculumGraphWithProposal(graph *models.CurriculumGraph, proposal *models.CurriculumProposal) *models.CurriculumGraph {
	return services.CurriculumGraphWithProposal(graph, proposal)
}

func curriculumGraphWithRemovedDependencies(working, published *models.CurriculumGraph, proposal *models.CurriculumProposal) *models.CurriculumGraph {
	if working == nil || proposal == nil {
		return working
	}
	visual := working.Clone()
	publishedDependencies := make(map[[2]int64]models.UnitDependency)
	publishedUnits := make(map[int64]models.Unit)
	if published != nil {
		for _, unit := range published.Units {
			publishedUnits[unit.ID] = unit
		}
		for _, dependency := range published.Dependencies {
			publishedDependencies[[2]int64{dependency.PrerequisiteID, dependency.UnitID}] = dependency
		}
	}
	deletedIDs := make(map[int64]bool)
	for _, change := range proposal.Changes {
		if change.Kind != "delete_unit" {
			continue
		}
		deletedIDs[change.UnitID] = true
		if unit, exists := publishedUnits[change.UnitID]; exists && visual.Unit(change.UnitID) == nil {
			visual.Units = append(visual.Units, unit)
		}
	}
	for key, dependency := range publishedDependencies {
		if (deletedIDs[key[0]] || deletedIDs[key[1]]) && !visual.HasDependency(key[1], key[0]) {
			visual.Dependencies = append(visual.Dependencies, dependency)
		}
	}
	for _, change := range proposal.Changes {
		if change.Kind != "remove_dependency" || change.PrerequisiteID == nil {
			continue
		}
		key := [2]int64{*change.PrerequisiteID, change.UnitID}
		dependency, exists := publishedDependencies[key]
		if !exists {
			dependency = models.UnitDependency{PrerequisiteID: key[0], UnitID: key[1]}
		}
		if !visual.HasDependency(key[1], key[0]) {
			visual.Dependencies = append(visual.Dependencies, dependency)
		}
	}
	return visual
}

func positionIsolatedCreatedUnits(layout *models.CurriculumGraphLayout, proposal *models.CurriculumProposal) {
	if layout == nil || proposal == nil {
		return
	}
	createdIDs := make(map[int64]bool)
	for _, change := range proposal.Changes {
		if change.Kind == "create_unit" {
			createdIDs[change.UnitID] = true
		}
	}
	connectedIDs := make(map[int64]bool)
	for _, edge := range layout.Edges {
		connectedIDs[edge.PrerequisiteID] = true
		connectedIDs[edge.DependentID] = true
	}
	isolated := make([]models.CurriculumGraphNode, 0)
	connected := make([]models.CurriculumGraphNode, 0, len(layout.Nodes))
	for _, node := range layout.Nodes {
		if createdIDs[node.ID] && !connectedIDs[node.ID] {
			isolated = append(isolated, node)
		} else {
			connected = append(connected, node)
		}
	}
	layout.Nodes = append(isolated, connected...)
}

func applyProposalGraphStates(view *curriculumGraphView, proposal *models.CurriculumProposal) {
	if view == nil || proposal == nil {
		return
	}
	priority := map[string]int{"content": 1, "rename": 2, "created": 3, "deleted": 4}
	states := make(map[int64]string)
	for _, change := range proposal.Changes {
		state := ""
		switch change.Kind {
		case "create_unit":
			state = "created"
		case "delete_unit":
			state = "deleted"
		case "rename_unit":
			state = "rename"
		case "update_content":
			state = "content"
		}
		if priority[state] > priority[states[change.UnitID]] {
			states[change.UnitID] = state
		}
	}
	for index := range view.Nodes {
		view.Nodes[index].ProposalState = states[view.Nodes[index].ID]
	}
	connectedIDs := make(map[int64]bool)
	for _, edge := range view.Edges {
		connectedIDs[edge.PrerequisiteID] = true
		connectedIDs[edge.DependentID] = true
	}
	for index := range view.Nodes {
		node := &view.Nodes[index]
		node.IsProposedIsolated = node.ProposalState == "created" && !connectedIDs[node.ID]
	}
	proposedDependencies := make(map[[2]int64]string)
	for _, change := range proposal.Changes {
		if change.PrerequisiteID == nil {
			continue
		}
		key := [2]int64{*change.PrerequisiteID, change.UnitID}
		switch change.Kind {
		case "add_dependency":
			proposedDependencies[key] = "created"
		case "remove_dependency":
			proposedDependencies[key] = "deleted"
		}
	}
	for index := range view.Edges {
		edge := &view.Edges[index]
		edge.ProposalState = proposedDependencies[[2]int64{edge.PrerequisiteID, edge.DependentID}]
		if states[edge.PrerequisiteID] == "deleted" || states[edge.DependentID] == "deleted" {
			edge.ProposalState = "deleted"
		}
	}
}

func applyCurriculumChangeLabels(proposal *models.CurriculumProposal, graph *models.CurriculumGraph) {
	if proposal == nil || graph == nil {
		return
	}
	names := make(map[int64]string, len(graph.Units))
	for _, unit := range graph.Units {
		names[unit.ID] = unit.Name
	}
	for index := range proposal.Changes {
		if proposal.Changes[index].UnitName == "" {
			proposal.Changes[index].UnitName = names[proposal.Changes[index].UnitID]
		}
		if proposal.Changes[index].PrerequisiteID != nil && proposal.Changes[index].PrerequisiteName == "" {
			proposal.Changes[index].PrerequisiteName = names[*proposal.Changes[index].PrerequisiteID]
		}
		if proposal.Changes[index].Recognition == nil {
			continue
		}
		for sourceIndex := range proposal.Changes[index].Recognition.Sources {
			source := &proposal.Changes[index].Recognition.Sources[sourceIndex]
			if name := names[source.ID]; name != "" {
				source.Name = name
			}
		}
		for targetIndex := range proposal.Changes[index].Recognition.Targets {
			target := &proposal.Changes[index].Recognition.Targets[targetIndex]
			if name := names[target.ID]; name != "" {
				target.Name = name
			}
		}
	}
}

func curriculumRecognitionPublishWarning(proposal *models.CurriculumProposal) string {
	coverage := services.CurriculumRecognitionCoverage(proposal)
	createdCount := len(coverage.CreatedWithoutSource)
	deletedCount := len(coverage.DeletedWithoutTarget)
	if createdCount == 0 && deletedCount == 0 {
		return ""
	}
	parts := make([]string, 0, 2)
	if deletedCount > 0 {
		label := "units"
		if deletedCount == 1 {
			label = "unit"
		}
		parts = append(parts, fmt.Sprintf("%d retired %s without outgoing recognition", deletedCount, label))
	}
	if createdCount > 0 {
		label := "units"
		if createdCount == 1 {
			label = "unit"
		}
		parts = append(parts, fmt.Sprintf("%d new %s without incoming recognition", createdCount, label))
	}
	message := "This proposal contains " + strings.Join(parts, " and ") + "."
	if deletedCount > 0 {
		message += " Historical progress and certifications will remain recorded, but they will not be recognized in another unit."
	}
	if createdCount > 0 {
		message += " No prior progress will be recognized in the new units."
	}
	return message + " Continue only if the new knowledge or lack of a successor is intentional. Publish anyway?"
}

func curriculumProposalHistory(
	accepted []models.CurriculumProposal,
	drafts []curriculumDraftProposalView,
) ([]curriculumProposalHistoryView, []curriculumDraftProposalView) {
	accepted = slices.DeleteFunc(accepted, func(proposal models.CurriculumProposal) bool {
		return proposal.Status != "accepted"
	})
	draftsByBase := make(map[int64][]curriculumDraftProposalView)
	var rootDrafts []curriculumDraftProposalView
	for _, draft := range drafts {
		if draft.BaseProposalID == nil {
			rootDrafts = append(rootDrafts, draft)
			continue
		}
		draftsByBase[*draft.BaseProposalID] = append(draftsByBase[*draft.BaseProposalID], draft)
	}
	history := make([]curriculumProposalHistoryView, 0, len(accepted))
	for index, proposal := range accepted {
		history = append(history, curriculumProposalHistoryView{
			CurriculumProposal: proposal,
			Drafts:             draftsByBase[proposal.ID],
			IsHead:             index == 0,
		})
	}
	return history, rootDrafts
}

func curriculumRebaseTimeline(
	plan *services.CurriculumProposalRebasePlan,
	draft *models.CurriculumProposal,
) *curriculumRebaseTimelineView {
	if plan == nil || draft == nil || !plan.NeedsReview() || len(plan.AcceptedProposals) == 0 {
		return nil
	}
	view := &curriculumRebaseTimelineView{
		DraftID:    draft.ID,
		BaseTitle:  "Previous accepted curriculum",
		DraftTitle: draft.Title,
	}
	if plan.BaseProposal != nil && plan.BaseProposal.Title != "" {
		view.BaseID = plan.BaseProposal.ID
		view.BaseTitle = plan.BaseProposal.Title
	}
	conflicting := make(map[int64]bool)
	for _, conflict := range plan.Conflicts {
		for _, work := range conflict.AcceptedWork {
			conflicting[work.Proposal.ID] = true
		}
	}
	for index, proposal := range plan.AcceptedProposals {
		current := index == len(plan.AcceptedProposals)-1
		if !current && !conflicting[proposal.ID] {
			if len(view.Items) == 0 || !view.Items[len(view.Items)-1].Ellipsis {
				view.Items = append(view.Items, curriculumRebaseTimelineItemView{Ellipsis: true})
			}
			continue
		}
		view.Items = append(view.Items, curriculumRebaseTimelineItemView{
			ID: proposal.ID, Title: proposal.Title, Current: current, Conflicts: conflicting[proposal.ID],
		})
	}
	slices.Reverse(view.Items)
	previous := ""
	for _, item := range view.Items {
		if item.Ellipsis {
			continue
		}
		target := "accepted-" + strconv.FormatInt(item.ID, 10)
		if previous != "" {
			view.Edges = append(view.Edges, curriculumRebaseTimelineEdgeView{Source: target, Target: previous})
		}
		previous = target
	}
	view.Edges = append(view.Edges,
		curriculumRebaseTimelineEdgeView{Source: "base", Target: previous},
		curriculumRebaseTimelineEdgeView{Source: "base", Target: "draft"},
	)
	return view
}

func visibleRebaseProposal(plan *services.CurriculumProposalRebasePlan, proposalID int64) *models.CurriculumProposal {
	if plan == nil || !plan.NeedsReview() {
		return nil
	}
	if plan.BaseProposal != nil && plan.BaseProposal.ID == proposalID {
		return plan.BaseProposal
	}
	conflicting := make(map[int64]bool)
	for _, conflict := range plan.Conflicts {
		for _, work := range conflict.AcceptedWork {
			conflicting[work.Proposal.ID] = true
		}
	}
	for index := range plan.AcceptedProposals {
		proposal := &plan.AcceptedProposals[index]
		if proposal.ID != proposalID {
			continue
		}
		current := index == len(plan.AcceptedProposals)-1
		if current || conflicting[proposalID] {
			return proposal
		}
		return nil
	}
	return nil
}

func curriculumUnitViews(graph *models.CurriculumGraph, layout *models.CurriculumGraphLayout) []curriculumUnitView {
	if graph == nil {
		return nil
	}
	unitsByID := make(map[int64]models.Unit, len(graph.Units))
	viewsByID := make(map[int64]*curriculumUnitView, len(graph.Units))
	views := make([]curriculumUnitView, len(graph.Units))
	for index, unit := range graph.Units {
		unitsByID[unit.ID] = unit
		views[index].Unit = unit
		viewsByID[unit.ID] = &views[index]
	}
	for _, dependency := range graph.Dependencies {
		dependent := viewsByID[dependency.UnitID]
		prerequisite := viewsByID[dependency.PrerequisiteID]
		if dependent == nil || prerequisite == nil {
			continue
		}
		dependent.Prerequisites = append(dependent.Prerequisites, unitsByID[dependency.PrerequisiteID])
		prerequisite.Dependents = append(prerequisite.Dependents, unitsByID[dependency.UnitID])
	}
	if layout != nil && len(layout.Nodes) == len(views) {
		ordered := make([]curriculumUnitView, 0, len(views))
		for _, node := range layout.Nodes {
			view := viewsByID[node.ID]
			if view == nil {
				continue
			}
			view.Lane = node.Lane
			ordered = append(ordered, *view)
		}
		views = ordered
	}
	return views
}
