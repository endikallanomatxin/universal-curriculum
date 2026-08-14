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

const (
	curriculumProposalHistoryPageSize = 10
	curriculumProposalListPageSize    = 25
)

var errInvalidProposalListLimit = errors.New("invalid proposal list limit")

func (server *Server) curriculumModification(writer http.ResponseWriter, request *http.Request) {
	server.renderCurriculumModification(writer, request, http.StatusOK, "")
}

func (server *Server) renderCurriculumModification(writer http.ResponseWriter, request *http.Request, status int, message string) {
	showProposalHistory := request.URL.Query().Get("history") == "1"
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
			activeProposal, err = services.GetVisibleCurriculumProposal(server.Database, userID, user.IsAdmin, proposalID)
			if err != nil && !errors.Is(err, services.ErrProposalNotFound) {
				log.Printf("load active curriculum proposal: %v", err)
				http.Error(writer, "Unable to load curriculum proposal", http.StatusInternalServerError)
				return
			}
			if errors.Is(err, services.ErrProposalNotFound) {
				activeProposal = nil
			}
		}
	}
	var rebasePlan *services.CurriculumProposalRebasePlan
	proposalBaseGraph := graph
	if activeProposal != nil && activeProposal.Status == "draft" {
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
	} else if activeProposal != nil {
		proposalBaseGraph, err = services.CurriculumGraphAtProposal(server.Database, activeProposal.BaseProposalID)
		if err != nil {
			log.Printf("load proposal base curriculum: %v", err)
			http.Error(writer, "Unable to load curriculum proposal base", http.StatusInternalServerError)
			return
		}
		services.PopulateCurriculumProposalPreviousState(proposalBaseGraph, activeProposal)
	}
	var reviewedProposal *models.CurriculumProposal
	var reviewedBaseGraph, reviewedWorkingGraph *models.CurriculumGraph
	if reviewedValue := request.URL.Query().Get("review-proposal"); reviewedValue != "" {
		reviewedID, parseErr := parsePositiveID(reviewedValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum proposal", http.StatusBadRequest)
			return
		}
		if showProposalHistory {
			reviewedProposal, err = db.GetCurriculumProposal(server.Database, reviewedID)
			if err != nil {
				log.Printf("load accepted curriculum proposal: %v", err)
				http.Error(writer, "Unable to load accepted curriculum proposal", http.StatusInternalServerError)
				return
			}
			if reviewedProposal != nil && reviewedProposal.Status != "accepted" {
				reviewedProposal = nil
			}
		} else {
			reviewedProposal = visibleRebaseProposal(rebasePlan, reviewedID)
		}
		if reviewedProposal == nil {
			http.Error(writer, "Accepted curriculum proposal not found", http.StatusNotFound)
			return
		}
		reviewedGraph, graphErr := services.CurriculumGraphAtProposal(server.Database, &reviewedProposal.ID)
		if graphErr != nil {
			log.Printf("load related proposal curriculum: %v", graphErr)
			http.Error(writer, "Unable to load related curriculum proposal", http.StatusInternalServerError)
			return
		}
		applyCurriculumChangeLabels(reviewedProposal, reviewedGraph)
		if showProposalHistory {
			reviewedBaseGraph, err = services.CurriculumGraphAtProposal(server.Database, reviewedProposal.BaseProposalID)
			if err != nil {
				log.Printf("load accepted proposal base curriculum: %v", err)
				http.Error(writer, "Unable to load accepted proposal base", http.StatusInternalServerError)
				return
			}
			reviewedWorkingGraph = reviewedGraph
		}
	}
	workingGraph := curriculumGraphWithProposal(proposalBaseGraph, activeProposal)
	graphProposal := activeProposal
	if reviewedWorkingGraph != nil {
		proposalBaseGraph = reviewedBaseGraph
		workingGraph = reviewedWorkingGraph
		graphProposal = reviewedProposal
	}
	applyCurriculumChangeLabels(graphProposal, workingGraph)
	visualGraph := curriculumGraphWithRemovedDependencies(workingGraph, proposalBaseGraph, graphProposal)
	var focusID *int64
	if unitValue := request.URL.Query().Get("unit"); unitValue != "" {
		unitID, parseErr := parsePositiveID(unitValue)
		if parseErr != nil {
			http.Error(writer, "Invalid curriculum unit", http.StatusBadRequest)
			return
		}
		focusID = &unitID
	}
	visibleGraph, focusedUnit, boundaries, err := services.CurriculumProposalNeighborhood(visualGraph, graphProposal, focusID)
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
	positionIsolatedCreatedUnits(layout, graphProposal)
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
			ChangeSummary: curriculumProposalChangeSummary(draftProposals[index].ChangeKindCounts),
		})
	}
	activeLimit, err := growingProposalLimit(request, "active-limit")
	if err != nil {
		http.Error(writer, "Invalid active proposal limit", http.StatusBadRequest)
		return
	}
	activeProposals, activeTotal, err := db.ListSubmittedCurriculumProposals(server.Database, activeLimit, 0)
	if err != nil {
		log.Printf("load active proposals: %v", err)
		http.Error(writer, "Unable to load active proposals", http.StatusInternalServerError)
		return
	}
	reviewedLimit, err := growingProposalLimit(request, "reviewed-limit")
	if err != nil {
		http.Error(writer, "Invalid reviewed proposal limit", http.StatusBadRequest)
		return
	}
	reviewedProposals, reviewedTotal, err := db.ListRejectedCurriculumProposalsByAuthor(server.Database, userID, reviewedLimit, 0)
	if err != nil {
		log.Printf("load reviewed proposals: %v", err)
		http.Error(writer, "Unable to load reviewed proposals", http.StatusInternalServerError)
		return
	}
	historyLimit := curriculumProposalHistoryPageSize
	if value := request.URL.Query().Get("history-limit"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < curriculumProposalHistoryPageSize {
			http.Error(writer, "Invalid proposal history limit", http.StatusBadRequest)
			return
		}
		historyLimit = parsed
	}
	var history []curriculumProposalHistoryView
	var rootDrafts []curriculumDraftProposalView
	historyHasMore := false
	if showProposalHistory {
		accepted, total, listErr := db.ListAcceptedCurriculumProposals(server.Database, historyLimit, 0)
		if listErr != nil {
			log.Printf("load curriculum proposal history: %v", listErr)
			http.Error(writer, "Unable to load curriculum proposal history", http.StatusInternalServerError)
			return
		}
		history, rootDrafts = curriculumProposalHistory(accepted, draftViews)
		historyHasMore = len(accepted) < total
	}
	data := curriculumModificationPageData{
		userPageData: userPageData{
			User: user, CSRFToken: sessionCSRFToken(request), CurrentSection: "curriculum-modification",
		},
		Dependencies:            proposalBaseGraph.Dependencies,
		Graph:                   layout,
		FocusedUnit:             focusedUnit,
		DraftProposals:          draftViews,
		ReviewedProposals:       reviewedProposals,
		ActiveProposals:         activeProposals,
		ActiveProposalTotal:     activeTotal,
		ActiveProposalMore:      len(activeProposals) < activeTotal,
		ActiveProposalLimit:     activeLimit,
		ActiveProposalNext:      activeLimit + curriculumProposalListPageSize,
		ActiveProposal:          activeProposal,
		ReviewedProposalTotal:   reviewedTotal,
		ReviewedProposalMore:    len(reviewedProposals) < reviewedTotal,
		ReviewedProposalLimit:   reviewedLimit,
		ReviewedProposalNext:    reviewedLimit + curriculumProposalListPageSize,
		ProposalRebase:          rebasePlan,
		RebaseTimeline:          curriculumRebaseTimeline(rebasePlan, activeProposal),
		ReviewedProposal:        reviewedProposal,
		ProposalHistory:         history,
		RootDraftProposals:      rootDrafts,
		ShowProposalHistory:     showProposalHistory,
		ProposalHistoryMore:     historyHasMore,
		ProposalHistoryLimit:    historyLimit,
		ProposalHistoryNext:     historyLimit + curriculumProposalHistoryPageSize,
		CanEditProposal:         activeProposal != nil && activeProposal.Status == "draft" && (rebasePlan == nil || !rebasePlan.NeedsReview()),
		ViewingAcceptedProposal: reviewedWorkingGraph != nil,
		RecognitionSources:      proposalBaseGraph.Units,
		RecognitionTargets:      workingGraph.Units,
		Error:                   message,
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
			if historical := proposalBaseGraph.Unit(contentID); historical != nil {
				data.ContentUnit = &curriculumUnitView{Unit: *historical, Historical: true}
			} else {
				http.Error(writer, "Curriculum unit not found", http.StatusNotFound)
				return
			}
		}
		applyUnitContentDiff(data.ContentUnit, graphProposal)
	}
	graphQuery := ""
	if activeProposal != nil {
		graphQuery = "proposal=" + strconv.FormatInt(activeProposal.ID, 10)
	} else if reviewedWorkingGraph != nil {
		graphQuery = "history=1&history-limit=" + strconv.Itoa(historyLimit) +
			"&review-proposal=" + strconv.FormatInt(reviewedProposal.ID, 10)
	}
	data.GraphURL = "/curriculum-modification"
	if graphQuery != "" {
		data.GraphURL += "?" + graphQuery
	}
	unitURL := func(unitID int64) string {
		target := data.GraphURL
		if graphQuery == "" {
			target += "?"
		} else {
			target += "&"
		}
		return target + "unit=" + strconv.FormatInt(unitID, 10)
	}
	if data.ContentUnit != nil {
		data.UnitContentCloseURL = unitURL(data.ContentUnit.ID)
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
	applyProposalGraphStates(&data.GraphView, graphProposal)
	data.GraphSearch = newUnitNavigationSearchView(
		"curriculum-graph-search-results",
		"Find a unit in the curriculum",
		workingGraph.Units,
		navigateURL,
	)
	server.renderStatus(writer, status, "curriculum-modification.html", data)
}

func growingProposalLimit(request *http.Request, name string) (int, error) {
	limit := curriculumProposalListPageSize
	if value := request.URL.Query().Get(name); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < curriculumProposalListPageSize {
			return 0, errInvalidProposalListLimit
		}
		limit = parsed
	}
	return limit, nil
}
