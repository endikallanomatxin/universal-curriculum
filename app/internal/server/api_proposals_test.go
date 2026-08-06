package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func TestAPIAdminAuthorizationUsesCurrentTokenOwnerPermission(t *testing.T) {
	called := false
	protected := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	adminCheck := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user := apiUser(request)
		if user == nil || !user.IsAdmin {
			writeAPIError(writer, http.StatusForbidden, "forbidden", "Administrator access is required.", nil)
			return
		}
		protected.ServeHTTP(writer, request)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/proposals", nil)
	request = request.WithContext(withAPIUser(request, &models.User{ID: 1}))
	response := httptest.NewRecorder()
	adminCheck.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("member admin check = %d, called %v", response.Code, called)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/proposals", nil)
	request = request.WithContext(withAPIUser(request, &models.User{ID: 1, IsAdmin: true}))
	response = httptest.NewRecorder()
	adminCheck.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !called {
		t.Fatalf("administrator check = %d, called %v", response.Code, called)
	}
}

func TestAPIRebasePlanKeepsConflictPublicationReferences(t *testing.T) {
	plan := newAPIRebasePlan(&services.CurriculumProposalRebasePlan{
		Status:            services.ProposalRebaseNeedsReview,
		AcceptedProposals: []models.CurriculumProposal{{ID: 4}, {ID: 7}},
		Conflicts: []services.CurriculumProposalRebaseConflict{{
			Change: models.CurriculumProposalChange{ID: 11},
			AcceptedWork: []services.CurriculumProposalRebaseAcceptedWork{
				{Proposal: models.CurriculumProposal{ID: 7}},
				{Proposal: models.CurriculumProposal{ID: 7}},
			},
		}},
	})
	if len(plan.AcceptedProposalIDs) != 2 || len(plan.Conflicts) != 1 ||
		len(plan.Conflicts[0].AcceptedProposalIDs) != 1 || plan.Conflicts[0].AcceptedProposalIDs[0] != 7 {
		t.Fatalf("rebase resource = %#v", plan)
	}
}
