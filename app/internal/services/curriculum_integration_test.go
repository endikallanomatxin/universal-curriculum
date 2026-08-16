package services

import (
	"errors"
	"testing"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

func TestCurrentProposalRebaseDoesNotLoadCurriculumGraph(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "current_proposal_rebase")
	var authorID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name, is_admin) VALUES ('Curriculum Editor', TRUE) RETURNING id
	`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}

	accepted, err := CreateCurriculumProposal(database, authorID, "Published foundations", "Establish the current curriculum version.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCurriculumUnit(database, authorID, accepted.ID, "Foundations", "Teach the foundations."); err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, accepted.ID); err != nil {
		t.Fatal(err)
	}

	current, err := CreateCurriculumProposal(database, authorID, "Current draft", "Remain based on the current curriculum version.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`ALTER TABLE units RENAME COLUMN content TO unavailable_content`); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanCurriculumProposalRebase(database, current)
	if err != nil || plan == nil || plan.Status != ProposalRebaseCurrent {
		t.Fatalf("plan current proposal rebase = %#v, err=%v", plan, err)
	}
	plan, err = TryAutoRebaseCurriculumProposal(database, current.ID)
	if err != nil || plan == nil || plan.Status != ProposalRebaseCurrent {
		t.Fatalf("auto-rebase current proposal = %#v, err=%v", plan, err)
	}
	if err := ResolveCurriculumProposalRebase(database, authorID, current.ID, nil); err != nil {
		t.Fatalf("resolve current proposal rebase: %v", err)
	}
}

func TestCurriculumProposalCollectsChangesAndPublishesAtomically(t *testing.T) {
	database := openPostgresIntegrationDatabase(t, "curriculum")
	var authorID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name, is_admin) VALUES ('Curriculum Editor', TRUE) RETURNING id
	`).Scan(&authorID); err != nil {
		t.Fatal(err)
	}

	proposal, err := CreateCurriculumProposal(database, authorID, "Mathematics foundations", "Introduce a coherent learning path.")
	if err != nil {
		t.Fatal(err)
	}
	var coauthorID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name) VALUES ('Curriculum Coauthor') RETURNING id
	`).Scan(&coauthorID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO curriculum_proposal_authors (proposal_id, user_id)
		VALUES ($1, $2)
	`, proposal.ID, coauthorID); err != nil {
		t.Fatal(err)
	}
	proposal, err = db.GetCurriculumProposal(database, proposal.ID)
	if err != nil || proposal == nil || !proposal.HasAuthor(authorID) || !proposal.HasAuthor(coauthorID) ||
		proposal.AuthorName != "Curriculum Coauthor, Curriculum Editor" {
		t.Fatalf("proposal authors = %#v err=%v", proposal, err)
	}
	foundations, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Foundations", "Learn the core foundations.")
	if err != nil {
		t.Fatal(err)
	}
	algebra, err := CreateCurriculumUnit(database, authorID, proposal.ID, "Algebra", "Learn variables and equations.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, foundations.ID, "Core foundations"); err != nil {
		t.Fatalf("rename proposed unit creation: %v", err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, foundations.ID, "Revised foundations."); err != nil {
		t.Fatalf("edit proposed unit creation content: %v", err)
	}
	for range 2 {
		if err := UpdateCurriculumUnitAndContent(
			database, authorID, proposal.ID, foundations.ID,
			"Final foundations", "Learn the final foundations.",
		); err != nil {
			t.Fatalf("replace proposed unit creation: %v", err)
		}
	}
	draftCreation, err := db.GetCurriculumProposal(database, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(draftCreation.Changes) != 2 ||
		draftCreation.Changes[0].Kind != "create_unit" ||
		draftCreation.Changes[0].ID != foundations.ID ||
		draftCreation.Changes[0].UnitID != foundations.ID ||
		draftCreation.Changes[0].UnitName != "Final foundations" ||
		draftCreation.Changes[0].UnitContent != "Learn the final foundations." {
		t.Fatalf("proposed unit creation did not converge to its final state: %#v", draftCreation.Changes)
	}
	if err := AddUnitDependency(database, authorID, proposal.ID, algebra.ID, foundations.ID); err != nil {
		t.Fatalf("connect units created by the same proposal: %v", err)
	}
	if err := AddUnitDependency(database, authorID, proposal.ID, foundations.ID, algebra.ID); err != ErrDependencyCycle {
		t.Fatalf("cycle between proposed units error = %v, want %v", err, ErrDependencyCycle)
	}
	// Draft changes do not mutate the published projection.
	graph, err := db.GetCurriculumGraphWithContent(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Units) != 0 {
		t.Fatalf("draft leaked into projection: %#v", graph)
	}

	// Publish the unit creations first: later proposals can refer to their stable IDs.
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	overview, err := db.GetCurriculumGraph(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Units) != 2 || overview.Units[0].Content != "" || overview.Units[1].Content != "" ||
		!overview.Units[0].CreatedAt.IsZero() || !overview.Units[1].UpdatedAt.IsZero() || len(overview.Dependencies) != 1 {
		t.Fatalf("lightweight curriculum graph = %#v", overview)
	}
	learningPath, err := CreateLearningPath(
		database, authorID, "Algebra goal", []int64{algebra.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetUnitCompleted(database, authorID, foundations.ID, true); err != nil {
		t.Fatal(err)
	}
	invalidProposal, err := CreateCurriculumProposal(
		database, authorID, "Cyclic foundations", "Exercise authoritative proposal validation.",
	)
	if err != nil {
		t.Fatal(err)
	}
	algebraID := algebra.ID
	invalidChangeTx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AddDraftCurriculumProposalChange(invalidChangeTx, invalidProposal.ID, authorID, &models.CurriculumProposalChange{
		Kind: "add_dependency", UnitID: foundations.ID, PrerequisiteID: &algebraID,
	}); err != nil {
		_ = invalidChangeTx.Rollback()
		t.Fatal(err)
	}
	if err := invalidChangeTx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, invalidProposal.ID); !errors.Is(err, ErrProposalInvalid) {
		t.Fatalf("publish invalid proposal error = %v, want %v", err, ErrProposalInvalid)
	}
	invalidProposal, err = db.GetCurriculumProposal(database, invalidProposal.ID)
	if err != nil || invalidProposal == nil || invalidProposal.Status != "draft" {
		t.Fatalf("invalid proposal was not preserved as a draft: proposal=%#v err=%v", invalidProposal, err)
	}
	staleProposal, err := CreateCurriculumProposal(database, authorID, "Alternative foundations", "Exercise conflict detection.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, staleProposal.ID, foundations.ID, "Alternative foundations"); err != nil {
		t.Fatal(err)
	}
	conflictingProposal, err := CreateCurriculumProposal(database, authorID, "Alternative algebra", "Exercise manual rebase review.")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, conflictingProposal.ID, algebra.ID, "Competing algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, conflictingProposal.ID, algebra.ID, "Competing algebra content."); err != nil {
		t.Fatal(err)
	}
	proposal, err = CreateCurriculumProposal(database, authorID, "Algebra path", "Connect and refine the new units.")
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveUnitDependency(database, authorID, proposal.ID, algebra.ID, foundations.ID); err != nil {
		t.Fatalf("remove published dependency: %v", err)
	}
	if err := AddUnitDependency(database, authorID, proposal.ID, algebra.ID, foundations.ID); err != nil {
		t.Fatalf("restore dependency in the same proposal: %v", err)
	}
	normalizedDependencies, err := db.GetCurriculumProposal(database, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range normalizedDependencies.Changes {
		if change.Kind == "add_dependency" || change.Kind == "remove_dependency" {
			t.Fatalf("dependency restored to its base state left changes behind: %#v", normalizedDependencies.Changes)
		}
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Work through variables, expressions, and equations."); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Learn variables and equations."); err != nil {
		t.Fatal(err)
	}
	draft, err := db.GetCurriculumProposal(database, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range draft.Changes {
		if change.Kind == "rename_unit" || change.Kind == "update_content" {
			t.Fatalf("unchanged unit value left a proposal change behind: %#v", change)
		}
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, proposal.ID, algebra.ID, "Introductory algebra"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Work through variables, expressions, and equations."); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, proposal.ID, algebra.ID, "Work through variables, expressions, and equations."); err != nil {
		t.Fatal(err)
	}
	draft, err = db.GetCurriculumProposal(database, proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	changeCounts := map[string]int{}
	for _, change := range draft.Changes {
		changeCounts[change.Kind]++
	}
	if changeCounts["rename_unit"] != 1 || changeCounts["update_content"] != 1 {
		t.Fatalf("unit edits accumulated duplicate proposal changes: %#v", draft.Changes)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, proposal.ID); err != nil {
		t.Fatal(err)
	}
	automaticallyRebased, err := db.GetCurriculumProposal(database, staleProposal.ID)
	if err != nil || automaticallyRebased == nil ||
		automaticallyRebased.BaseProposalID == nil || *automaticallyRebased.BaseProposalID != proposal.ID {
		t.Fatalf("disjoint proposal was not automatically rebased: proposal=%#v err=%v", automaticallyRebased, err)
	}
	rebasePlan, err := PlanCurriculumProposalRebase(database, conflictingProposal)
	if err != nil || !rebasePlan.NeedsReview() || len(rebasePlan.Conflicts) != 2 {
		t.Fatalf("overlapping proposal rebase plan = %#v, err=%v", rebasePlan, err)
	}
	resolutions := make(map[int64]CurriculumProposalRebaseResolution)
	var contentChangeID int64
	for _, conflict := range rebasePlan.Conflicts {
		switch conflict.Change.Kind {
		case "rename_unit":
			if conflict.AcceptedUnit == nil || conflict.AcceptedUnit.Name != "Introductory algebra" {
				t.Fatalf("rename conflict accepted version = %#v", conflict.AcceptedUnit)
			}
			resolutions[conflict.Change.ID] = CurriculumProposalRebaseResolution{Choice: "keep"}
		case "update_content":
			contentChangeID = conflict.Change.ID
			if conflict.AcceptedUnit == nil || conflict.AcceptedUnit.Content != "Work through variables, expressions, and equations." {
				t.Fatalf("content conflict accepted version = %#v", conflict.AcceptedUnit)
			}
			resolutions[conflict.Change.ID] = CurriculumProposalRebaseResolution{
				Choice: "edit", Content: "A reconciled algebra explanation.",
			}
		}
	}
	if err := ResolveCurriculumProposalRebase(
		database, authorID, conflictingProposal.ID, nil,
	); !errors.Is(err, ErrRebaseResolutionRequired) {
		t.Fatalf("incomplete rebase resolution error = %v, want %v", err, ErrRebaseResolutionRequired)
	}
	if err := ResolveCurriculumProposalRebase(
		database, authorID, conflictingProposal.ID, resolutions,
	); err != nil {
		t.Fatalf("resolve curriculum proposal rebase: %v", err)
	}
	conflictingProposal, err = db.GetCurriculumProposal(database, conflictingProposal.ID)
	if err != nil || conflictingProposal == nil || conflictingProposal.BaseProposalID == nil ||
		*conflictingProposal.BaseProposalID != proposal.ID ||
		len(conflictingProposal.Changes) != 2 {
		t.Fatalf("manual rebase did not normalize the retained change: proposal=%#v err=%v", conflictingProposal, err)
	}
	var resolvedContent *models.CurriculumProposalChange
	for index := range conflictingProposal.Changes {
		if conflictingProposal.Changes[index].ID == contentChangeID {
			resolvedContent = &conflictingProposal.Changes[index]
		}
	}
	if resolvedContent == nil || resolvedContent.UnitContent != "A reconciled algebra explanation." {
		t.Fatalf("custom content rebase resolution = %#v", resolvedContent)
	}
	if err := DeleteCurriculumProposal(database, authorID, conflictingProposal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, staleProposal.ID); err != nil {
		t.Fatalf("publish automatically rebased proposal: %v", err)
	}
	graph, err = db.GetCurriculumGraphWithContent(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Units) != 2 || len(graph.Dependencies) != 1 {
		t.Fatalf("unexpected published graph: %#v", graph)
	}
	completedUnitIDs, err := db.CompletedUnitIDs(database, authorID)
	if err != nil || !completedUnitIDs[foundations.ID] {
		t.Fatalf("curriculum publication did not preserve completion: ids=%v err=%v", completedUnitIDs, err)
	}
	persistedPath, err := db.GetLearningPath(database, authorID, learningPath.ID)
	if err != nil || persistedPath == nil || len(persistedPath.Units) != 1 || persistedPath.Units[0].ID != algebra.ID {
		t.Fatalf("curriculum rebuild did not preserve the learning path: path=%#v err=%v", persistedPath, err)
	}
	if graph.Units[0].ID == algebra.ID && graph.Units[0].Content != "Work through variables, expressions, and equations." ||
		graph.Units[1].ID == algebra.ID && graph.Units[1].Content != "Work through variables, expressions, and equations." {
		t.Fatalf("content change was not published: %#v", graph.Units)
	}
	if _, err := database.Exec(`DELETE FROM curriculum_proposals WHERE id = $1`, proposal.ID); err == nil {
		t.Fatal("accepted proposal was deleted")
	}
	if err := db.SetUnitCompleted(database, authorID, foundations.ID, false); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || completedUnitIDs[foundations.ID] {
		t.Fatalf("unit remained completed after returning it to pending: ids=%v err=%v", completedUnitIDs, err)
	}
	var completionCount int
	if err := database.QueryRow(`
		SELECT count(*) FROM unit_completions
		WHERE user_id = $1 AND unit_id = $2
	`, authorID, foundations.ID).Scan(&completionCount); err != nil || completionCount != 0 {
		t.Fatalf("direct completion count = %d err=%v, want no row after returning to pending", completionCount, err)
	}

	if err := db.SetUnitCompleted(database, authorID, algebra.ID, true); err != nil {
		t.Fatal(err)
	}
	var creationChangeID, completionProposalID int64
	if err := database.QueryRow(`
		SELECT creation.change_id, completion.curriculum_proposal_id
		FROM curriculum_unit_creations creation
		JOIN unit_completions completion ON completion.unit_id = creation.change_id
		WHERE creation.change_id = $1 AND completion.user_id = $2
	`, algebra.ID, authorID).Scan(&creationChangeID, &completionProposalID); err != nil {
		t.Fatal(err)
	}
	if creationChangeID != algebra.ID || completionProposalID == 0 {
		t.Fatalf("durable unit identity or completion state missing: creation=%d completion proposal=%d", creationChangeID, completionProposalID)
	}
	retirement, err := CreateCurriculumProposal(database, authorID, "Retire algebra", "Exercise durable unit references.")
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := CreateCurriculumUnit(
		database, authorID, retirement.ID, "Applied algebra", "Apply algebra to practical problems.",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnit(database, authorID, retirement.ID, algebra.ID, "Temporary algebra name"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(database, authorID, retirement.ID, algebra.ID, "Temporary algebra content."); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCurriculumUnit(database, authorID, retirement.ID, algebra.ID); err != nil {
		t.Fatal(err)
	}
	if err := AddCurriculumRecognition(
		database,
		authorID,
		retirement.ID,
		[]int64{algebra.ID},
		[]int64{replacement.ID},
	); err != nil {
		t.Fatal(err)
	}
	normalizedRetirement, err := db.GetCurriculumProposal(database, retirement.ID)
	if err != nil || normalizedRetirement == nil ||
		len(normalizedRetirement.Changes) != 3 ||
		normalizedRetirement.Changes[1].Kind != "recognition" ||
		normalizedRetirement.Changes[2].Kind != "delete_unit" ||
		len(normalizedRetirement.Changes[1].Recognition.Sources) != 1 ||
		normalizedRetirement.Changes[1].Recognition.Sources[0].ID != algebra.ID ||
		len(normalizedRetirement.Changes[1].Recognition.Targets) != 1 ||
		normalizedRetirement.Changes[1].Recognition.Targets[0].ID != replacement.ID {
		t.Fatalf("unit deletion did not remove superseded changes: proposal=%#v err=%v", normalizedRetirement, err)
	}
	if err := UpdateCurriculumUnit(database, authorID, retirement.ID, algebra.ID, "Renamed after deletion"); err != ErrUnitNotFound {
		t.Fatalf("rename deleted unit error = %v, want %v", err, ErrUnitNotFound)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, retirement.ID); err != nil {
		t.Fatal(err)
	}
	recognitionChangeID := normalizedRetirement.Changes[1].ID
	if _, err := database.Exec(`
		DELETE FROM curriculum_recognition_targets WHERE recognition_change_id = $1
	`, recognitionChangeID); err == nil {
		t.Fatal("accepted recognition target was deleted")
	}
	persistedPath, err = db.GetLearningPath(database, authorID, learningPath.ID)
	if err != nil || persistedPath == nil || len(persistedPath.Units) != 1 ||
		persistedPath.Units[0].ID != replacement.ID || persistedPath.Units[0].Retired {
		t.Fatalf("recognition did not migrate the learning path target: path=%#v err=%v", persistedPath, err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || !completedUnitIDs[algebra.ID] || !completedUnitIDs[replacement.ID] {
		t.Fatalf("retiring a unit did not preserve and recognition progress: ids=%v err=%v", completedUnitIDs, err)
	}
	completionStatuses, err := db.UnitCompletionStatuses(database, authorID)
	if err != nil ||
		!completionStatuses[algebra.ID].Direct ||
		completionStatuses[replacement.ID].Direct ||
		!completionStatuses[replacement.ID].Recognized {
		t.Fatalf("direct and recognized progress were conflated: statuses=%v err=%v", completionStatuses, err)
	}
	if err := db.SetUnitCompleted(database, authorID, foundations.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetUnitCompleted(database, coauthorID, replacement.ID, true); err != nil {
		t.Fatal(err)
	}
	var sameProposalUserID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name) VALUES ('Same proposal learner') RETURNING id
	`).Scan(&sameProposalUserID); err != nil {
		t.Fatal(err)
	}
	if err := db.SetUnitCompleted(database, sameProposalUserID, foundations.ID, true); err != nil {
		t.Fatal(err)
	}
	advancedProposal, err := CreateCurriculumProposal(
		database, authorID, "Extend applied algebra", "Exercise forward-only recognition.",
	)
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := CreateCurriculumUnit(
		database, authorID, advancedProposal.ID, "Advanced applied algebra", "Solve advanced applications.",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddCurriculumRecognition(
		database, authorID, advancedProposal.ID,
		[]int64{foundations.ID}, []int64{replacement.ID},
	); err != nil {
		t.Fatal(err)
	}
	if err := AddCurriculumRecognition(
		database, authorID, advancedProposal.ID,
		[]int64{replacement.ID, foundations.ID}, []int64{advanced.ID},
	); err != nil {
		t.Fatal(err)
	}
	activeRecognitionPath, err := CreateLearningPath(
		database, sameProposalUserID, "Foundations goal", []int64{foundations.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, advancedProposal.ID); err != nil {
		t.Fatal(err)
	}
	persistedPath, err = db.GetLearningPath(database, sameProposalUserID, activeRecognitionPath.ID)
	if err != nil || persistedPath == nil || len(persistedPath.Units) != 3 {
		t.Fatalf("active recognition sources and targets were not preserved in the learning path: path=%#v err=%v", persistedPath, err)
	}
	pathTargetIDs := make(map[int64]bool, len(persistedPath.Units))
	for _, unit := range persistedPath.Units {
		pathTargetIDs[unit.ID] = true
	}
	if !pathTargetIDs[foundations.ID] || !pathTargetIDs[replacement.ID] || !pathTargetIDs[advanced.ID] {
		t.Fatalf("recognition did not add every target for a path containing one merge source: ids=%v", pathTargetIDs)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || !completedUnitIDs[advanced.ID] {
		t.Fatalf("recognition did not flow through an earlier proposal: ids=%v err=%v", completedUnitIDs, err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, sameProposalUserID)
	if err != nil || !completedUnitIDs[replacement.ID] || completedUnitIDs[advanced.ID] {
		t.Fatalf("recognitions in one proposal fed one another: ids=%v err=%v", completedUnitIDs, err)
	}
	var lateEvidenceUserID int64
	if err := database.QueryRow(`
		INSERT INTO users (full_name) VALUES ('Late evidence learner') RETURNING id
	`).Scan(&lateEvidenceUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO unit_completions (user_id, unit_id, curriculum_proposal_id)
		VALUES ($1, $2, $4), ($1, $3, $4)
	`, lateEvidenceUserID, foundations.ID, replacement.ID, retirement.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.MaterializeCurriculumRecognitions(database, advancedProposal.ID); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, lateEvidenceUserID)
	if err != nil || !completedUnitIDs[advanced.ID] {
		t.Fatalf("late historical evidence did not catch up through recognition: ids=%v err=%v", completedUnitIDs, err)
	}
	if err := db.SetUnitCompleted(database, coauthorID, replacement.ID, true); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, coauthorID)
	if err != nil || !completedUnitIDs[replacement.ID] || completedUnitIDs[advanced.ID] {
		t.Fatalf("recognition was applied without every source: ids=%v err=%v", completedUnitIDs, err)
	}
	if err := db.SetUnitCompleted(database, coauthorID, foundations.ID, true); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, coauthorID)
	if err != nil || completedUnitIDs[advanced.ID] {
		t.Fatalf("later source completion activated an earlier recognition: ids=%v err=%v", completedUnitIDs, err)
	}
	if err := db.SetUnitCompleted(database, authorID, algebra.ID, false); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || completedUnitIDs[algebra.ID] || !completedUnitIDs[replacement.ID] || completedUnitIDs[advanced.ID] {
		t.Fatalf("unsupported recognized progress survived returning its source to pending: ids=%v err=%v", completedUnitIDs, err)
	}
	if err := db.SetUnitCompleted(database, authorID, replacement.ID, false); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || completedUnitIDs[replacement.ID] || completedUnitIDs[advanced.ID] {
		t.Fatalf("recognized completion did not softly return to pending: ids=%v err=%v", completedUnitIDs, err)
	}
	if err := db.MaterializeCurriculumRecognitions(database, advancedProposal.ID); err != nil {
		t.Fatal(err)
	}
	completedUnitIDs, err = db.CompletedUnitIDs(database, authorID)
	if err != nil || !completedUnitIDs[replacement.ID] || completedUnitIDs[advanced.ID] {
		t.Fatalf("explicit replay did not regenerate soft recognition correctly: ids=%v err=%v", completedUnitIDs, err)
	}
	if err := db.SetUnitCompleted(database, authorID, replacement.ID, true); err != nil {
		t.Fatal(err)
	}
	contentRevision, err := CreateCurriculumProposal(
		database, authorID, "Revise applied algebra", "Exercise version-aware completion evidence.",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateCurriculumUnitContent(
		database, authorID, contentRevision.ID, replacement.ID,
		"Apply algebra to practical problems with revised examples.",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := submitAndAcceptCurriculumProposal(database, authorID, contentRevision.ID); err != nil {
		t.Fatal(err)
	}
	completionStatuses, err = db.UnitCompletionStatuses(database, authorID)
	if err != nil || completionStatuses[replacement.ID].Direct || !completionStatuses[replacement.ID].Recognized {
		t.Fatalf("modified completion was not convalidated: statuses=%v err=%v", completionStatuses, err)
	}
	if err := db.SetUnitCompleted(database, authorID, replacement.ID, true); err != nil {
		t.Fatal(err)
	}
	completionStatuses, err = db.UnitCompletionStatuses(database, authorID)
	if err != nil || !completionStatuses[replacement.ID].Direct || completionStatuses[replacement.ID].Recognized {
		t.Fatalf("current-version completion was not refreshed: statuses=%v err=%v", completionStatuses, err)
	}

}
