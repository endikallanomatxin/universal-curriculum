package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
