package server

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"

	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func TestCurriculumGraphViewPreparesConsumerSpecificNodes(t *testing.T) {
	layout := &models.CurriculumGraphLayout{
		Nodes: []models.CurriculumGraphNode{
			{Unit: models.Unit{ID: 1, Name: "Foundations"}},
			{Unit: models.Unit{ID: 2, Name: "Algebra"}},
		},
	}
	focused := &models.Unit{ID: 2}
	view := newCurriculumGraphView(
		"test",
		"Description",
		layout,
		focused,
		map[int64]bool{1: true},
		map[int64]models.UnitCompletionStatus{2: {Direct: true}},
		true,
		func(id int64) string { return "/units/" + strconv.FormatInt(id, 10) },
		func(id int64) string { return "/units/" + strconv.FormatInt(id, 10) + "/content" },
	)

	if len(view.Nodes) != 2 {
		t.Fatalf("node count = %d, want 2", len(view.Nodes))
	}
	if !view.Nodes[0].IsTarget || view.Nodes[0].IsCurrent {
		t.Fatalf("unexpected first node state: %#v", view.Nodes[0])
	}
	if !view.Nodes[1].IsCurrent || !view.Nodes[1].IsCompleted || !view.Nodes[1].HasProgress ||
		view.Nodes[1].NavigateURL != "/units/2" ||
		view.Nodes[1].ContentURL != "/units/2/content" {
		t.Fatalf("unexpected second node state: %#v", view.Nodes[1])
	}
}

func TestUnitNavigationSearchViewPreparesURLs(t *testing.T) {
	view := newUnitNavigationSearchView(
		"results",
		"Find a unit",
		[]models.Unit{{ID: 7, Name: "Geometry"}},
		func(id int64) string { return "/unit/7" },
	)
	if len(view.Options) != 1 || view.Options[0].Name != "Geometry" || view.Options[0].URL != "/unit/7" {
		t.Fatalf("unexpected search view: %#v", view)
	}
}

func TestCurriculumUnitURLsPreserveContentViewerState(t *testing.T) {
	unitURL := func(id int64) string {
		return "/learn?path=all&unit=" + strconv.FormatInt(id, 10)
	}

	navigateURL, contentURL := curriculumUnitURLs(unitURL, false)
	if got := navigateURL(7); got != "/learn?path=all&unit=7" {
		t.Fatalf("closed-viewer navigation URL = %q", got)
	}
	if got := contentURL(7); got != "/learn?path=all&unit=7&content=7" {
		t.Fatalf("explicit content URL = %q", got)
	}

	navigateURL, _ = curriculumUnitURLs(unitURL, true)
	if got := navigateURL(8); got != "/learn?path=all&unit=8&content=8" {
		t.Fatalf("open-viewer navigation URL = %q", got)
	}
}

func TestCombinedLearningPathTargetsDeduplicatesUnits(t *testing.T) {
	ids, targets := combinedLearningPathTargets([]models.LearningPath{
		{Units: []models.Unit{{ID: 1}, {ID: 2}}},
		{Units: []models.Unit{{ID: 2}, {ID: 3}}},
	})

	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("combined target ids = %v, want [1 2 3]", ids)
	}
	for _, id := range ids {
		if !targets[id] {
			t.Errorf("combined target map is missing %d", id)
		}
	}
}

func TestSharedCurriculumGraphTemplateRendersPreparedView(t *testing.T) {
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
	view := newCurriculumGraphView(
		"test-graph",
		"Graph description",
		&models.CurriculumGraphLayout{
			Nodes: []models.CurriculumGraphNode{{Unit: models.Unit{ID: 1, Name: "Foundations"}}},
		},
		&models.Unit{ID: 1},
		nil,
		map[int64]models.UnitCompletionStatus{1: {Direct: true}},
		true,
		func(int64) string { return "/learn?unit=1" },
		func(int64) string { return "/learn?unit=1&content=1" },
	)
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, "curriculum-graph", view); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`id="test-graph-arrow"`,
		`id="test-graph-arrow-proposed"`,
		`href="/learn?unit=1"`,
		`href="/learn?unit=1&amp;content=1"`,
		`aria-label="Open content for Foundations"`,
		`aria-current="page"`,
		`aria-describedby="test-graph-description"`,
		`has-progress`,
		`is-completed`,
	} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("rendered graph does not contain %q", fragment)
		}
	}
}
