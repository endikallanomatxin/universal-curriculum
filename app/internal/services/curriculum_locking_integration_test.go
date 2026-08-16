package services

import (
	"errors"
	"testing"
	"time"

	"universal-curriculum/internal/db"
)

func TestCurriculumProposalLocksAreScopedByDraft(t *testing.T) {
	fixtureDatabase := openPostgresIntegrationDatabase(t, "proposal_locks")
	database := openConcurrentPostgresIntegrationDatabase(t, fixtureDatabase)
	var authorID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name, is_admin) VALUES ('Concurrent Editor', TRUE) RETURNING id
	`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	first, err := CreateCurriculumProposal(database, authorID, "First draft", "Exercise the first proposal lock.")
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateCurriculumProposal(database, authorID, "Second draft", "Exercise an independent proposal lock.")
	if err != nil {
		t.Fatal(err)
	}

	held, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Rollback()
	if _, err := db.LockCurrentCurriculumProposalShared(held); err != nil {
		t.Fatal(err)
	}
	if locked, err := db.LockCurriculumProposal(held, first.ID); err != nil || !locked {
		t.Fatalf("lock first proposal: locked=%v err=%v", locked, err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := CreateCurriculumUnit(database, authorID, second.ID, "Independent unit", "Teach independently.")
		result <- err
	}()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("mutate independently locked proposal: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mutation of a different proposal waited for the held draft lock")
	}
	if _, err := db.LockCurrentCurriculumProposal(held); err != nil {
		t.Fatal(err)
	}
	result = make(chan error, 1)
	go func() {
		plan, err := PlanCurriculumProposalRebase(database, second)
		if err == nil && (plan == nil || plan.Status != ProposalRebaseCurrent) {
			err = errors.New("unexpected current proposal rebase plan")
		}
		result <- err
	}()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("plan rebase while publication lock is held: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read-only rebase plan waited for the publication lock")
	}
}

func TestConcurrentDependencyChangesOnOneDraftAreSerialized(t *testing.T) {
	fixtureDatabase := openPostgresIntegrationDatabase(t, "proposal_dependency_locks")
	database := openConcurrentPostgresIntegrationDatabase(t, fixtureDatabase)
	var authorID int64
	if err := database.QueryRow(`INSERT INTO users (full_name) VALUES ('Concurrent Editor') RETURNING id`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	proposal, err := CreateCurriculumProposal(database, authorID, "Concurrent graph", "Keep dependency validation atomic.")
	if err != nil {
		t.Fatal(err)
	}
	first, err := CreateCurriculumUnit(database, authorID, proposal.ID, "First", "Teach the first concept.")
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Second", "Teach the second concept.")
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- AddUnitDependency(database, authorID, proposal.ID, first.ID, second.ID) }()
	go func() { <-start; results <- AddUnitDependency(database, authorID, proposal.ID, second.ID, first.ID) }()
	close(start)
	errorsSeen := []error{<-results, <-results}
	var successes, cycles int
	for _, err := range errorsSeen {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrDependencyCycle):
			cycles++
		default:
			t.Fatalf("concurrent dependency result: %v", err)
		}
	}
	if successes != 1 || cycles != 1 {
		t.Fatalf("concurrent dependency results: successes=%d cycles=%d", successes, cycles)
	}
}
