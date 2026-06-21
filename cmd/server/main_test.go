package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestRequireAuthRedirectsPagesWithoutLogin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireAuth(next, "api-secret", "session-secret", true)

	for _, path := range []string{"/", "/orders"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()

		handler.ServeHTTP(resp, req)

		if resp.Code != http.StatusFound {
			t.Fatalf("expected %s to redirect, got %d", path, resp.Code)
		}
	}
}

func TestRequireAuthDoesNotRedirectAPI(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireAuth(next, "api-secret", "session-secret", true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected API without key to return 401, got %d", resp.Code)
	}
	if location := resp.Header().Get("Location"); location != "" {
		t.Fatalf("expected no redirect location, got %q", location)
	}
}

func TestRequireAuthAllowsAPIKey(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireAuth(next, "api-secret", "session-secret", true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("X-API-Key", "api-secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected API with key to pass, got %d", resp.Code)
	}
}

func TestRequireAuthAllowsPagesWithLogin(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireAuth(next, "api-secret", "session-secret", true)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sessionCookie(makeSessionValue("admin", "session-secret", time.Now().Add(time.Hour))))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected page with login to pass, got %d", resp.Code)
	}
}

func TestRequireAuthDoesNotProtectDisabledAPITestPage(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler := requireAuth(next, "api-secret", "session-secret", false)

	req := httptest.NewRequest(http.MethodGet, "/api-test", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected disabled api-test page to return 404, got %d", resp.Code)
	}
}

func TestStatusRecorderCapturesStatus(t *testing.T) {
	resp := httptest.NewRecorder()
	recorder := &statusRecorder{
		ResponseWriter: resp,
		status:         http.StatusOK,
	}

	recorder.WriteHeader(http.StatusCreated)

	if recorder.status != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, recorder.status)
	}
}

func TestParseOptionalInt64(t *testing.T) {
	empty, err := parseOptionalInt64("")
	if err != nil || empty != 0 {
		t.Fatalf("expected empty value to parse as 0, got value=%d err=%v", empty, err)
	}

	value, err := parseOptionalInt64("12345")
	if err != nil || value != 12345 {
		t.Fatalf("expected value 12345, got value=%d err=%v", value, err)
	}
}

func TestParseBoolEnv(t *testing.T) {
	t.Setenv("BOOL_TEST_EMPTY", "")
	if !parseBoolEnv("BOOL_TEST_EMPTY", true) {
		t.Fatal("expected empty bool env to use fallback true")
	}

	t.Setenv("BOOL_TEST_FALSE", "false")
	if parseBoolEnv("BOOL_TEST_FALSE", true) {
		t.Fatal("expected false string to disable bool env")
	}

	t.Setenv("BOOL_TEST_TRUE", "true")
	if !parseBoolEnv("BOOL_TEST_TRUE", false) {
		t.Fatal("expected true string to enable bool env")
	}

	_ = os.Unsetenv("BOOL_TEST_MISSING")
	if parseBoolEnv("BOOL_TEST_MISSING", false) {
		t.Fatal("expected missing bool env to use fallback false")
	}
}

func TestLoginRequiresCaptcha(t *testing.T) {
	handler := loginHandler("admin", "password", "session-secret")

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
		"next":     {"/"},
	}
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect on failed login, got %d", resp.Code)
	}
	if location := resp.Header().Get("Location"); location == "/" {
		t.Fatal("expected login without captcha not to redirect to home")
	}
}

func TestValidCaptcha(t *testing.T) {
	expiresAt := time.Now().Add(time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.AddCookie(&http.Cookie{
		Name:  "bookkeeping_captcha",
		Value: makeCaptchaValue("12", "session-secret", expiresAt),
	})

	if !validCaptcha(req, "session-secret", "12") {
		t.Fatal("expected captcha to be valid")
	}
	if validCaptcha(req, "session-secret", "11") {
		t.Fatal("expected wrong captcha to be invalid")
	}
}
