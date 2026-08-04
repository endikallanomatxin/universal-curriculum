package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func TestAboutPageIsPublicAndUsesShellNavigation(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })

	templates, err := services.LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Templates: templates}
	request := httptest.NewRequest(http.MethodGet, "/about", nil)
	response := httptest.NewRecorder()

	server.routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, fragment := range []string{
		`<title>About · Universal Curriculum</title>`,
		`<h1 id="about-title">About</h1>`,
		`Universal Curriculum is a platform for the collaborative development of a free, publicly accessible curriculum.`,
		`href="/license"`,
		`href="/about" aria-current="page"`,
		`hx-get="/about"`,
	} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Errorf("rendered About page does not contain %q", fragment)
		}
	}
}

func TestLicensePageIsPublic(t *testing.T) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDirectory) })

	templates, err := services.LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Templates: templates}
	response := httptest.NewRecorder()
	server.routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/license", nil))
	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, fragment := range []string{`<title>License · Universal Curriculum</title>`, `Unless otherwise stated`, `provided that you give appropriate credit`, `creativecommons.org/licenses/by-sa/4.0/`, `is not covered by this license`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Errorf("response does not contain %q", fragment)
		}
	}
}

func TestHomeLearningRecommendationsListAvailableUnitsForIncompletePaths(t *testing.T) {
	graph := &models.CurriculumGraph{
		Units: []models.Unit{
			{ID: 1, Name: "Foundations"},
			{ID: 2, Name: "Algebra"},
			{ID: 3, Name: "Completed target"},
		},
		Dependencies: []models.UnitDependency{{UnitID: 2, PrerequisiteID: 1}},
	}
	paths := []models.LearningPath{
		{ID: 7, Name: "Mathematics", Units: []models.Unit{{ID: 2}}},
		{ID: 8, Name: "Finished", Units: []models.Unit{{ID: 3}}},
	}

	recommendations := newHomeLearningRecommendations(paths, graph, map[int64]bool{3: true})

	if len(recommendations) != 1 {
		t.Fatalf("recommendations = %#v, want one incomplete path", recommendations)
	}
	recommendation := recommendations[0]
	if recommendation.ID != 7 || recommendation.PendingCount != 2 ||
		recommendation.URL != "/learn?path=7" || len(recommendation.NextUnits) != 1 {
		t.Fatalf("unexpected path recommendation: %#v", recommendation)
	}
	next := recommendation.NextUnits[0]
	if next.ID != 1 || next.URL != "/learn?path=7&unit=1&content=1" {
		t.Fatalf("unexpected next unit: %#v", next)
	}
}
