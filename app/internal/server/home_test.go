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
		`href="/about/case"`,
		`href="/about/proposal"`,
		`href="/about/documentation"`,
		`href="/license"`,
		`href="/about" aria-current="page"`,
		`hx-get="/about"`,
	} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Errorf("rendered About page does not contain %q", fragment)
		}
	}
}

func TestDocumentationPagesUseCanonicalContentAndNestedPanels(t *testing.T) {
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

	index := httptest.NewRecorder()
	server.routes().ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/about/documentation", nil))
	if index.Code != http.StatusOK {
		t.Fatalf("documentation status = %d", index.Code)
	}
	for _, fragment := range []string{`<h1 id="about-title">About</h1>`, `<h1 id="documentation-title">Documentation</h1>`, `href="/about/documentation/curriculum-units"`, `href="/about/documentation/mcp-api"`} {
		if !strings.Contains(index.Body.String(), fragment) {
			t.Errorf("documentation index does not contain %q", fragment)
		}
	}

	page := httptest.NewRecorder()
	server.routes().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/about/documentation/writing-content", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("documentation page status = %d", page.Code)
	}
	for _, fragment := range []string{`<h1 id="documentation-page-title">Writing content</h1>`, `Content supports Markdown`, `$...$`, `$$...$$`} {
		if !strings.Contains(page.Body.String(), fragment) {
			t.Errorf("documentation page does not contain %q", fragment)
		}
	}

	missing := httptest.NewRecorder()
	server.routes().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/about/documentation/unknown", nil))
	if missing.Code != http.StatusNotFound {
		t.Errorf("unknown documentation status = %d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestAboutDocumentsArePublic(t *testing.T) {
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
	for _, test := range []struct {
		path      string
		fragments []string
	}{
		{path: "/about/case", fragments: []string{`<title>The case for a shared curriculum · Universal Curriculum</title>`, `Access to information is not access to education`, `Educational work does not accumulate as much as it could`, `Learning should not be shaped around certification`, `Qualifications lack a common reference`, `Education in a time of accelerating technological change`}},
		{path: "/about/proposal", fragments: []string{`<title>The approach · Universal Curriculum</title>`, `Curriculum structure`, `Stewardship`}},
		{path: "/license", fragments: []string{`<title>License · Universal Curriculum</title>`, `Unless otherwise stated`, `provided that you give appropriate credit`, `creativecommons.org/licenses/by-sa/4.0/`, `is not covered by this license`}},
	} {
		response := httptest.NewRecorder()
		server.routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", test.path, response.Code, http.StatusOK)
		}
		for _, fragment := range test.fragments {
			if !strings.Contains(response.Body.String(), fragment) {
				t.Errorf("GET %s response does not contain %q", test.path, fragment)
			}
		}
	}
}

func TestOldManifestURLRedirectsToCase(t *testing.T) {
	response := httptest.NewRecorder()
	(&Server{}).routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/about/manifest", nil))

	if response.Code != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPermanentRedirect)
	}
	if location := response.Header().Get("Location"); location != "/about/case" {
		t.Fatalf("Location = %q, want /about/case", location)
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
