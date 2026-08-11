package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	// A migrated DB is attached: the landing page (and other shared pages) now
	// read app settings, so a nil DB is no longer a valid server.
	return newTestServerWithDB(t)
}

func TestVersionEndpoint(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/version", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["version"]; !ok {
		t.Error("response missing \"version\"")
	}
	if _, ok := body["build"]; !ok {
		t.Error("response missing \"build\"")
	}
}

func TestHomeRendersWithSecurityHeadersAndCSRFCookie(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pbccreate") {
		t.Error("home body missing expected content")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("missing/weak CSP: %q", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options: nosniff")
	}
	if getCSRFCookie(rec.Result().Cookies()) == "" {
		t.Error("no CSRF cookie set on GET")
	}
}

func TestStaticAssetsServed(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/app.css", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pbccreate") {
		t.Error("app.css not served from embedded FS")
	}
}

func TestUnsafeMethodRejectedWithoutCSRF(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "http://"+req.Host) // same-origin, but no token
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (missing CSRF token)", rec.Code)
	}
}

func TestUnsafeMethodRejectedCrossOrigin(t *testing.T) {
	s := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "http://evil.example")
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (cross-origin)", rec.Code)
	}
}

func TestUnsafeMethodPassesWithSameOriginAndToken(t *testing.T) {
	s := newTestServer(t)

	// GET first to obtain a CSRF cookie/token.
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/", nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	if token == "" {
		t.Fatal("expected a CSRF cookie from GET")
	}

	// POST with the same cookie, matching header token, and same origin.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "http://"+req.Host)
	req.Header.Set(csrfHeader, token)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	// The middleware must let it through; the mux then 405s (no POST route),
	// which proves CSRF/same-origin did not block it.
	if rec.Code == http.StatusForbidden {
		t.Fatalf("valid same-origin+token POST was rejected (403)")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (passed middleware, no POST route)", rec.Code)
	}
}

// TestUnsafeMethodPassesWithoutOriginHeader reproduces the real-browser case
// (e.g. a privacy-hardened Firefox) that omits the Origin header on a same-origin
// form POST and, under the app's Referrer-Policy, sends no Referer either. With a
// valid double-submit CSRF token the request must still be accepted.
func TestUnsafeMethodPassesWithoutOriginHeader(t *testing.T) {
	s := newTestServer(t)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/", nil))
	token := getCSRFCookie(getRec.Result().Cookies())
	if token == "" {
		t.Fatal("expected a CSRF cookie from GET")
	}

	// No Origin and no Referer — only the token cookie + form field.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(csrfHeader, token)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("header-less same-origin POST with a valid token was rejected (403)")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 (passed middleware, no POST route)", rec.Code)
	}
}

// TestUnsafeMethodRejectedCrossOriginWithoutToken confirms a cross-origin request
// that also lacks the CSRF cookie is still blocked (both layers hold).
func TestUnsafeMethodRejectedCrossOriginWithoutToken(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set(csrfHeader, "anything")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (cross-origin)", rec.Code)
	}
}

func getCSRFCookie(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name == csrfCookie {
			return c.Value
		}
	}
	return ""
}
