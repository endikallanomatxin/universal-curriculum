package mcpadapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

const instructions = "Universal Curriculum is a shared, dependency-aware curriculum. Search existing units before proposing a new one. Curriculum changes happen only through proposals. Use get_recommendations instead of inferring what a learner should study next; recorded progress is authoritative. Inspect rebase state before changing a stale proposal. Never publish merely because a user asked to edit or prepare a proposal: publish only after an explicit request and confirmation. Server-side permissions always apply."

var (
	falseHint   = false
	trueHint    = true
	schemaCache = mcp.NewSchemaCache()
)

type adapter struct {
	database *sql.DB
	user     *models.User
	baseURL  string
}

func NewHandler(database *sql.DB, baseURL string) http.Handler {
	resourceURL := strings.TrimRight(baseURL, "/") + "/mcp"
	stream := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		info := auth.TokenInfoFromContext(request.Context())
		var user *models.User
		if info != nil {
			user, _ = info.Extra["user"].(*models.User)
		}
		return newServer(&adapter{database: database, user: user, baseURL: strings.TrimRight(baseURL, "/")})
	}, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 1 << 20,
		PropagateRequestCancellation: true,
	})
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		user, err := db.AuthenticateAPIToken(database, token)
		if err == nil && user == nil {
			user, err = db.AuthenticateOAuthAccessToken(database, token, resourceURL)
		}
		if err != nil {
			log.Printf("authenticate MCP token: %v", err)
			return nil, errors.New("unable to validate bearer token")
		}
		if user == nil {
			return nil, fmt.Errorf("%w: bearer token is invalid or expired", auth.ErrInvalidToken)
		}
		return &auth.TokenInfo{
			Scopes: []string{"mcp"}, UserID: strconv.FormatInt(user.ID, 10),
			Extra: map[string]any{"user": user},
		}, nil
	}
	protected := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: strings.TrimRight(baseURL, "/") + "/.well-known/oauth-protected-resource/mcp",
		Scopes:              []string{"mcp"}, AllowMissingExpiration: true,
	})(stream)
	originProtection := http.NewCrossOriginProtection()
	return originProtection.Handler(protected)
}

func newServer(application *adapter) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name: "universal-curriculum", Title: "Universal Curriculum", Version: "0.2.0",
		Description: "Agent-oriented access to Universal Curriculum.", WebsiteURL: application.baseURL,
	}, &mcp.ServerOptions{
		Instructions: instructions,
		Capabilities: &mcp.ServerCapabilities{
			Resources: &mcp.ResourceCapabilities{}, Tools: &mcp.ToolCapabilities{},
		},
		SchemaCache: schemaCache,
	})
	application.addResources(server)
	application.addReadTools(server)
	application.addLearningTools(server)
	application.addProposalTools(server)
	return server
}

func addTool[In, Out any](
	server *mcp.Server, name, title, description string, annotations *mcp.ToolAnnotations,
	handler mcp.ToolHandlerFor[In, toolOutput[Out]],
) {
	inputSchema, err := jsonschema.For[In](nil)
	if err != nil {
		panic(err)
	}
	constrainSchema(inputSchema)
	outputSchema, err := jsonschema.For[toolOutput[Out]](nil)
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: name, Title: title, Description: description, Annotations: annotations,
		InputSchema: inputSchema, OutputSchema: outputSchema,
		Meta: mcp.Meta{"securitySchemes": []map[string]any{{"type": "oauth2", "scopes": []string{"mcp"}}}},
	}, handler)
}

func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title: title, ReadOnlyHint: true, IdempotentHint: true,
		DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
}

func mutation(title string, idempotent, destructive bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title: title, ReadOnlyHint: false, IdempotentHint: idempotent,
		DestructiveHint: boolPointer(destructive), OpenWorldHint: &falseHint,
	}
}

func boolPointer(value bool) *bool {
	if value {
		return &trueHint
	}
	return &falseHint
}

func ok[T any](data T) (*mcp.CallToolResult, toolOutput[T], error) {
	return nil, toolOutput[T]{OK: true, Data: &data}, nil
}

func failed[T any](code, message string, fields map[string]string) (*mcp.CallToolResult, toolOutput[T], error) {
	return &mcp.CallToolResult{IsError: true}, toolOutput[T]{
		OK: false, Error: &toolError{Code: code, Message: message, Fields: fields, Retryable: false},
	}, nil
}

func internalFailure[T any](operation string, err error) (*mcp.CallToolResult, toolOutput[T], error) {
	log.Printf("MCP %s: %v", operation, err)
	return failed[T]("internal_error", "The operation could not be completed.", nil)
}

func requireAdmin[T any](application *adapter) (*mcp.CallToolResult, toolOutput[T], error, bool) {
	if application.user != nil && application.user.IsAdmin {
		return nil, toolOutput[T]{}, nil, true
	}
	result, output, err := failed[T]("permission_denied", "Administrator permission is required.", nil)
	return result, output, err, false
}

func constrainSchema(schema *jsonschema.Schema) {
	for _, definition := range schema.Defs {
		constrainSchema(definition)
	}
	if schema.Items != nil {
		constrainSchema(schema.Items)
	}
	for name, property := range schema.Properties {
		constrainSchema(property)
		switch name {
		case "query":
			property.MaxLength = jsonschema.Ptr(200)
		case "name":
			property.MinLength, property.MaxLength = jsonschema.Ptr(1), jsonschema.Ptr(200)
		case "title":
			property.MinLength, property.MaxLength = jsonschema.Ptr(1), jsonschema.Ptr(200)
		case "rationale":
			property.MinLength, property.MaxLength = jsonschema.Ptr(1), jsonschema.Ptr(1000)
		case "content":
			property.MinLength = jsonschema.Ptr(1)
		case "limit":
			property.Minimum, property.Maximum = jsonschema.Ptr(1.0), jsonschema.Ptr(100.0)
		case "offset":
			property.Minimum = jsonschema.Ptr(0.0)
		case "proposal_id", "unit_id", "learning_path_id", "change_id", "prerequisite_id":
			property.Minimum = jsonschema.Ptr(1.0)
		case "target_unit_ids", "source_unit_ids":
			property.MinItems, property.UniqueItems = jsonschema.Ptr(1), true
			if property.Items != nil {
				property.Items.Minimum = jsonschema.Ptr(1.0)
			}
		case "status":
			property.Enum = []any{"draft", "accepted", "rejected"}
		case "choice":
			property.Enum = []any{"keep", "drop", "edit"}
		}
	}
}
