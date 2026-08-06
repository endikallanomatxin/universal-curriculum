package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSensitiveAuthResponsesAreNotCachedOrIncludedInReferrers(t *testing.T) {
	handler := sensitiveAuthResponse(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/auth/reset-password?token=secret", nil))

	if cacheControl := response.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
	}
	if referrerPolicy := response.Header().Get("Referrer-Policy"); referrerPolicy != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q, want no-referrer", referrerPolicy)
	}
}
