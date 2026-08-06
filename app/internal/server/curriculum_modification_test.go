package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCurriculumModificationUsesProtectedRoutes(t *testing.T) {
	handler := (&Server{}).routes()

	for _, test := range []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/curriculum-modification"},
		{method: http.MethodPost, target: "/curriculum-modification/proposals"},
		{method: http.MethodPost, target: "/curriculum-modification/proposals/1/rebase"},
	} {
		request := httptest.NewRequest(test.method, test.target, nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusSeeOther {
			t.Errorf("%s %s status = %d, want %d", test.method, test.target, recorder.Code, http.StatusSeeOther)
		}
		wantLocation := "/auth/login?next=" + url.QueryEscape(test.target)
		if location := recorder.Header().Get("Location"); location != wantLocation {
			t.Errorf("%s %s location = %q, want %q", test.method, test.target, location, wantLocation)
		}
	}
}
