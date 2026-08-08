package mcpadapter

import (
	"context"
	"errors"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/services"
)

type learningPathsOutput struct {
	LearningPaths []learningPath `json:"learning_paths"`
}

type createLearningPathInput struct {
	Name          string  `json:"name" jsonschema:"A concise learner-facing name, at most 200 characters."`
	TargetUnitIDs []int64 `json:"target_unit_ids" jsonschema:"One or more stable published unit IDs that are the goals of this path."`
}

type updateLearningPathInput struct {
	LearningPathID int64   `json:"learning_path_id" jsonschema:"ID of a learning path owned by the authenticated user."`
	Name           string  `json:"name" jsonschema:"The complete replacement name, at most 200 characters."`
	TargetUnitIDs  []int64 `json:"target_unit_ids" jsonschema:"The complete replacement set of target unit IDs."`
}

type deleteLearningPathInput struct {
	LearningPathID int64 `json:"learning_path_id" jsonschema:"ID of the learning path to delete."`
}

type deleteOutput struct {
	Deleted bool `json:"deleted"`
}

type progressOutput struct {
	Progress []progress `json:"progress"`
}

type setProgressInput struct {
	UnitID    int64 `json:"unit_id" jsonschema:"Stable published curriculum unit ID."`
	Completed bool  `json:"completed" jsonschema:"True records direct completion; false removes direct and derived completion for this unit."`
}

type recommendationsOutput struct {
	Recommendations []recommendation `json:"recommendations"`
}

func (application *adapter) addLearningTools(server *mcp.Server) {
	addTool(server, "get_learning_paths", "Get learning paths",
		"Returns only learning paths owned by the authenticated user.",
		readOnly("Get learning paths"), application.getLearningPaths)
	addTool(server, "create_learning_path", "Create learning path",
		"Creates a private path toward one or more published target units. This creation is not safe to retry after an ambiguous transport failure.",
		mutation("Create learning path", false, false), application.createLearningPath)
	addTool(server, "update_learning_path", "Update learning path",
		"Replaces the name and target units of a path owned by the authenticated user.",
		mutation("Update learning path", true, false), application.updateLearningPath)
	addTool(server, "delete_learning_path", "Delete learning path",
		"Deletes one private learning path. It does not erase recorded progress.",
		mutation("Delete learning path", true, true), application.deleteLearningPath)
	addTool(server, "get_progress", "Get recorded progress",
		"Returns authoritative direct, recognized and overall completion state for the authenticated user.",
		readOnly("Get recorded progress"), application.getProgress)
	addTool(server, "set_progress", "Set unit progress",
		"Idempotently records or removes completion for one published unit and returns the resulting authoritative status.",
		mutation("Set unit progress", true, true), application.setProgress)
	addTool(server, "get_recommendations", "Get study recommendations",
		"Uses Universal Curriculum's dependency and progress rules to find what the user can study next. Do not replace this with independent inference.",
		readOnly("Get study recommendations"), application.getRecommendations)
}

func (application *adapter) getLearningPaths(
	_ context.Context, request *mcp.CallToolRequest, _ emptyInput,
) (*mcp.CallToolResult, toolOutput[learningPathsOutput], error) {
	user := userFromRequest(request)
	models, err := db.ListLearningPaths(application.database, user.ID)
	if err != nil {
		return internalFailure[learningPathsOutput]("list learning paths", err)
	}
	result := learningPathsOutput{LearningPaths: make([]learningPath, 0, len(models))}
	for _, model := range models {
		result.LearningPaths = append(result.LearningPaths, newLearningPath(model))
	}
	return ok(result)
}

func (application *adapter) createLearningPath(
	_ context.Context, request *mcp.CallToolRequest, input createLearningPathInput,
) (*mcp.CallToolResult, toolOutput[learningPath], error) {
	user := userFromRequest(request)
	created, err := services.CreateLearningPath(
		application.database, user.ID, input.Name, input.TargetUnitIDs,
	)
	if err != nil {
		return learningPathFailure[learningPath]("create learning path", err)
	}
	created, err = db.GetLearningPath(application.database, user.ID, created.ID)
	if err != nil {
		return internalFailure[learningPath]("reload created learning path", err)
	}
	return ok(newLearningPath(*created))
}

func (application *adapter) updateLearningPath(
	_ context.Context, request *mcp.CallToolRequest, input updateLearningPathInput,
) (*mcp.CallToolResult, toolOutput[learningPath], error) {
	user := userFromRequest(request)
	err := services.UpdateLearningPath(
		application.database, user.ID, input.LearningPathID, input.Name, input.TargetUnitIDs,
	)
	if err != nil {
		return learningPathFailure[learningPath]("update learning path", err)
	}
	updated, err := db.GetLearningPath(application.database, user.ID, input.LearningPathID)
	if err != nil {
		return internalFailure[learningPath]("reload updated learning path", err)
	}
	return ok(newLearningPath(*updated))
}

func (application *adapter) deleteLearningPath(
	_ context.Context, request *mcp.CallToolRequest, input deleteLearningPathInput,
) (*mcp.CallToolResult, toolOutput[deleteOutput], error) {
	user := userFromRequest(request)
	deleted, err := db.DeleteLearningPath(application.database, user.ID, input.LearningPathID)
	if err != nil {
		return internalFailure[deleteOutput]("delete learning path", err)
	}
	if !deleted {
		return failed[deleteOutput]("learning_path_not_found", "The learning path was not found.", nil)
	}
	return ok(deleteOutput{Deleted: true})
}

func (application *adapter) getProgress(
	_ context.Context, request *mcp.CallToolRequest, _ emptyInput,
) (*mcp.CallToolResult, toolOutput[progressOutput], error) {
	user := userFromRequest(request)
	statuses, err := db.UnitCompletionStatuses(application.database, user.ID)
	if err != nil {
		return internalFailure[progressOutput]("get progress", err)
	}
	result := progressOutput{Progress: make([]progress, 0, len(statuses))}
	for unitID, status := range statuses {
		result.Progress = append(result.Progress, progress{
			UnitID: unitID, Direct: status.Direct, Recognized: status.Recognized,
			Completed: status.Completed(),
		})
	}
	sort.Slice(result.Progress, func(left, right int) bool {
		return result.Progress[left].UnitID < result.Progress[right].UnitID
	})
	return ok(result)
}

func (application *adapter) setProgress(
	_ context.Context, request *mcp.CallToolRequest, input setProgressInput,
) (*mcp.CallToolResult, toolOutput[progress], error) {
	user := userFromRequest(request)
	status, err := services.SetUnitProgress(
		application.database, user.ID, input.UnitID, input.Completed,
	)
	if errors.Is(err, services.ErrUnitNotFound) {
		return failed[progress]("unit_not_found", "The published curriculum unit was not found.", nil)
	}
	if err != nil {
		return internalFailure[progress]("set progress", err)
	}
	return ok(progress{
		UnitID: input.UnitID, Direct: status.Direct, Recognized: status.Recognized,
		Completed: status.Completed(),
	})
}

func (application *adapter) getRecommendations(
	_ context.Context, request *mcp.CallToolRequest, _ emptyInput,
) (*mcp.CallToolResult, toolOutput[recommendationsOutput], error) {
	user := userFromRequest(request)
	models, err := services.LearningRecommendations(application.database, user.ID)
	if err != nil {
		return internalFailure[recommendationsOutput]("get recommendations", err)
	}
	result := recommendationsOutput{Recommendations: make([]recommendation, 0, len(models))}
	for _, model := range models {
		item := recommendation{
			LearningPathID: model.LearningPathID, LearningPathName: model.LearningPathName,
			Units: make([]recommendedUnit, 0, len(model.Units)),
		}
		for _, recommended := range model.Units {
			item.Units = append(item.Units, recommendedUnit{
				unitSummary: newUnitSummary(recommended.Unit), Reason: recommended.Reason,
			})
		}
		result.Recommendations = append(result.Recommendations, item)
	}
	return ok(result)
}

func learningPathFailure[T any](
	operation string, err error,
) (*mcp.CallToolResult, toolOutput[T], error) {
	switch {
	case errors.Is(err, services.ErrLearningPathNotFound):
		return failed[T]("learning_path_not_found", "The learning path was not found.", nil)
	case errors.Is(err, services.ErrLearningPathNameRequired):
		return failed[T]("validation_failed", "The learning path name is required.", map[string]string{"name": "is required"})
	case errors.Is(err, services.ErrLearningPathNameTooLong):
		return failed[T]("validation_failed", "The learning path name is too long.", map[string]string{"name": "must not exceed 200 characters"})
	case errors.Is(err, services.ErrLearningPathUnitsRequired):
		return failed[T]("validation_failed", "At least one target unit is required.", map[string]string{"target_unit_ids": "must contain a current unit"})
	case errors.Is(err, services.ErrUnitNotFound):
		return failed[T]("validation_failed", "A target unit does not exist.", map[string]string{"target_unit_ids": "contains an unknown unit ID"})
	default:
		return internalFailure[T](operation, err)
	}
}
