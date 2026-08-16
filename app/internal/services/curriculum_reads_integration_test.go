package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

func TestCurriculumProposalUnitResolvesFocusedHistoricalContent(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "focused_unit_history")
	var authorID int64
	if err := database.QueryRow(`INSERT INTO users (full_name, is_admin) VALUES ('History Editor', TRUE) RETURNING id`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	base, err := CreateCurriculumProposal(database, authorID, "Base", "Create the original unit.")
	if err != nil {
		t.Fatal(err)
	}
	unit, err := CreateCurriculumUnit(database, authorID, base.ID, "Memoization", "Original content.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, base.ID); err != nil {
		t.Fatal(err)
	}

	revision, err := CreateCurriculumProposal(database, authorID, "Revision", "Revise and extend the curriculum.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, revision.ID, unit.ID, "Revised content."); err != nil {
		t.Fatal(err)
	}
	created, err := CreateCurriculumUnit(database, authorID, revision.ID, "Cache keys", "Created in the revision.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, revision.ID); err != nil {
		t.Fatal(err)
	}
	revision, err = db.GetCurriculumProposal(database, revision.ID)
	if err != nil {
		t.Fatal(err)
	}

	later, err := CreateCurriculumProposal(database, authorID, "Later", "Change the current projection again.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, later.ID, unit.ID, "Current content."); err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, later.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`ALTER TABLE units RENAME COLUMN content TO unavailable_content`); err != nil {
		t.Fatal(err)
	}

	resolved, previous, historical, err := CurriculumProposalUnit(context.Background(), database, revision, unit.ID)
	if err != nil || resolved == nil || resolved.Content != "Revised content." || previous == nil || previous.Content != "Original content." || historical {
		t.Fatalf("historical revised unit = %#v previous=%#v historical=%v err=%v", resolved, previous, historical, err)
	}
	resolved, previous, historical, err = CurriculumProposalUnit(context.Background(), database, revision, created.ID)
	if err != nil || resolved == nil || resolved.Content != "Created in the revision." || previous != nil || historical {
		t.Fatalf("historical created unit = %#v previous=%#v historical=%v err=%v", resolved, previous, historical, err)
	}
}

func TestSearchCurriculumProposalUnitsUsesBaseAndWorkingStates(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "proposal_unit_search")
	var authorID int64
	if err := database.QueryRow(`INSERT INTO users (full_name, is_admin) VALUES ('Search Editor', TRUE) RETURNING id`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	base, err := CreateCurriculumProposal(database, authorID, "Search base", "Create searchable units.")
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := CreateCurriculumUnit(database, authorID, base.ID, "Shared old concept", "Old.")
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := CreateCurriculumUnit(database, authorID, base.ID, "Shared original name", "Original.")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 25; index++ {
		if _, err := CreateCurriculumUnit(database, authorID, base.ID, fmt.Sprintf("Bounded match %02d", index), "Content."); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, base.ID); err != nil {
		t.Fatal(err)
	}
	draft, err := CreateCurriculumProposal(database, authorID, "Working search", "Change search membership.")
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteCurriculumUnit(database, authorID, draft.ID, deleted.ID); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, draft.ID, renamed.ID, "Shared renamed concept"); err != nil {
		t.Fatal(err)
	}
	created, err := CreateCurriculumUnit(database, authorID, draft.ID, "Shared created concept", "Created.")
	if err != nil {
		t.Fatal(err)
	}
	draft, err = db.GetCurriculumProposal(database, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	historical, previous, isHistorical, err := CurriculumProposalUnit(context.Background(), database, draft, deleted.ID)
	if err != nil || historical == nil || historical.Content != "Old." || previous == nil || !isHistorical {
		t.Fatalf("deleted proposal unit = %#v previous=%#v historical=%v err=%v", historical, previous, isHistorical, err)
	}

	sources, err := SearchCurriculumProposalUnits(context.Background(), database, draft, CurriculumUnitSearchRecognitionSource, "shared", 20)
	if err != nil || !unitIDsContain(sources, deleted.ID) || !unitIDsContain(sources, renamed.ID) || unitIDsContain(sources, created.ID) {
		t.Fatalf("recognition sources = %#v err=%v", sources, err)
	}
	targets, err := SearchCurriculumProposalUnits(context.Background(), database, draft, CurriculumUnitSearchRecognitionTarget, "shared", 20)
	if err != nil || unitIDsContain(targets, deleted.ID) || !unitIDsContain(targets, renamed.ID) || !unitIDsContain(targets, created.ID) {
		t.Fatalf("recognition targets = %#v err=%v", targets, err)
	}
	dependencies, err := SearchCurriculumProposalUnits(context.Background(), database, draft, CurriculumUnitSearchDependency, "shared", 20)
	if err != nil || unitIDsContain(dependencies, deleted.ID) || !unitIDsContain(dependencies, created.ID) {
		t.Fatalf("dependency candidates = %#v err=%v", dependencies, err)
	}
	bounded, err := SearchCurriculumProposalUnits(context.Background(), database, draft, CurriculumUnitSearchDependency, "bounded", 20)
	if err != nil || len(bounded) != 20 {
		t.Fatalf("bounded search count = %d err=%v", len(bounded), err)
	}
	empty, err := SearchCurriculumProposalUnits(context.Background(), database, draft, CurriculumUnitSearchDependency, "", 20)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty search = %#v err=%v", empty, err)
	}
}

func unitIDsContain(units []models.Unit, id int64) bool {
	for _, unit := range units {
		if unit.ID == id {
			return true
		}
	}
	return false
}

func TestSearchCurriculumProposalUnitsHonorsContextCancellation(t *testing.T) {
	fixtureDatabase := openPostgresIntegrationDatabase(t, "proposal_search_context")
	database := openConcurrentPostgresIntegrationDatabase(t, fixtureDatabase)
	var authorID int64
	if err := database.QueryRow(`INSERT INTO users (full_name) VALUES ('Cancelable Searcher') RETURNING id`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	proposal, err := CreateCurriculumProposal(database, authorID, "Cancelable search", "Stop abandoned picker searches.")
	if err != nil {
		t.Fatal(err)
	}
	held, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Rollback()
	if _, err := held.Exec(`LOCK TABLE units IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = SearchCurriculumProposalUnits(ctx, database, proposal, CurriculumUnitSearchDependency, "unit", 20)
	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("canceled proposal search error = %v context=%v", err, ctx.Err())
	}
}
