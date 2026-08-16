package mcpadapter

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/server/guidance"
)

type emptyInput struct{}

type searchUnitsInput struct {
	Query  string `json:"query,omitempty" jsonschema:"Case-insensitive text to find in unit names or content; omit to browse."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum results from 1 to 100; defaults to 25."`
	Offset int    `json:"offset,omitempty" jsonschema:"Zero-based result offset."`
}

type searchUnitsOutput struct {
	Units   []unitSummary `json:"units"`
	Total   int           `json:"total"`
	Offset  int           `json:"offset"`
	HasMore bool          `json:"has_more"`
}

type getUnitInput struct {
	UnitID int64 `json:"unit_id" jsonschema:"Stable published curriculum unit ID."`
}

type authoringGuidanceDocument struct {
	URI     string `json:"uri"`
	Content string `json:"content"`
}

type authoringGuidance struct {
	Documents []authoringGuidanceDocument `json:"documents"`
}

func (application *adapter) addReadTools(server *mcp.Server) {
	addTool(server, "get_authoring_guidance", "Get curriculum authoring guidance",
		"Returns the canonical curriculum-unit, dependency, and writing guidance. Call before designing or modifying curriculum.",
		readOnly("Get curriculum authoring guidance"), application.getAuthoringGuidance)
	addTool(server, "get_curriculum", "Get published curriculum",
		"Returns compact unit summaries and the complete prerequisite graph. Use get_unit for content and prefer search_units when looking for a topic.",
		readOnly("Get published curriculum"), application.getCurriculum)
	addTool(server, "search_units", "Search curriculum units",
		"Searches published unit names and content. After loading get_authoring_guidance, use this to find existing and overlapping knowledge before proposing a unit.",
		readOnly("Search curriculum units"), application.searchUnits)
	addTool(server, "get_unit", "Get curriculum unit",
		"Returns one published unit with content, prerequisite IDs and dependent IDs. Inspect relevant search results to understand their scope and graph relationships.",
		readOnly("Get curriculum unit"), application.getUnit)
}

func (application *adapter) getAuthoringGuidance(
	_ context.Context, _ *mcp.CallToolRequest, _ emptyInput,
) (*mcp.CallToolResult, toolOutput[authoringGuidance], error) {
	pages := guidance.AuthoringPages()
	documents := make([]authoringGuidanceDocument, 0, len(pages))
	for _, page := range pages {
		documents = append(documents, authoringGuidanceDocument{
			URI: "curriculum://documentation/" + page.Slug, Content: page.Content,
		})
	}
	return ok(authoringGuidance{Documents: documents})
}

func (application *adapter) getCurriculum(
	_ context.Context, _ *mcp.CallToolRequest, _ emptyInput,
) (*mcp.CallToolResult, toolOutput[curriculumOverview], error) {
	graph, err := db.GetCurriculumGraph(application.database)
	if err != nil {
		return internalFailure[curriculumOverview]("get curriculum", err)
	}
	proposalID, err := db.GetCurrentCurriculumProposalID(application.database)
	if err != nil {
		return internalFailure[curriculumOverview]("get curriculum proposal", err)
	}
	return ok(newCurriculumOverview(curriculumRepresentation(graph, proposalID)))
}

func (application *adapter) searchUnits(
	_ context.Context, _ *mcp.CallToolRequest, input searchUnitsInput,
) (*mcp.CallToolResult, toolOutput[searchUnitsOutput], error) {
	if input.Limit == 0 {
		input.Limit = 25
	}
	graph, err := db.GetCurriculumGraph(application.database)
	if err != nil {
		return internalFailure[searchUnitsOutput]("search units", err)
	}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	items := make([]unitSummary, 0, len(graph.Units))
	for _, model := range graph.Units {
		if query != "" && !strings.Contains(strings.ToLower(model.Name), query) &&
			!strings.Contains(strings.ToLower(model.Content), query) {
			continue
		}
		items = append(items, newUnitSummary(model))
	}
	total := len(items)
	page := []unitSummary{}
	if input.Offset < total {
		end := min(input.Offset+input.Limit, total)
		page = items[input.Offset:end]
	}
	return ok(searchUnitsOutput{
		Units: page, Total: total, Offset: input.Offset,
		HasMore: input.Offset+len(page) < total,
	})
}

func (application *adapter) getUnit(
	_ context.Context, _ *mcp.CallToolRequest, input getUnitInput,
) (*mcp.CallToolResult, toolOutput[unit], error) {
	graph, err := db.GetCurriculumGraph(application.database)
	if err != nil {
		return internalFailure[unit]("get unit", err)
	}
	for _, item := range curriculumRepresentation(graph, nil).Units {
		if item.ID == input.UnitID {
			return ok(item)
		}
	}
	return failed[unit]("unit_not_found", "The published curriculum unit was not found.", nil)
}
