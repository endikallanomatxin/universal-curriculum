package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSafeRedirectPath(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "/account", want: "/account"},
		{value: "https://example.com", want: "/"},
		{value: "//example.com", want: "/"},
		{value: "", want: "/"},
	} {
		if got := safeRedirectPath(test.value, "/"); got != test.want {
			t.Errorf("safeRedirectPath(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestSessionCookieSecurity(t *testing.T) {
	recorder := httptest.NewRecorder()
	setSessionCookie(recorder, "secret", true, time.Unix(1_000, 0))
	cookie := recorder.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.SameSite == 0 {
		t.Fatalf("insecure session cookie: %+v", cookie)
	}
}

func TestReplaceSessionCookiePreservesOtherCookies(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "session_token=old; preference=compact")
	got := replaceSessionCookie(request.Cookies(), "new")
	if !strings.Contains(got, "session_token=new") || !strings.Contains(got, "preference=compact") {
		t.Fatalf("replaceSessionCookie() = %q", got)
	}
}
