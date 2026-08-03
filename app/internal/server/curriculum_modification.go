package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func (server *Server) curriculumModification(writer http.ResponseWriter, request *http.Request) {
	server.renderCurriculumModification(writer, request, http.StatusOK, "")
}

func (server *Server) renderCurriculumModification(writer http.ResponseWriter, request *http.Request, status int, message string) {
	userID, _ := services.SessionUserID(request)
	user, err := db.GetUserByID(server.Database, userID)
	if err != nil || user == nil {
		log.Printf("load curriculum editor: %v", err)
		http.Error(writer, "Unable to load curriculum editor", http.StatusInternalServerError)
		return
	}
	graph, err := db.GetCurriculumGraph(server.Database)
	if err != nil {
		log.Printf("load curriculum graph: %v", err)
		http.Error(writer, "Unable to load curriculum", http.StatusInternalServerError)
		return
	}
	completionStatuses, err := db.UnitCompletionStatuses(server.Database, userID)
	if err != nil {
		log.Printf("load curriculum completion indicators: %v", err)
		http.Error(writer, "Unable to load progress", http.StatusInternalServerError)
		return
	}
	var activeProposal *models.CurriculumProposal
	if proposalValue := request.URL.Query().Get("proposal"); proposalValue != "" {
		if proposalID, parseErr := parsePositiveID(proposalValue); parseErr == nil {
			activeProposal, err = db.GetCurriculumProposal(server.Database, proposalID)
			if err != nil {
				log.Printf("load active curriculum proposal: %v", err)
				http.Error(writer, "Unable to load curriculum proposal", http.StatusInternalServerError)
				return
			}
			if activeProposal != nil && (activeProposal.Status != "draft" || !activeProposal.HasAuthor(userID)) {
				activeProposal = nil
			}
		}
	}
	var rebasePlan *services.CurriculumProposalRebasePlan
	proposalBaseGraph := graph
	if activeProposal != nil {
		rebasePlan, err = services.PlanCurriculumProposalRebase(server.Database, activeProposal)
		if err != nil {
			log.Printf("plan curriculum proposal rebase: %v", err)
			http.Error(writer, "Unable to inspect curriculum proposal base", http.StatusInternalServerError)
			return
		}
		if rebasePlan.NeedsReview() {
			proposalBaseGraph, err = services.CurriculumGraphAtProposal(server.Database, activeProposal.BaseProposalID)
			if err != nil {
				log.Printf("load proposal base curriculum: %v", err)
				http.Error(writer, "Unable to load proposal base curriculum", http.StatusInternalServerError)
				return
			}
		}
		services.PopulateCurriculumProposalPreviousState(proposalBaseGraph, activeProposal)
	}
	var reviewedProposal *models.CurriculumProposal
	if reviewedValue := request.URL.Query().Get("review-proposal"); reviewedValue != "" {
		reviewedID, parseErr := parsePositiveID(reviewedValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum proposal", http.StatusBadRequest)
			return
		}
		reviewedProposal = visibleRebaseProposal(rebasePlan, reviewedID)
		if reviewedProposal == nil {
			http.Error(writer, "Related curriculum proposal not found", http.StatusNotFound)
			return
		}
		reviewedGraph, graphErr := services.CurriculumGraphAtProposal(server.Database, &reviewedProposal.ID)
		if graphErr != nil {
			log.Printf("load related proposal curriculum: %v", graphErr)
			http.Error(writer, "Unable to load related curriculum proposal", http.StatusInternalServerError)
			return
		}
		applyCurriculumChangeLabels(reviewedProposal, reviewedGraph)
	}
	workingGraph := curriculumGraphWithProposal(proposalBaseGraph, activeProposal)
	applyCurriculumChangeLabels(activeProposal, workingGraph)
	visualGraph := curriculumGraphWithRemovedDependencies(workingGraph, proposalBaseGraph, activeProposal)
	var focusID *int64
	if unitValue := request.URL.Query().Get("unit"); unitValue != "" {
		unitID, parseErr := parsePositiveID(unitValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		focusID = &unitID
	}
	visibleGraph, focusedUnit, boundaries, err := services.CurriculumProposalNeighborhood(visualGraph, activeProposal, focusID)
	if errors.Is(err, services.ErrCurriculumUnitNotFound) {
		http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("build curriculum modification neighborhood: %v", err)
		http.Error(writer, "Unable to navigate curriculum", http.StatusInternalServerError)
		return
	}
	layoutHints := curriculumLayoutHints(request)
	layoutHints.AnchorID = focusID
	layout, err := services.BuildCurriculumGraphLayoutWithHints(visibleGraph, layoutHints)
	if err != nil {
		log.Printf("layout curriculum graph: %v", err)
		http.Error(writer, "Unable to lay out curriculum", http.StatusInternalServerError)
		return
	}
	layout.Boundaries = boundaries
	positionIsolatedCreatedUnits(layout, activeProposal)
	proposals, err := db.ListCurriculumProposals(server.Database, 100)
	if err != nil {
		log.Printf("load curriculum proposals: %v", err)
		http.Error(writer, "Unable to load curriculum proposals", http.StatusInternalServerError)
		return
	}
	draftProposals, err := db.ListDraftCurriculumProposalsByAuthor(server.Database, userID)
	if err != nil {
		log.Printf("load draft curriculum proposals: %v", err)
		http.Error(writer, "Unable to load draft curriculum proposals", http.StatusInternalServerError)
		return
	}
	draftViews := make([]curriculumDraftProposalView, 0, len(draftProposals))
	for index := range draftProposals {
		plan := rebasePlan
		if activeProposal == nil || activeProposal.ID != draftProposals[index].ID {
			plan, err = services.PlanCurriculumProposalRebase(server.Database, &draftProposals[index])
			if err != nil {
				log.Printf("plan draft curriculum proposal rebase: %v", err)
				http.Error(writer, "Unable to inspect draft proposals", http.StatusInternalServerError)
				return
			}
		}
		draftViews = append(draftViews, curriculumDraftProposalView{
			CurriculumProposal: draftProposals[index], RebaseStatus: plan.Status,
		})
	}
	history, rootDrafts := curriculumProposalHistory(proposals, draftViews)
	data := curriculumModificationPageData{
		userPageData: userPageData{
			User: user, CSRFToken: sessionCSRFToken(request), CurrentSection: "curriculum-modification",
		},
		Dependencies:        proposalBaseGraph.Dependencies,
		Graph:               layout,
		FocusedUnit:         focusedUnit,
		Proposals:           proposals,
		DraftProposals:      draftViews,
		ActiveProposal:      activeProposal,
		ProposalRebase:      rebasePlan,
		RebaseTimeline:      curriculumRebaseTimeline(rebasePlan, activeProposal),
		ReviewedProposal:    reviewedProposal,
		ProposalHistory:     history,
		RootDraftProposals:  rootDrafts,
		ShowProposalHistory: request.URL.Query().Get("history") == "1",
		CanEditProposal:     activeProposal != nil && (rebasePlan == nil || !rebasePlan.NeedsReview()),
		RecognitionSources:  proposalBaseGraph.Units,
		RecognitionTargets:  workingGraph.Units,
		Error:               message,
	}
	data.PublishWarning = curriculumRecognitionPublishWarning(activeProposal)
	data.Units = curriculumUnitViews(workingGraph, layout)
	if contentValue := request.URL.Query().Get("content"); contentValue != "" {
		contentID, parseErr := parsePositiveID(contentValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		for index := range data.Units {
			if data.Units[index].ID == contentID {
				data.ContentUnit = &data.Units[index]
				break
			}
		}
		if data.ContentUnit == nil {
			if historical := graphUnitByID(proposalBaseGraph, contentID); historical != nil {
				data.ContentUnit = &curriculumUnitView{Unit: *historical, Historical: true}
			} else {
				http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
				return
			}
		}
		applyUnitContentDiff(data.ContentUnit, activeProposal)
	}
	unitURL := func(unitID int64) string {
		target := "/curriculum-modification?"
		if activeProposal != nil {
			target += "proposal=" + strconv.FormatInt(activeProposal.ID, 10) + "&"
		}
		return target + "unit=" + strconv.FormatInt(unitID, 10)
	}
	navigateURL, contentURL := curriculumUnitURLs(unitURL, data.ContentUnit != nil)
	data.GraphView = newCurriculumGraphView(
		"curriculum-modification",
		"Arrows go from each prerequisite to the units that depend on it. Select a unit to navigate, or use its document action to open the content.",
		layout,
		focusedUnit,
		nil,
		completionStatuses,
		true,
		navigateURL,
		contentURL,
	)
	applyProposalGraphStates(&data.GraphView, activeProposal)
	data.GraphSearch = newUnitNavigationSearchView(
		"curriculum-graph-search-results",
		"Find a unit in the curriculum",
		workingGraph.Units,
		navigateURL,
	)
	server.renderStatus(writer, status, "curriculum-modification.html", data)
}
