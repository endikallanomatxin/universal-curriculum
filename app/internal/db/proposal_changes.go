package db

import (
	"database/sql"
	"fmt"

	"universal-curriculum/internal/models"
)

func listCurriculumProposalChanges(q curriculumExecutor, proposalID int64) ([]models.CurriculumProposalChange, error) {
	rows, err := q.Query(`
		SELECT id, proposal_id, kind, unit_id,
		       COALESCE(unit_name, ''), COALESCE(unit_content, ''),
		       prerequisite_id
		FROM curriculum_proposal_change_details
		WHERE proposal_id = $1
		ORDER BY CASE kind
		             WHEN 'create_unit' THEN 1
		             WHEN 'rename_unit' THEN 2
		             WHEN 'update_content' THEN 2
		             WHEN 'remove_dependency' THEN 3
		             WHEN 'add_dependency' THEN 4
		             WHEN 'recognition' THEN 5
		             WHEN 'delete_unit' THEN 6
		         END,
		         unit_id NULLS LAST,
		         prerequisite_id NULLS LAST,
		         id
	`, proposalID)
	if err != nil {
		return nil, fmt.Errorf("list curriculum proposal changes: %w", err)
	}
	defer rows.Close()
	var changes []models.CurriculumProposalChange
	for rows.Next() {
		var change models.CurriculumProposalChange
		var unitID, prerequisite sql.NullInt64
		if err := rows.Scan(
			&change.ID, &change.ProposalID, &change.Kind,
			&unitID, &change.UnitName, &change.UnitContent, &prerequisite,
		); err != nil {
			return nil, fmt.Errorf("scan curriculum proposal change: %w", err)
		}
		if unitID.Valid {
			change.UnitID = unitID.Int64
		}
		if prerequisite.Valid {
			change.PrerequisiteID = &prerequisite.Int64
		}
		if change.Kind == "recognition" {
			change.Recognition = &models.Recognition{}
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close curriculum proposal changes: %w", err)
	}
	if err := loadCurriculumRecognitionMembers(q, proposalID, changes); err != nil {
		return nil, err
	}
	return changes, nil
}

func insertCurriculumProposalChangeDetail(q curriculumExecutor, change *models.CurriculumProposalChange) error {
	var err error
	switch change.Kind {
	case "create_unit":
		change.UnitID = change.ID
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_creations (change_id, name, content)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitName, change.UnitContent)
	case "rename_unit":
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_renames (change_id, unit_id, name)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitID, change.UnitName)
	case "update_content":
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_content_updates (change_id, unit_id, content)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitID, change.UnitContent)
	case "delete_unit":
		_, err = q.Exec(`
			INSERT INTO curriculum_unit_deletions (change_id, unit_id)
			VALUES ($1, $2)
		`, change.ID, change.UnitID)
	case "add_dependency":
		_, err = q.Exec(`
			INSERT INTO curriculum_dependency_additions (
				change_id, unit_id, prerequisite_id
			)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitID, change.PrerequisiteID)
	case "remove_dependency":
		_, err = q.Exec(`
			INSERT INTO curriculum_dependency_removals (
				change_id, unit_id, prerequisite_id
			)
			VALUES ($1, $2, $3)
		`, change.ID, change.UnitID, change.PrerequisiteID)
	case "recognition":
		if change.Recognition == nil {
			return fmt.Errorf("create curriculum recognition detail: missing recognition")
		}
		_, err = q.Exec(`
			INSERT INTO curriculum_recognitions (change_id)
			VALUES ($1)
		`, change.ID)
		if err == nil {
			for _, source := range change.Recognition.Sources {
				if _, err = q.Exec(`
					INSERT INTO curriculum_recognition_sources (recognition_change_id, unit_id)
					VALUES ($1, $2)
				`, change.ID, source.ID); err != nil {
					break
				}
			}
		}
		if err == nil {
			for _, target := range change.Recognition.Targets {
				if _, err = q.Exec(`
					INSERT INTO curriculum_recognition_targets (recognition_change_id, unit_id)
					VALUES ($1, $2)
				`, change.ID, target.ID); err != nil {
					break
				}
			}
		}
	default:
		return fmt.Errorf("create unsupported curriculum proposal change %q", change.Kind)
	}
	if err != nil {
		return fmt.Errorf("create curriculum proposal %s detail: %w", change.Kind, err)
	}
	return nil
}

func loadCurriculumRecognitionMembers(
	q curriculumExecutor,
	proposalID int64,
	changes []models.CurriculumProposalChange,
) error {
	changeIndexes := make(map[int64]int)
	for index := range changes {
		if changes[index].Recognition != nil {
			changeIndexes[changes[index].ID] = index
		}
	}
	if len(changeIndexes) == 0 {
		return nil
	}
	rows, err := q.Query(`
		SELECT member.recognition_change_id, member.role, member.unit_id,
		       COALESCE(unit.name, creation.name)
		FROM (
			SELECT source.recognition_change_id, 'source'::TEXT AS role, source.unit_id
			FROM curriculum_recognition_sources source
			UNION ALL
			SELECT target.recognition_change_id, 'target'::TEXT AS role, target.unit_id
			FROM curriculum_recognition_targets target
		) member
		JOIN curriculum_unit_creations creation ON creation.change_id = member.unit_id
		JOIN curriculum_recognitions recognition ON recognition.change_id = member.recognition_change_id
		JOIN curriculum_proposal_changes change ON change.id = recognition.change_id
		LEFT JOIN units unit ON unit.id = member.unit_id
		WHERE change.proposal_id = $1
		ORDER BY member.recognition_change_id, member.role, lower(COALESCE(unit.name, creation.name)), member.unit_id
	`, proposalID)
	if err != nil {
		return fmt.Errorf("list curriculum recognition members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var changeID, unitID int64
		var role, name string
		if err := rows.Scan(&changeID, &role, &unitID, &name); err != nil {
			return fmt.Errorf("scan curriculum recognition member: %w", err)
		}
		index, exists := changeIndexes[changeID]
		if !exists {
			return fmt.Errorf("recognition member references unknown proposal change %d", changeID)
		}
		unit := models.Unit{ID: unitID, Name: name}
		if role == "source" {
			changes[index].Recognition.Sources = append(
				changes[index].Recognition.Sources, unit,
			)
		} else {
			changes[index].Recognition.Targets = append(
				changes[index].Recognition.Targets, unit,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate curriculum recognition members: %w", err)
	}
	return nil
}
