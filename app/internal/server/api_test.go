package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedOpenAPIContractMatchesCanonicalSource(t *testing.T) {
	canonical, err := os.ReadFile("../../../docs/openapi.yaml")
	if os.IsNotExist(err) {
		t.Skip("canonical contract is outside this build context")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, openAPIContract) {
		t.Fatal("embedded OpenAPI contract differs from docs/openapi.yaml")
	}
}

func TestExperimentalAPIInfoAndErrorsAreJSON(t *testing.T) {
	application := (&Server{}).routes()

	request := httptest.NewRequest(http.MethodGet, "/api", nil)
	response := httptest.NewRecorder()
	application.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("API info response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	var info map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil || info["status"] != "experimental" {
		t.Fatalf("API info = %v, error %v", info, err)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	response = httptest.NewRecorder()
	application.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("unknown API response = %d %s", response.Code, response.Body.String())
	}
}

func TestAPIRejectsUnknownAndRepeatedQueryParameters(t *testing.T) {
	for _, target := range []string{"/api/units?unknown=1", "/api/units?limit=1&limit=2"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		(&Server{}).routes().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_query"`) {
			t.Fatalf("%s response = %d %s", target, response.Code, response.Body.String())
		}
	}
}

func TestUnsupportedAPIMethodReturnsJSON(t *testing.T) {
	application := (&Server{}).routes()
	for _, test := range []struct {
		method string
		target string
		allow  string
	}{
		{http.MethodPost, "/api/units", "GET, HEAD"},
		{http.MethodPatch, "/api/units", "GET, HEAD"},
		{http.MethodTrace, "/api/units", "GET, HEAD"},
		{http.MethodHead, "/api/progress/1", "PUT"},
	} {
		request := httptest.NewRequest(test.method, test.target, nil)
		response := httptest.NewRecorder()
		application.ServeHTTP(response, request)
		if response.Code != http.StatusMethodNotAllowed ||
			response.Header().Get("Allow") != test.allow ||
			response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
			!strings.Contains(response.Body.String(), `"code":"method_not_allowed"`) {
			t.Fatalf("unsupported API method = %d, allow %q, content type %q: %s", response.Code, response.Header().Get("Allow"), response.Header().Get("Content-Type"), response.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/unknown", nil)
	response := httptest.NewRecorder()
	application.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("unknown API resource = %d %s", response.Code, response.Body.String())
	}
}

func TestDecodeAPIJSONIsStrict(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}
	for _, test := range []struct {
		contentType string
		body        string
	}{
		{"text/plain", `{"name":"valid"}`},
		{"application/json", `{"name":"valid","extra":true}`},
		{"application/json", `{"name":"valid"} {}`},
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/example", strings.NewReader(test.body))
		request.Header.Set("Content-Type", test.contentType)
		if err := decodeAPIJSON(httptest.NewRecorder(), request, &input{}); err == nil {
			t.Fatalf("decodeAPIJSON(%q, %q) accepted invalid request", test.contentType, test.body)
		}
	}
}

func TestPrivateAPIRequiresBearerWithoutAcceptingSessionContext(t *testing.T) {
	handler := (&Server{}).requireAPIToken(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("private handler was called")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/progress", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("private API response = %d, authenticate %q", response.Code, response.Header().Get("WWW-Authenticate"))
	}
}
