package server

import (
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/server/views"
	"universal-curriculum/internal/services"
)

func TestCurriculumModificationInitialRenderDoesNotLoadAllContentOrPickerOptions(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, unit := createCurriculumModificationFixture(t, database)
	draft, err := services.CreateCurriculumProposal(database, user.ID, "Working draft", "Render the normal editor workspace.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`ALTER TABLE units RENAME COLUMN content TO unavailable_content`); err != nil {
		t.Fatal(err)
	}
	templates := loadCurriculumModificationIntegrationTemplates(t)
	server := &Server{Database: database, Templates: templates}
	target := "/curriculum-modification?proposal=" + strconv.FormatInt(draft.ID, 10)
	request := services.WithSession(httptest.NewRequest(http.MethodGet, target, nil), user.ID, "csrf")
	response := httptest.NewRecorder()

	server.curriculumModification(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("curriculum modification status = %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, unit.Name) {
		t.Fatalf("lightweight graph omitted unit name: %s", body)
	}
	if strings.Contains(body, "data-unit-picker-option") {
		t.Fatal("initial curriculum modification HTML contained eager picker options")
	}
}

func TestCurriculumModificationDraftListDoesNotPlanEveryStaleRebase(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, _ := createCurriculumModificationFixture(t, database)
	stale, err := services.CreateCurriculumProposal(database, user.ID, "Stale draft", "List without planning its full rebase.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.CreateCurriculumUnit(database, user.ID, stale.ID, "Stale addition", "Draft content."); err != nil {
		t.Fatal(err)
	}
	advance, err := services.CreateCurriculumProposal(database, user.ID, "Advance curriculum", "Make the other draft outdated.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.CreateCurriculumUnit(database, user.ID, advance.ID, "Published addition", "Published content."); err != nil {
		t.Fatal(err)
	}
	if err := services.SubmitCurriculumProposal(database, user.ID, advance.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := services.AcceptCurriculumProposal(database, advance.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`ALTER TABLE units RENAME COLUMN content TO unavailable_content`); err != nil {
		t.Fatal(err)
	}

	server := &Server{Database: database, Templates: loadCurriculumModificationIntegrationTemplates(t)}
	request := services.WithSession(httptest.NewRequest(http.MethodGet, "/curriculum-modification", nil), user.ID, "csrf")
	response := httptest.NewRecorder()
	server.curriculumModification(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Stale draft") ||
		!strings.Contains(response.Body.String(), "Outdated") {
		t.Fatalf("stale draft list response = %d: %s", response.Code, response.Body.String())
	}
}

func TestCurriculumModificationLoadsOnlyOpenedUnitContent(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, unit := createCurriculumModificationFixture(t, database)
	templates := loadCurriculumModificationIntegrationTemplates(t)
	server := &Server{Database: database, Templates: templates}
	target := "/curriculum-modification?unit=" + strconv.FormatInt(unit.ID, 10) + "&content=" + strconv.FormatInt(unit.ID, 10)
	request := services.WithSession(httptest.NewRequest(http.MethodGet, target, nil), user.ID, "csrf")
	response := httptest.NewRecorder()

	server.curriculumModification(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Focused published content.") {
		t.Fatalf("opened unit response = %d: %s", response.Code, response.Body.String())
	}
}

func TestCurriculumModificationLoadsProposalContentAndFocusedDiff(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, unit := createCurriculumModificationFixture(t, database)
	draft, err := services.CreateCurriculumProposal(database, user.ID, "Content draft", "Exercise focused proposal content.")
	if err != nil {
		t.Fatal(err)
	}
	if err := services.UpdateCurriculumUnitContent(database, user.ID, draft.ID, unit.ID, "Focused proposed content."); err != nil {
		t.Fatal(err)
	}
	created, err := services.CreateCurriculumUnit(database, user.ID, draft.ID, "Draft-only unit", "Draft-only content.")
	if err != nil {
		t.Fatal(err)
	}
	templates := loadCurriculumModificationIntegrationTemplates(t)
	server := &Server{Database: database, Templates: templates}

	for _, test := range []struct {
		unitID int64
		want   []string
	}{
		{unitID: unit.ID, want: []string{"Focused proposed content.", "Focused published content.", "Proposed content changes"}},
		{unitID: created.ID, want: []string{"Draft-only unit", "Draft-only content."}},
	} {
		target := "/curriculum-modification?proposal=" + strconv.FormatInt(draft.ID, 10) +
			"&unit=" + strconv.FormatInt(test.unitID, 10) + "&content=" + strconv.FormatInt(test.unitID, 10)
		request := services.WithSession(httptest.NewRequest(http.MethodGet, target, nil), user.ID, "csrf")
		response := httptest.NewRecorder()
		server.curriculumModification(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("proposal content status = %d: %s", response.Code, response.Body.String())
		}
		for _, want := range test.want {
			if !strings.Contains(response.Body.String(), want) {
				t.Fatalf("proposal content omitted %q: %s", want, response.Body.String())
			}
		}
	}
}

func TestCurriculumModificationUnitSearchReturnsLazyProposalResults(t *testing.T) {
	database := openAPIIntegrationDatabase(t)
	user, _ := createCurriculumModificationFixture(t, database)
	draft, err := services.CreateCurriculumProposal(database, user.ID, "Search draft", "Search the working curriculum.")
	if err != nil {
		t.Fatal(err)
	}
	created, err := services.CreateCurriculumUnit(database, user.ID, draft.ID, "Draft search result", "Draft content.")
	if err != nil {
		t.Fatal(err)
	}
	templates := loadCurriculumModificationIntegrationTemplates(t)
	server := &Server{Database: database, Templates: templates}

	target := "/curriculum-modification/unit-search?scope=recognition-target&proposal_id=" +
		strconv.FormatInt(draft.ID, 10) + "&q=draft+search"
	request := services.WithSession(httptest.NewRequest(http.MethodGet, target, nil), user.ID, "csrf")
	response := httptest.NewRecorder()
	server.curriculumModificationUnitSearch(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `data-unit-id="`+strconv.FormatInt(created.ID, 10)+`"`) {
		t.Fatalf("lazy search response = %d: %s", response.Code, response.Body.String())
	}
	dependencyTarget := "/curriculum-modification/unit-search?scope=dependency&proposal_id=" +
		strconv.FormatInt(draft.ID, 10) + "&unit_id=" + strconv.FormatInt(created.ID, 10) + "&q=draft+search"
	dependencyRequest := services.WithSession(httptest.NewRequest(http.MethodGet, dependencyTarget, nil), user.ID, "csrf")
	dependencyResponse := httptest.NewRecorder()
	server.curriculumModificationUnitSearch(dependencyResponse, dependencyRequest)
	if dependencyResponse.Code != http.StatusOK || strings.Contains(dependencyResponse.Body.String(), `data-unit-id="`+strconv.FormatInt(created.ID, 10)+`"`) {
		t.Fatalf("dependency self-filter response = %d: %s", dependencyResponse.Code, dependencyResponse.Body.String())
	}

	emptyTarget := "/curriculum-modification/unit-search?scope=recognition-target&proposal_id=" + strconv.FormatInt(draft.ID, 10)
	emptyRequest := services.WithSession(httptest.NewRequest(http.MethodGet, emptyTarget, nil), user.ID, "csrf")
	emptyResponse := httptest.NewRecorder()
	server.curriculumModificationUnitSearch(emptyResponse, emptyRequest)
	if emptyResponse.Code != http.StatusOK || strings.Contains(emptyResponse.Body.String(), "data-unit-picker-option") {
		t.Fatalf("empty lazy search response = %d: %s", emptyResponse.Code, emptyResponse.Body.String())
	}
}

func loadCurriculumModificationIntegrationTemplates(t *testing.T) *template.Template {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	templates, loadErr := views.LoadTemplates()
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	return templates
}

func createCurriculumModificationFixture(t *testing.T, database *sql.DB) (*models.User, *models.Unit) {
	t.Helper()
	user, err := services.RegisterLocalUser(database, "Curriculum Editor", "editor@example.com", "long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE users SET is_contributor = TRUE, is_admin = TRUE WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	proposal, err := services.CreateCurriculumProposal(database, user.ID, "Initial curriculum", "Create the fixture curriculum.")
	if err != nil {
		t.Fatal(err)
	}
	unit, err := services.CreateCurriculumUnit(database, user.ID, proposal.ID, "Focused unit", "Focused published content.")
	if err != nil {
		t.Fatal(err)
	}
	if err := services.SubmitCurriculumProposal(database, user.ID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := services.AcceptCurriculumProposal(database, proposal.ID); err != nil {
		t.Fatal(err)
	}
	return user, unit
}
