package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
)

// Loopback-only trust boundary aside, we still defend state-changing requests
// with a same-origin check and a double-submit CSRF token (see docs/SPEC.md §9).

const (
	csrfCookie = "pbccreate_csrf"
	csrfHeader = "X-CSRF-Token"
	csrfField  = "csrf_token"
)

type ctxKey int

const csrfTokenKey ctxKey = 0

// securityHeaders sets a strict, self-hosted-only CSP and hardening headers on
// every response.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; base-uri 'self'; form-action 'self'; " +
		"frame-ancestors 'none'; img-src 'self' data:; object-src 'none'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		// same-origin (not no-referrer): a referrer is never leaked cross-origin
		// (the app makes no external requests anyway), but same-origin requests
		// keep a Referer so the same-origin check below has a value to verify when
		// the browser omits the Origin header on form POSTs.
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// csrf ensures a CSRF token cookie exists, exposes the token via the request
// context (for form rendering), and — for unsafe methods — enforces same-origin
// and a matching token.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.ensureCSRFToken(w, r)
		r = r.WithContext(context.WithValue(r.Context(), csrfTokenKey, token))

		if isUnsafeMethod(r.Method) {
			if !sameOrigin(r) {
				s.log.Warn("csrf: cross-origin request rejected",
					"method", r.Method, "path", r.URL.Path, "origin", r.Header.Get("Origin"))
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			if !validCSRF(r, token) {
				s.log.Warn("csrf: token mismatch", "method", r.Method, "path", r.URL.Path)
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ensureCSRFToken returns the request's CSRF token, minting and setting a cookie
// if absent.
func (s *Server) ensureCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return c.Value
	}
	token := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		// Secure is intentionally false: the app is served over loopback HTTP.
	})
	return token
}

func isUnsafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// sameOrigin verifies that a state-changing request did not originate from
// another site. When an Origin (or, failing that, Referer) header is present its
// host must match the target Host — this rejects genuine cross-origin attacks.
//
// When BOTH headers are absent the request is allowed: privacy-hardened browsers
// legitimately omit Origin on same-origin form POSTs and may strip Referer, so a
// missing origin is not proof of an attack. The double-submit CSRF token remains
// the primary defense — a cross-site request cannot carry the SameSite=Strict
// cookie, so its token can never match (a cross-site attacker also sends an
// Origin header, which is caught above). An unparseable header is treated as a
// mismatch.
func sameOrigin(r *http.Request) bool {
	src := r.Header.Get("Origin")
	if src == "" {
		src = r.Header.Get("Referer")
	}
	if src == "" {
		return true // no origin information; rely on the CSRF token
	}
	u, err := url.Parse(src)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Host == r.Host
}

// validCSRF compares the submitted token (header or form field) to the expected
// token in constant time.
func validCSRF(r *http.Request, expected string) bool {
	got := r.Header.Get(csrfHeader)
	if got == "" {
		got = r.PostFormValue(csrfField)
	}
	if got == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// csrfToken returns the token stored on the request context (empty if none).
func csrfToken(r *http.Request) string {
	if v, ok := r.Context().Value(csrfTokenKey).(string); ok {
		return v
	}
	return ""
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is not recoverable; panicking is correct here.
		panic("server: read random: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
