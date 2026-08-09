package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestScriptSaveFlow(t *testing.T) {
	s := newTestServerWithDB(t)
	item := seedItem(t, s)
	itemPath := "/content/" + strconv.FormatInt(item.ID, 10)

	// GET detail for a CSRF token.
	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, itemPath, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET detail = %d, want 200", getRec.Code)
	}
	token := getCSRFCookie(getRec.Result().Cookies())

	// POST the script.
	form := url.Values{
		"body":       {"Hello world this is my script."},
		"wpm":        {"120"},
		"csrf_token": {token},
	}
	req := httptest.NewRequest(http.MethodPost, itemPath+"/script", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	postRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(postRec, req)

	if postRec.Code != http.StatusSeeOther {
		t.Fatalf("POST script = %d, want 303", postRec.Code)
	}
	if loc := postRec.Header().Get("Location"); loc != itemPath {
		t.Errorf("Location = %q, want %q", loc, itemPath)
	}

	// Detail now shows the saved body and word count.
	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, itemPath, nil))
	body := listRec.Body.String()
	if !strings.Contains(body, "Hello world this is my script.") {
		t.Error("saved script body not shown on detail")
	}
	if !strings.Contains(body, "6 words") {
		t.Error("word count not shown on detail")
	}
}

func TestScriptSaveNotFound(t *testing.T) {
	s := newTestServerWithDB(t)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/content", nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	form := url.Values{"body": {"x"}, "wpm": {"150"}, "csrf_token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/content/999999/script", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
