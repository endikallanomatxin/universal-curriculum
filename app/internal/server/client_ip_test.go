package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func requestWithClientIPConfig(request *http.Request, trustProxy bool) *http.Request {
	var configured *http.Request
	configureClientIP(
		&Server{Config: Config{TrustRenderProxyHeaders: trustProxy}},
		http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			configured = request
		}),
	).ServeHTTP(httptest.NewRecorder(), request)
	return configured
}

func TestClientIPIgnoresProxyHeaderByDefault(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", nil)
	request.RemoteAddr = "198.51.100.7:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")

	address, err := ClientIP(requestWithClientIPConfig(request, false))
	if err != nil {
		t.Fatal(err)
	}
	if address.String() != "198.51.100.7" {
		t.Fatalf("ClientIP() = %s", address)
	}
}

func TestClientIPUsesValidatedRenderProxyHeaderWhenConfigured(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/auth/forgot-password", nil)
	request.RemoteAddr = "10.0.0.4:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.4")

	address, err := ClientIP(requestWithClientIPConfig(request, true))
	if err != nil {
		t.Fatal(err)
	}
	if address.String() != "203.0.113.9" {
		t.Fatalf("ClientIP() = %s", address)
	}
}
