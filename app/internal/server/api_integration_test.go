package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"universal-curriculum/internal/db/migrations"
	"universal-curriculum/internal/services"
)

func TestExperimentalAPIEndToEnd(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, err := services.RegisterLocalUser(
		database, "API Administrator", "api-admin@example.com", "long-enough-password",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE users SET is_admin = TRUE WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	token, err := services.CreateAPIToken(database, user.ID, "Integration test")
	if err != nil {
		t.Fatal(err)
	}
	var rawTokenRows int
	if err := database.QueryRow(`SELECT count(*) FROM api_tokens WHERE token_hash = $1`, token.Token).Scan(&rawTokenRows); err != nil {
		t.Fatal(err)
	}
	if rawTokenRows != 0 {
		t.Fatal("raw API token was persisted")
	}
	if _, err := database.Exec(`
		CREATE FUNCTION reject_api_token_last_used_update() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'last_used_at unavailable';
		END;
		$$
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		CREATE TRIGGER api_token_last_used_update_fails
		BEFORE UPDATE OF last_used_at ON api_tokens
		FOR EACH ROW EXECUTE FUNCTION reject_api_token_last_used_update()
	`); err != nil {
		t.Fatal(err)
	}

	application := (&Server{Database: database}).routes()
	proposalResponse := apiIntegrationRequest(t, application, token.Token, http.MethodPost, "/api/proposals", map[string]any{
		"title": "API curriculum", "rationale": "Exercise the complete API workflow.",
	}, http.StatusCreated)
	var proposal struct {
		ID int64 `json:"id"`
	}
	decodeAPIIntegrationResponse(t, proposalResponse, &proposal)

	unitResponse := apiIntegrationRequest(t, application, token.Token, http.MethodPost,
		fmt.Sprintf("/api/proposals/%d/units", proposal.ID), map[string]any{
			"name": "API design", "content": "Learn how to design an API.",
		}, http.StatusCreated)
	var unit struct {
		ID int64 `json:"id"`
	}
	decodeAPIIntegrationResponse(t, unitResponse, &unit)
	prerequisiteResponse := apiIntegrationRequest(t, application, token.Token, http.MethodPost,
		fmt.Sprintf("/api/proposals/%d/units", proposal.ID), map[string]any{
			"name": "API foundations", "content": "Learn the foundations of API design.",
		}, http.StatusCreated)
	var prerequisite struct {
		ID int64 `json:"id"`
	}
	decodeAPIIntegrationResponse(t, prerequisiteResponse, &prerequisite)
	apiIntegrationRequest(t, application, token.Token, http.MethodPut,
		fmt.Sprintf("/api/proposals/%d/units/%d", proposal.ID, unit.ID), map[string]any{
			"name": "Practical API design", "content": "Learn how to design a practical API.",
		}, http.StatusOK)
	apiIntegrationRequest(t, application, token.Token, http.MethodPut,
		fmt.Sprintf("/api/proposals/%d/units/%d", proposal.ID, unit.ID), map[string]any{
			"name": "Final API design", "content": "Learn how to design the final API.",
		}, http.StatusOK)
	apiDraftResponse := apiIntegrationRequest(
		t, application, token.Token, http.MethodGet,
		fmt.Sprintf("/api/proposals/%d", proposal.ID), nil, http.StatusOK,
	)
	var apiDraft apiProposal
	decodeAPIIntegrationResponse(t, apiDraftResponse, &apiDraft)
	var apiCreationCount int
	for _, change := range *apiDraft.Changes {
		if change.UnitID == nil || *change.UnitID != unit.ID {
			continue
		}
		if change.Kind != "create_unit" || change.UnitName != "Final API design" ||
			change.UnitContent != "Learn how to design the final API." {
			t.Fatalf("API unit update left edit-history changes: %#v", *apiDraft.Changes)
		}
		apiCreationCount++
	}
	if apiCreationCount != 1 {
		t.Fatalf("API unit creation count = %d, changes = %#v", apiCreationCount, *apiDraft.Changes)
	}
	apiIntegrationRequest(t, application, token.Token, http.MethodPost,
		fmt.Sprintf("/api/proposals/%d/dependencies", proposal.ID), map[string]any{
			"unit_id": unit.ID, "prerequisite_id": prerequisite.ID,
		}, http.StatusNoContent)
	apiIntegrationRequest(t, application, token.Token, http.MethodDelete,
		fmt.Sprintf("/api/proposals/%d/dependencies/%d/%d", proposal.ID, unit.ID, prerequisite.ID),
		nil, http.StatusNoContent)
	apiIntegrationRequest(t, application, token.Token, http.MethodPost,
		fmt.Sprintf("/api/proposals/%d/dependencies", proposal.ID), map[string]any{
			"unit_id": unit.ID, "prerequisite_id": prerequisite.ID,
		}, http.StatusNoContent)

	apiIntegrationRequest(t, application, token.Token, http.MethodPost,
		fmt.Sprintf("/api/proposals/%d/submit", proposal.ID), nil, http.StatusOK)
	apiIntegrationRequest(t, application, token.Token, http.MethodPost,
		fmt.Sprintf("/api/proposals/%d/accept", proposal.ID), nil, http.StatusOK)
	curriculumResponse := apiIntegrationRequest(t, application, "", http.MethodGet, "/api/curriculum", nil, http.StatusOK)
	var curriculumOverview struct {
		Units []map[string]any `json:"units"`
	}
	decodeAPIIntegrationResponse(t, curriculumResponse, &curriculumOverview)
	for _, summary := range curriculumOverview.Units {
		if _, present := summary["content"]; present {
			t.Fatalf("curriculum overview exposed unit content: %#v", summary)
		}
	}
	searchResponse := apiIntegrationRequest(t, application, "", http.MethodGet,
		"/api/units?query=design+the+final", nil, http.StatusOK)
	var searchResult struct {
		Units []map[string]any `json:"units"`
	}
	decodeAPIIntegrationResponse(t, searchResponse, &searchResult)
	if len(searchResult.Units) != 1 || searchResult.Units[0]["id"] != float64(unit.ID) {
		t.Fatalf("content search summaries = %#v", searchResult.Units)
	}
	if _, present := searchResult.Units[0]["content"]; present {
		t.Fatalf("content search exposed unit content: %#v", searchResult.Units[0])
	}
	unitDetailResponse := apiIntegrationRequest(t, application, "", http.MethodGet,
		fmt.Sprintf("/api/units/%d", unit.ID), nil, http.StatusOK)
	var unitDetail apiUnit
	decodeAPIIntegrationResponse(t, unitDetailResponse, &unitDetail)
	if unitDetail.Content != "Learn how to design the final API." ||
		len(unitDetail.PrerequisiteIDs) != 1 || unitDetail.PrerequisiteIDs[0] != prerequisite.ID {
		t.Fatalf("unit detail = %#v", unitDetail)
	}
	apiIntegrationRequest(t, application, token.Token, http.MethodPost, "/api/learning-paths", map[string]any{
		"name": "API path", "target_unit_ids": []int64{unit.ID},
	}, http.StatusCreated)
	progressResponse := apiIntegrationRequest(t, application, token.Token, http.MethodPut,
		fmt.Sprintf("/api/progress/%d", unit.ID), map[string]any{"completed": true}, http.StatusOK)
	var progress apiProgress
	decodeAPIIntegrationResponse(t, progressResponse, &progress)
	if !progress.Direct || !progress.Completed {
		t.Fatalf("completion response = %#v", progress)
	}

	if err := services.RevokeAPIToken(database, user.ID, token.ID); err != nil {
		t.Fatal(err)
	}
	var remainingTokens int
	if err := database.QueryRow(`SELECT count(*) FROM api_tokens WHERE id = $1`, token.ID).Scan(&remainingTokens); err != nil {
		t.Fatal(err)
	}
	if remainingTokens != 0 {
		t.Fatal("revoked API token was not deleted")
	}
	apiIntegrationRequest(t, application, token.Token, http.MethodGet, "/api/progress", nil, http.StatusUnauthorized)
}

func openAPIIntegrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL API integration tests")
	}
	database, err := sql.Open("postgres", connectionString)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	schema := fmt.Sprintf("api_test_%d", time.Now().UnixNano())
	if _, err := database.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
		_ = database.Close()
	})
	if _, err := database.Exec("SET search_path TO " + schema); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func apiIntegrationRequest(
	t *testing.T,
	application http.Handler,
	token, method, target string,
	body any,
	wantStatus int,
) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, target, &encoded)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	application.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s = %d, want %d: %s", method, target, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func decodeAPIIntegrationResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
