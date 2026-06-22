package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterRoutesServesCustomNotFoundPage(t *testing.T) {
	mux := http.NewServeMux()
	if err := RegisterRoutes(mux, false); err != nil {
		t.Fatalf("register web routes: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/missing-page", nil)
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.Code)
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	if body := resp.Body.String(); !strings.Contains(body, "信号丢失") {
		t.Fatalf("expected custom 404 page body, got %q", body)
	}
}

func TestRegisterRoutesServesCustomNotFoundForMissingAsset(t *testing.T) {
	mux := http.NewServeMux()
	if err := RegisterRoutes(mux, false); err != nil {
		t.Fatalf("register web routes: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/missing.css", nil)
	resp := httptest.NewRecorder()

	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, resp.Code)
	}
	if body := resp.Body.String(); !strings.Contains(body, "信号丢失") {
		t.Fatalf("expected custom 404 page body, got %q", body)
	}
}
