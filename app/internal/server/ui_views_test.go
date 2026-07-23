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
		func(id int64) string { return "/units/" + strconv.FormatInt(id, 10) },
	)

	if len(view.Nodes) != 2 {
		t.Fatalf("node count = %d, want 2", len(view.Nodes))
	}
	if !view.Nodes[0].IsTarget || view.Nodes[0].IsCurrent {
		t.Fatalf("unexpected first node state: %#v", view.Nodes[0])
	}
	if !view.Nodes[1].IsCurrent || view.Nodes[1].URL != "/units/2" {
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
		func(int64) string { return "/learn?unit=1" },
	)
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, "curriculum-graph", view); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`id="test-graph-arrow"`,
		`href="/learn?unit=1"`,
		`aria-current="page"`,
		`aria-describedby="test-graph-description"`,
	} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("rendered graph does not contain %q", fragment)
		}
	}
}
