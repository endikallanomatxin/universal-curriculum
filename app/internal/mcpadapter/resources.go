package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/db"
)

const aboutResource = `# Universal Curriculum

Universal Curriculum is a shared, evolving map of learning units and prerequisite relationships. Units contain stable IDs, names and learning content. A dependency means the prerequisite should be completed before the dependent unit. Recognitions preserve learner progress when curriculum changes make earlier study equivalent to newer units.

Learners organize target units into private learning paths. Recorded progress is authoritative, and the platform's recommendation service determines what is currently available to study.

Administrators change the curriculum through draft proposals. A proposal may create, update or remove units and relationships. Drafts can become stale when another proposal is published: inspect and resolve rebase state before continuing. Publication changes the shared curriculum and must only happen after an explicit user request.`

func (application *adapter) addResources(server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		URI: "curriculum://about", Name: "universal_curriculum_about",
		Title: "About Universal Curriculum", MIMEType: "text/markdown",
		Description: "Concise domain concepts, workflows and safety constraints for an unfamiliar agent.",
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Cacheable: mcp.Cacheable{TTLMs: 3_600_000, CacheScope: "public"},
			Contents: []*mcp.ResourceContents{{
				URI: request.Params.URI, MIMEType: "text/markdown", Text: aboutResource,
			}},
		}, nil
	})
	server.AddResource(&mcp.Resource{
		URI: "curriculum://published", Name: "published_curriculum",
		Title: "Published curriculum", MIMEType: "application/json",
		Description: "The current published units and dependency graph with stable IDs.",
	}, application.readCurriculumResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "curriculum://units/{unit_id}", Name: "published_unit",
		Title: "Published curriculum unit", MIMEType: "application/json",
		Description: "One published unit with its content, prerequisites and dependents.",
	}, application.readUnitResource)
}

func (application *adapter) readCurriculumResource(
	_ context.Context, request *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	graph, err := db.GetCurriculumGraph(application.database)
	if err != nil {
		return nil, fmt.Errorf("load published curriculum: %w", err)
	}
	proposalID, err := db.GetCurrentCurriculumProposalID(application.database)
	if err != nil {
		return nil, fmt.Errorf("load curriculum publication: %w", err)
	}
	return jsonResource(request.Params.URI, curriculumRepresentation(graph, proposalID))
}

func (application *adapter) readUnitResource(
	_ context.Context, request *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	parsed, err := url.Parse(request.Params.URI)
	if err != nil || parsed.Scheme != "curriculum" || parsed.Host != "units" {
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	unitID, err := strconv.ParseInt(strings.TrimPrefix(parsed.Path, "/"), 10, 64)
	if err != nil || unitID <= 0 {
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	graph, err := db.GetCurriculumGraph(application.database)
	if err != nil {
		return nil, fmt.Errorf("load published curriculum unit: %w", err)
	}
	representation := curriculumRepresentation(graph, nil)
	for _, item := range representation.Units {
		if item.ID == unitID {
			return jsonResource(request.Params.URI, item)
		}
	}
	return nil, mcp.ResourceNotFoundError(request.Params.URI)
}

func jsonResource(uri string, value any) (*mcp.ReadResourceResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Cacheable: mcp.Cacheable{TTLMs: 30_000, CacheScope: "public"},
		Contents: []*mcp.ResourceContents{{
			URI: uri, MIMEType: "application/json", Text: string(encoded),
		}},
	}, nil
}
