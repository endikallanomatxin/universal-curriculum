package mcpadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	for _, guidance := range []string{"Search existing units", "get_recommendations", "recorded progress", "Never publish"} {
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
	if len(templates.ResourceTemplates) != 1 || templates.ResourceTemplates[0].URITemplate != "curriculum://units/{unit_id}" {
		t.Fatalf("resource templates = %#v", templates.ResourceTemplates)
	}
	about, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "curriculum://about"})
	if err != nil {
		t.Fatal(err)
	}
	if len(about.Contents) != 1 || !strings.Contains(about.Contents[0].Text, "dependency") || about.TTLMs == 0 || about.CacheScope != "public" {
		t.Fatalf("about resource = %#v", about)
	}

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 24 {
		t.Fatalf("tool count = %d, want 24", len(tools.Tools))
	}
	byName := make(map[string]*mcp.Tool, len(tools.Tools))
	for _, tool := range tools.Tools {
		byName[tool.Name] = tool
		if tool.OutputSchema == nil {
			t.Errorf("%s has no output schema", tool.Name)
		}
		if tool.Meta["securitySchemes"] == nil {
			t.Errorf("%s has no security metadata", tool.Name)
		}
	}
	if !byName["search_units"].Annotations.ReadOnlyHint || !byName["search_units"].Annotations.IdempotentHint {
		t.Fatalf("search annotations = %#v", byName["search_units"].Annotations)
	}
	if byName["publish_proposal"].Annotations.ReadOnlyHint ||
		byName["publish_proposal"].Annotations.DestructiveHint == nil ||
		!*byName["publish_proposal"].Annotations.DestructiveHint {
		t.Fatalf("publication annotations = %#v", byName["publish_proposal"].Annotations)
	}
	assertSchemaContains(t, byName["create_learning_path"].InputSchema,
		`"maxLength":200`, `"minItems":1`, `"uniqueItems":true`)
	assertSchemaContains(t, byName["create_proposal"].InputSchema,
		`"maxLength":200`, `"maxLength":1000`)
	assertSchemaContains(t, byName["add_proposal_recognition"].InputSchema,
		`"minItems":1`, `"uniqueItems":true`)
	assertSchemaContains(t, byName["publish_proposal"].InputSchema,
		`"confirmed"`, `"expected_title"`)
}

func TestNonAdministratorCannotUseProposalMutation(t *testing.T) {
	session, closeSession := connectTestMCP(t, &models.User{ID: 9})
	defer closeSession()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_proposal", Arguments: map[string]any{
			"title": "Not authorized", "rationale": "Permission is enforced before persistence.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v", result)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	if !strings.Contains(string(encoded), `"code":"permission_denied"`) {
		t.Fatalf("structured error = %s", encoded)
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
	server := newServer(&adapter{user: user, baseURL: "https://curriculum.example"})
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
