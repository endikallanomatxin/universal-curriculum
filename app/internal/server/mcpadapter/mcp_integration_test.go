package mcpadapter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/db/migrations"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

func TestMCPAgentWorkflowWithPostgreSQL(t *testing.T) {
	database := openMCPIntegrationDatabase(t)
	admin := registerMCPIntegrationUser(t, database, "MCP Administrator", "mcp-admin@example.com", true)
	learner := registerMCPIntegrationUser(t, database, "MCP Learner", "mcp-learner@example.com", false)
	otherLearner := registerMCPIntegrationUser(t, database, "Other Learner", "other-mcp-learner@example.com", false)

	baseProposal, err := services.CreateCurriculumProposal(database, admin.ID, "Initial curriculum", "Create an agent workflow fixture.")
	if err != nil {
		t.Fatal(err)
	}
	foundation, err := services.CreateCurriculumUnit(database, admin.ID, baseProposal.ID, "Foundations", "Learn the foundations.")
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := services.CreateCurriculumUnit(database, admin.ID, baseProposal.ID, "Advanced topic", "Build on the foundations.")
	if err != nil {
		t.Fatal(err)
	}
	if err := services.AddUnitDependency(database, admin.ID, baseProposal.ID, advanced.ID, foundation.ID); err != nil {
		t.Fatal(err)
	}
	if err := services.SubmitCurriculumProposal(database, admin.ID, baseProposal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := services.AcceptCurriculumProposal(database, admin.ID, baseProposal.ID); err != nil {
		t.Fatal(err)
	}

	learnerSession, closeLearner := connectIntegrationHTTPMCP(t, database, learner)
	defer closeLearner()
	if learnerSession.InitializeResult() == nil || learnerSession.InitializeResult().Instructions == "" {
		t.Fatal("HTTP MCP discovery did not advertise instructions")
	}

	curriculumResult := callIntegrationTool[curriculumOverview](t, learnerSession, "get_curriculum", map[string]any{})
	if !curriculumResult.OK || curriculumResult.Data == nil || len(curriculumResult.Data.Units) != 2 {
		t.Fatalf("curriculum result = %#v", curriculumResult)
	}
	pathResult := callIntegrationTool[learningPath](t, learnerSession, "create_learning_path", map[string]any{
		"name": "Agent path", "target_unit_ids": []int64{advanced.ID},
	})
	if !pathResult.OK || pathResult.Data == nil {
		t.Fatalf("create path result = %#v", pathResult)
	}
	recommendations := callIntegrationTool[recommendationsOutput](t, learnerSession, "get_recommendations", map[string]any{})
	if !recommendations.OK || len(recommendations.Data.Recommendations) != 1 ||
		len(recommendations.Data.Recommendations[0].Units) != 1 || recommendations.Data.Recommendations[0].Units[0].ID != foundation.ID {
		t.Fatalf("initial recommendations = %#v", recommendations)
	}
	completion := callIntegrationTool[progress](t, learnerSession, "set_progress", map[string]any{
		"unit_id": foundation.ID, "completed": true,
	})
	if !completion.OK || !completion.Data.Direct || !completion.Data.Completed || completion.Data.Recognized {
		t.Fatalf("completion = %#v", completion)
	}
	recommendations = callIntegrationTool[recommendationsOutput](t, learnerSession, "get_recommendations", map[string]any{})
	if len(recommendations.Data.Recommendations[0].Units) != 1 || recommendations.Data.Recommendations[0].Units[0].ID != advanced.ID {
		t.Fatalf("recommendations after progress = %#v", recommendations)
	}

	otherSession, closeOther := connectIntegrationMCP(t, database, otherLearner)
	defer closeOther()
	otherPaths := callIntegrationTool[learningPathsOutput](t, otherSession, "get_learning_paths", map[string]any{})
	if !otherPaths.OK || len(otherPaths.Data.LearningPaths) != 0 {
		t.Fatalf("another user's learning paths leaked: %#v", otherPaths)
	}
	learnerTools, err := learnerSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range learnerTools.Tools {
		if tool.Name == "create_proposal" {
			t.Fatal("non-administrator discovered create_proposal")
		}
	}

	adminSession, closeAdmin := connectIntegrationMCP(t, database, admin)
	defer closeAdmin()
	proposalResult := callIntegrationTool[proposal](t, adminSession, "create_proposal", map[string]any{
		"title": "Agent-authored extension", "rationale": "Exercise proposal tools through MCP.",
	})
	proposalID := proposalResult.Data.ID
	unitResult := callIntegrationTool[unit](t, adminSession, "create_proposal_unit", map[string]any{
		"proposal_id": proposalID, "name": "Applied topic", "content": "Apply the advanced topic.",
	})
	createdUnitID := unitResult.Data.ID
	for _, update := range []struct {
		name    string
		content string
	}{
		{name: "Refined applied topic", content: "Apply the refined topic."},
		{name: "Final applied topic", content: "Apply the final topic."},
	} {
		updated := callIntegrationTool[unit](t, adminSession, "update_proposal_unit", map[string]any{
			"proposal_id": proposalID, "unit_id": createdUnitID,
			"name": update.name, "content": update.content,
		})
		if !updated.OK || updated.Data.ID != createdUnitID ||
			updated.Data.Name != update.name || updated.Data.Content != update.content {
			t.Fatalf("updated proposal unit = %#v", updated)
		}
	}
	normalized := callIntegrationTool[proposal](t, adminSession, "get_proposal", map[string]any{"proposal_id": proposalID})
	var createdChanges int
	for _, change := range normalized.Data.Changes {
		if change.UnitID == nil || *change.UnitID != createdUnitID {
			continue
		}
		if change.Kind != "create_unit" || change.UnitName != "Final applied topic" || change.UnitContent != "Apply the final topic." {
			t.Fatalf("MCP unit update left edit-history changes: %#v", normalized.Data.Changes)
		}
		createdChanges++
	}
	if createdChanges != 1 {
		t.Fatalf("MCP unit creation count = %d, changes = %#v", createdChanges, normalized.Data.Changes)
	}
	dependencyResult := callIntegrationTool[dependencyState](t, adminSession, "set_proposal_dependency", map[string]any{
		"proposal_id": proposalID, "unit_id": createdUnitID, "prerequisite_id": advanced.ID, "present": true,
	})
	if !dependencyResult.OK || !dependencyResult.Data.Present {
		t.Fatalf("dependency result = %#v", dependencyResult)
	}
	// The convergent dependency action is safe to retry.
	dependencyResult = callIntegrationTool[dependencyState](t, adminSession, "set_proposal_dependency", map[string]any{
		"proposal_id": proposalID, "unit_id": createdUnitID, "prerequisite_id": advanced.ID, "present": true,
	})
	if !dependencyResult.OK {
		t.Fatalf("retried dependency result = %#v", dependencyResult)
	}
	recognitionResult := callIntegrationTool[proposal](t, adminSession, "add_proposal_recognition", map[string]any{
		"proposal_id": proposalID, "source_unit_ids": []int64{advanced.ID}, "target_unit_ids": []int64{createdUnitID},
	})
	if !recognitionResult.OK {
		t.Fatalf("recognition result = %#v", recognitionResult)
	}
	// An identical recognition is also a successful no-op.
	recognitionResult = callIntegrationTool[proposal](t, adminSession, "add_proposal_recognition", map[string]any{
		"proposal_id": proposalID, "source_unit_ids": []int64{advanced.ID}, "target_unit_ids": []int64{createdUnitID},
	})
	if !recognitionResult.OK {
		t.Fatalf("retried recognition result = %#v", recognitionResult)
	}
	if got := recognitionChangeCount(*recognitionResult.Data); got != 1 {
		t.Fatalf("recognition change count = %d, want 1", got)
	}
	rebase := callIntegrationTool[rebasePlan](t, adminSession, "get_proposal_rebase", map[string]any{"proposal_id": proposalID})
	if !rebase.OK || rebase.Data.Status != services.ProposalRebaseCurrent {
		t.Fatalf("rebase plan = %#v", rebase)
	}
	resolved := callIntegrationTool[proposal](t, adminSession, "resolve_proposal_rebase", map[string]any{
		"proposal_id": proposalID, "resolutions": []any{},
	})
	if !resolved.OK {
		t.Fatalf("current rebase no-op = %#v", resolved)
	}
	cycle := callIntegrationTool[dependencyState](t, adminSession, "set_proposal_dependency", map[string]any{
		"proposal_id": proposalID, "unit_id": foundation.ID, "prerequisite_id": advanced.ID, "present": true,
	})
	if cycle.OK || cycle.Error == nil || cycle.Error.Code != "conflict" {
		t.Fatalf("cycle response = %#v", cycle)
	}
	unconfirmed := callIntegrationTool[proposal](t, adminSession, "submit_proposal", map[string]any{
		"proposal_id": proposalID, "expected_title": proposalResult.Data.Title, "confirmed": false,
	})
	if unconfirmed.OK || unconfirmed.Error.Code != "confirmation_required" {
		t.Fatalf("unconfirmed submission = %#v", unconfirmed)
	}
	submitted := callIntegrationTool[proposal](t, adminSession, "submit_proposal", map[string]any{
		"proposal_id": proposalID, "expected_title": proposalResult.Data.Title, "confirmed": true,
	})
	if !submitted.OK || submitted.Data.Status != "submitted" {
		t.Fatalf("submission result = %#v", submitted)
	}
	published := callIntegrationTool[proposal](t, adminSession, "accept_proposal", map[string]any{"proposal_id": proposalID})
	if !published.OK || published.Data.Status != "accepted" {
		t.Fatalf("acceptance result = %#v", published)
	}
	readCreated := callIntegrationTool[unit](t, adminSession, "get_unit", map[string]any{"unit_id": createdUnitID})
	if !readCreated.OK || readCreated.Data.Name != "Final applied topic" || readCreated.Data.Content != "Apply the final topic." {
		t.Fatalf("published unit = %#v", readCreated)
	}
}

func TestMCPOAuthTokensUsePKCEAudienceAndRotation(t *testing.T) {
	database := openMCPIntegrationDatabase(t)
	user := registerMCPIntegrationUser(t, database, "OAuth Learner", "oauth-learner@example.com", false)
	verifier := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	resource := "https://curriculum.example/mcp"
	clientID := "https://client.example/oauth.json"
	code, err := db.CreateOAuthAuthorizationCode(database, models.OAuthAuthorizationGrant{
		UserID: user.ID, ClientID: clientID, ClientName: "MCP integration client", RedirectURI: "https://client.example/callback",
		Resource: resource, Scope: "mcp", CodeChallenge: challenge,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.ExchangeOAuthAuthorizationCode(database, code, clientID, "https://client.example/callback", resource, verifier+"wrong"); !errors.Is(err, services.ErrInvalidOAuthGrant) {
		t.Fatalf("wrong verifier error = %v", err)
	}
	pair, err := services.ExchangeOAuthAuthorizationCode(database, code, clientID, "https://client.example/callback", resource, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if authenticated, err := db.AuthenticateOAuthAccessToken(database, pair.AccessToken, resource); err != nil || authenticated == nil || authenticated.ID != user.ID {
		t.Fatalf("authenticate OAuth token = %#v, %v", authenticated, err)
	}
	if authenticated, err := db.AuthenticateOAuthAccessToken(database, pair.AccessToken, "https://other.example/mcp"); err != nil || authenticated != nil {
		t.Fatalf("wrong audience authentication = %#v, %v", authenticated, err)
	}
	rotated, err := services.RefreshOAuthAccessToken(database, pair.RefreshToken, clientID, resource)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.AccessToken == pair.AccessToken || rotated.RefreshToken == pair.RefreshToken {
		t.Fatal("OAuth refresh did not rotate both tokens")
	}
	if _, err := services.RefreshOAuthAccessToken(database, pair.RefreshToken, clientID, resource); !errors.Is(err, services.ErrInvalidOAuthGrant) {
		t.Fatalf("reused refresh token error = %v", err)
	}
	var rawRows int
	if err := database.QueryRow(`
		SELECT count(*) FROM oauth_access_tokens WHERE token_hash = $1
	`, pair.AccessToken).Scan(&rawRows); err != nil {
		t.Fatal(err)
	}
	if rawRows != 0 {
		t.Fatal("raw OAuth access token was persisted")
	}
}

func openMCPIntegrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL MCP integration tests")
	}
	database, err := sql.Open("postgres", connectionString)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	schema := fmt.Sprintf("mcp_test_%d", time.Now().UnixNano())
	if _, err := database.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
		_ = database.Close()
	})
	if _, err := database.Exec("SET search_path TO " + schema); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func registerMCPIntegrationUser(t *testing.T, database *sql.DB, name, email string, admin bool) *models.User {
	t.Helper()
	user, err := services.RegisterLocalUser(database, name, email, "long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if admin {
		if _, err := database.Exec(`UPDATE users SET is_admin = TRUE WHERE id = $1`, user.ID); err != nil {
			t.Fatal(err)
		}
		user.IsAdmin = true
	}
	return user
}

func connectIntegrationHTTPMCP(t *testing.T, database *sql.DB, user *models.User) (*mcp.ClientSession, func()) {
	t.Helper()
	token, err := services.CreateAPIToken(database, user.ID, "MCP integration")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(database, "https://curriculum.example"))
	httpClient := &http.Client{Transport: bearerTransport{token: token.Token, base: http.DefaultTransport}}
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-agent", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: server.URL, HTTPClient: httpClient, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return session, func() { _ = session.Close(); server.Close() }
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (transport bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+transport.token)
	return transport.base.RoundTrip(clone)
}

func connectIntegrationMCP(t *testing.T, database *sql.DB, user *models.User) (*mcp.ClientSession, func()) {
	t.Helper()
	return connectIntegrationHTTPMCP(t, database, user)
}

func callIntegrationTool[T any](t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) toolOutput[T] {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output toolOutput[T]
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("decode %s output %s: %v", name, encoded, err)
	}
	return output
}

func recognitionChangeCount(value proposal) int {
	count := 0
	for _, change := range value.Changes {
		if change.Kind == "recognition" {
			count++
		}
	}
	return count
}
