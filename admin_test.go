package sseserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// it should serve a HTML index page
func TestAdminHTTPIndex(t *testing.T) {
	s := NewServer()
	defer s.hub.Shutdown()

	req, err := http.NewRequest("GET", "/admin/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := adminHandler(s)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// it should expose a REST JSON status API
func TestAdminHTTPStatusAPI(t *testing.T) {
	s := NewServer()
	defer s.hub.Shutdown()

	req, err := http.NewRequest("GET", "/admin/status.json", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := adminHandler(s)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	if ctype := rr.Header().Get("Content-Type"); ctype != "application/json" {
		t.Errorf("content type header does not match: got %v want %v",
			ctype, "application/json")
	}

	// TODO: test the actual output JSON
	// TODO: perhaps test proper clients show up as well!

}

// it should disable all HTTP endpoints based on ServerOptions
func TestAdminDisableEndpoints(t *testing.T) {
	s := NewServer()
	defer s.hub.Shutdown()
	s.Options.DisableAdminEndpoints = true

	for _, path := range []string{"/admin/", "/admin/status.json"} {
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := adminHandler(s)
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusForbidden {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusForbidden)
		}

		expected := "403 admin endpoint disabled\n"
		if rr.Body.String() != expected {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), expected)
		}
	}
}
