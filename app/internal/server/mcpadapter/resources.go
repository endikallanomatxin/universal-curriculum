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
	"universal-curriculum/internal/server/guidance"
)

func (application *adapter) addResources(server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		URI: "curriculum://about", Name: "universal_curriculum_about",
		Title: "About Universal Curriculum", MIMEType: "text/markdown",
		Description: "Concise domain concepts, workflows and safety constraints for an unfamiliar agent.",
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Cacheable: mcp.Cacheable{TTLMs: 3_600_000, CacheScope: "public"},
			Contents: []*mcp.ResourceContents{{
				URI: request.Params.URI, MIMEType: "text/markdown", Text: guidance.Index(),
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
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "curriculum://documentation/{slug}", Name: "documentation_page",
		Title: "Universal Curriculum documentation page", MIMEType: "text/markdown",
		Description: "Canonical guidance for humans and agents about one curriculum concept or workflow.",
	}, readDocumentationResource)
}

func readDocumentationResource(
	_ context.Context, request *mcp.ReadResourceRequest,
) (*mcp.ReadResourceResult, error) {
	parsed, err := url.Parse(request.Params.URI)
	if err != nil || parsed.Scheme != "curriculum" || parsed.Host != "documentation" {
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	page, ok := guidance.Find(strings.TrimPrefix(parsed.Path, "/"))
	if !ok {
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	return &mcp.ReadResourceResult{
		Cacheable: mcp.Cacheable{TTLMs: 3_600_000, CacheScope: "public"},
		Contents: []*mcp.ResourceContents{{
			URI: request.Params.URI, MIMEType: "text/markdown", Text: page.Content,
		}},
	}, nil
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
	return jsonResource(request.Params.URI, curriculumOverviewRepresentation(graph, proposalID))
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
	unit, err := db.GetUnit(application.database, unitID)
	if err != nil {
		return nil, fmt.Errorf("load published curriculum unit: %w", err)
	}
	if unit == nil {
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	dependencies, err := db.GetUnitDependencies(application.database, unitID)
	if err != nil {
		return nil, fmt.Errorf("load published curriculum unit relationships: %w", err)
	}
	return jsonResource(request.Params.URI, unitRepresentation(*unit, dependencies))
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
