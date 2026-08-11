package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

func TestContentScaffoldButton(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	ch, _ := store.CreateChannel(ctx, s.db, "TUL", "youtube")
	item, _ := store.CreateContentItem(ctx, s.db, ch.ID, "video", "single_cam", "Best VPN 2026")
	_, _ = store.SaveScript(ctx, s.db, item.ID, "A VPN protects your traffic.", 150)
	_, _ = store.AddShot(ctx, s.db, item.ID, store.Shot{Description: "wide", Scene: "Intro", Camera: "A-Cam"})
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// With no project root set, the section prompts to configure it and the
	// scaffold POST redirects with ?scaffold=noroot (nothing created).
	if !strings.Contains(getRec.Body.String(), "No project root set") {
		t.Error("detail should prompt to set a project root when unset")
	}
	noroot := postForm(t, s, base+"/scaffold", token, url.Values{})
	if noroot.Code != http.StatusSeeOther || !strings.Contains(noroot.Header().Get("Location"), "scaffold=noroot") {
		t.Fatalf("scaffold without root = %d loc=%q, want 303 ?scaffold=noroot", noroot.Code, noroot.Header().Get("Location"))
	}

	// Configure a project root (stored setting) and scaffold.
	root := t.TempDir()
	if err := store.SetSetting(ctx, s.db, store.SettingProjectRoot, root); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	// The detail page now shows the target path and the button.
	getRec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec2, httptest.NewRequest(http.MethodGet, base, nil))
	if !strings.Contains(getRec2.Body.String(), "Scaffold project folders") ||
		!strings.Contains(getRec2.Body.String(), filepath.Join(root, "Best VPN 2026")) {
		t.Error("detail missing scaffold button / target path when root set")
	}

	ok := postForm(t, s, base+"/scaffold", token, url.Values{})
	if ok.Code != http.StatusSeeOther || !strings.Contains(ok.Header().Get("Location"), "scaffold=ok") {
		t.Fatalf("scaffold = %d loc=%q, want 303 ?scaffold=ok", ok.Code, ok.Header().Get("Location"))
	}

	// The tree and Docs exist on disk under the project root.
	projectDir := filepath.Join(root, "Best VPN 2026")
	for _, rel := range []string{
		filepath.Join("01_Footage", "A-Cam"), "04_Project",
		filepath.Join("07_Docs", "script.md"), filepath.Join("07_Docs", "shotlist.md"),
	} {
		if _, err := os.Stat(filepath.Join(projectDir, rel)); err != nil {
			t.Errorf("expected scaffolded %q: %v", rel, err)
		}
	}
	// Docs content came from the item's script and shot list.
	if b, _ := os.ReadFile(filepath.Join(projectDir, "07_Docs", "shotlist.md")); !strings.Contains(string(b), "Camera: A-Cam") {
		t.Errorf("shotlist.md missing shot metadata: %s", b)
	}

	// The result notice renders after the redirect.
	doneRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(doneRec, httptest.NewRequest(http.MethodGet, base+"?scaffold=ok", nil))
	if !strings.Contains(doneRec.Body.String(), "Project folders scaffolded.") {
		t.Error("scaffold result notice not shown")
	}
}
