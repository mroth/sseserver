package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mroth/sseserver"
	"github.com/mroth/sseserver/admin"
)

// it should serve a HTML index page
func TestAdminHTTPIndex(t *testing.T) {
	s, err := sseserver.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown()

	req, err := http.NewRequest("GET", "/admin/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := admin.AdminHandler(s)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

// it should expose a REST JSON status API
func TestAdminHTTPStatusAPI(t *testing.T) {
	s, err := sseserver.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown()

	req, err := http.NewRequest("GET", "/admin/status.json", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := admin.AdminHandler(s)
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
