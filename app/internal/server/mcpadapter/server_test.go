package mcpadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/models"
)

func TestDiscoveryAdvertisesAgentGuidanceResourcesAndTools(t *testing.T) {
	session, closeSession := connectTestMCP(t, &models.User{ID: 7, IsAdmin: true})
	defer closeSession()

	discovery := session.InitializeResult()
	if discovery == nil || discovery.ServerInfo == nil {
		t.Fatal("MCP discovery result is missing")
	}
	if discovery.ServerInfo.Name != "universal-curriculum" {
		t.Fatalf("server name = %q", discovery.ServerInfo.Name)
	}
	for _, guidance := range []string{
		"curriculum://documentation/curriculum-units",
		"curriculum://documentation/dependencies",
		"curriculum://documentation/writing-content",
		"Search the published curriculum", "finished microlesson",
		"Review every changed unit", "get_recommendations", "recorded progress",
		"Never submit", "explicit request and confirmation",
	} {
		if !strings.Contains(discovery.Instructions, guidance) {
			t.Errorf("instructions do not contain %q: %s", guidance, discovery.Instructions)
		}
	}
	if discovery.Capabilities == nil || discovery.Capabilities.Tools == nil || discovery.Capabilities.Resources == nil {
		t.Fatalf("capabilities = %#v", discovery.Capabilities)
	}
	if discovery.Capabilities.Prompts != nil || discovery.Capabilities.Logging != nil {
		t.Fatalf("unexpected agent-framework capabilities = %#v", discovery.Capabilities)
	}

	resources, err := session.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Resources) != 2 || resources.Resources[0].URI != "curriculum://about" || resources.Resources[1].URI != "curriculum://published" {
		t.Fatalf("resources = %#v", resources.Resources)
	}
	templates, err := session.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	templateURIs := make(map[string]bool, len(templates.ResourceTemplates))
	for _, resourceTemplate := range templates.ResourceTemplates {
		templateURIs[resourceTemplate.URITemplate] = true
	}
	if len(templates.ResourceTemplates) != 2 || !templateURIs["curriculum://units/{unit_id}"] || !templateURIs["curriculum://documentation/{slug}"] {
		t.Fatalf("resource templates = %#v", templates.ResourceTemplates)
	}
	about, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "curriculum://about"})
	if err != nil {
		t.Fatal(err)
	}
	if len(about.Contents) != 1 || !strings.Contains(about.Contents[0].Text, "curriculum://documentation/dependencies") || about.TTLMs == 0 || about.CacheScope != "public" {
		t.Fatalf("about resource = %#v", about)
	}
	writing, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "curriculum://documentation/writing-content"})
	if err != nil {
		t.Fatal(err)
	}
	if len(writing.Contents) != 1 ||
		!strings.Contains(writing.Contents[0].Text, "worked example") ||
		!strings.Contains(writing.Contents[0].Text, "overlapping") ||
		!strings.Contains(writing.Contents[0].Text, "Content supports Markdown") ||
		!strings.Contains(writing.Contents[0].Text, "$$...$$") {
		t.Fatalf("writing documentation resource = %#v", writing)
	}

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 26 {
		t.Fatalf("tool count = %d, want 26", len(tools.Tools))
	}
	byName := make(map[string]*mcp.Tool, len(tools.Tools))
	for _, tool := range tools.Tools {
		byName[tool.Name] = tool
		if tool.OutputSchema == nil {
			t.Errorf("%s has no output schema", tool.Name)
		}
		if _, ok := tool.Meta["securitySchemes"]; ok {
			t.Errorf("%s has legacy security metadata", tool.Name)
		}
	}
	if !byName["search_units"].Annotations.ReadOnlyHint || !byName["search_units"].Annotations.IdempotentHint {
		t.Fatalf("search annotations = %#v", byName["search_units"].Annotations)
	}
	if byName["submit_proposal"].Annotations.ReadOnlyHint ||
		byName["submit_proposal"].Annotations.DestructiveHint == nil ||
		!*byName["submit_proposal"].Annotations.DestructiveHint {
		t.Fatalf("submission annotations = %#v", byName["submit_proposal"].Annotations)
	}
	assertSchemaContains(t, byName["create_learning_path"].InputSchema,
		`"maxLength":200`, `"minItems":1`, `"uniqueItems":true`)
	assertSchemaContains(t, byName["create_proposal"].InputSchema,
		`"maxLength":200`, `"maxLength":1000`)
	assertSchemaContains(t, byName["create_proposal_unit"].InputSchema,
		`Final learner-facing microlesson`, `rather than an outline`,
		`Supports Markdown and LaTeX`, `$...$`, `$$...$$`)
	assertSchemaContains(t, byName["update_proposal_unit"].InputSchema,
		`final learner-facing content`, `not an outline`,
		`Supports Markdown and LaTeX`, `$...$`, `$$...$$`)
	if !strings.Contains(byName["create_proposal_unit"].Description, "curriculum-units, dependencies, and writing-content") {
		t.Fatalf("create_proposal_unit description = %q", byName["create_proposal_unit"].Description)
	}
	if !strings.Contains(byName["update_proposal_unit"].Description, "rather than adding edit history") {
		t.Fatalf("update_proposal_unit description = %q", byName["update_proposal_unit"].Description)
	}
	assertSchemaContains(t, byName["add_proposal_recognition"].InputSchema,
		`"minItems":1`, `"uniqueItems":true`)
	assertSchemaContains(t, byName["submit_proposal"].InputSchema,
		`"confirmed"`, `"expected_title"`)
}

func TestNonAdministratorDiscoversOnlyAvailableTools(t *testing.T) {
	session, closeSession := connectTestMCP(t, &models.User{ID: 9})
	defer closeSession()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 10 {
		t.Fatalf("tool count = %d, want 10", len(result.Tools))
	}
	for _, tool := range result.Tools {
		if strings.Contains(tool.Name, "proposal") {
			t.Errorf("non-administrator discovered %q", tool.Name)
		}
	}
}

func TestProposalHandlersStillEnforceAdministratorPermission(t *testing.T) {
	application := &adapter{}
	request := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: tokenInfoForUser(&models.User{ID: 9})}}
	result, output, err := application.createProposal(context.Background(), request, createProposalInput{
		Title: "Not authorized", Rationale: "Permission remains enforced below discovery.",
	})
	if err != nil || result == nil || !result.IsError || output.Error == nil || output.Error.Code != "permission_denied" {
		t.Fatalf("createProposal() = %#v, %#v, %v", result, output, err)
	}
}

func TestServersAreReusedByPermissionLevel(t *testing.T) {
	shared := newServers(&adapter{baseURL: "https://curriculum.example"})
	learnerOne := tokenInfoForUser(&models.User{ID: 1})
	learnerTwo := tokenInfoForUser(&models.User{ID: 2})
	adminOne := tokenInfoForUser(&models.User{ID: 3, IsAdmin: true})
	adminTwo := tokenInfoForUser(&models.User{ID: 4, IsAdmin: true})

	if shared.forToken(learnerOne) != shared.forToken(learnerTwo) {
		t.Fatal("learner requests did not reuse the standard MCP server")
	}
	if shared.forToken(adminOne) != shared.forToken(adminTwo) {
		t.Fatal("administrator requests did not reuse the admin MCP server")
	}
	if shared.forToken(learnerOne) == shared.forToken(adminOne) {
		t.Fatal("standard and administrator requests selected the same tool catalog")
	}
}

func TestHTTPTransportRequiresBearerAuthentication(t *testing.T) {
	handler := NewHandler(nil, "https://curriculum.example")
	request := httptest.NewRequest(http.MethodPost, "https://curriculum.example/mcp", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	challenge := response.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, "oauth-protected-resource/mcp") || !strings.Contains(challenge, `scope="mcp"`) {
		t.Fatalf("WWW-Authenticate = %q", challenge)
	}
}

func connectTestMCP(t *testing.T, user *models.User) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()
	server := newServer(&adapter{baseURL: "https://curriculum.example"}, user.IsAdmin)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	return clientSession, func() {
		_ = clientSession.Close()
		_ = serverSession.Wait()
	}
}

func tokenInfoForUser(user *models.User) *auth.TokenInfo {
	return &auth.TokenInfo{Extra: map[string]any{"user": user}}
}

func assertSchemaContains(t *testing.T, schema any, values ...string) {
	t.Helper()
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if !strings.Contains(string(encoded), value) {
			t.Errorf("schema does not contain %s: %s", value, encoded)
		}
	}
}
