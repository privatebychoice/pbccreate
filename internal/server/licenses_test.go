package server

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"go.privatebychoice.com/pbccreate/internal/store"
)

// uploadLicense posts a multipart license-file upload with the CSRF token/cookie.
func uploadLicense(t *testing.T, s *Server, path, token, filename string, data []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("csrf_token", token)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", "http://"+req.Host)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestLicenseUploadDownloadDelete(t *testing.T) {
	s := newTestServerWithDB(t)
	ctx := context.Background()
	item := seedItem(t, s)
	prov, _ := store.CreateAssetProvider(ctx, s.db, store.AssetProvider{Name: "Artlist", ServiceType: "music"})
	base := "/content/" + strconv.FormatInt(item.ID, 10)

	getRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, base, nil))
	token := getCSRFCookie(getRec.Result().Cookies())

	// Reject an unsupported extension.
	if rec := uploadLicense(t, s, base+"/licenses", token, "malware.exe", []byte("nope"), nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad-ext upload = %d, want 400", rec.Code)
	}

	// Upload a valid text license linked to the provider.
	content := []byte("LICENSE TERMS: royalty-free, credit not required.\n")
	up := uploadLicense(t, s, base+"/licenses", token, "terms.txt", content, map[string]string{
		"description": "annual terms",
		"applies_to":  "intro music",
		"provider_id": strconv.FormatInt(prov.ID, 10),
	})
	if up.Code != http.StatusSeeOther {
		t.Fatalf("upload = %d, want 303", up.Code)
	}

	files, _ := store.ListLicenseFiles(ctx, s.db, item.ID)
	if len(files) != 1 {
		t.Fatalf("want 1 license file, got %d", len(files))
	}
	lf := files[0]
	if lf.StoredName == "" || lf.SizeBytes != int64(len(content)) {
		t.Fatalf("stored metadata wrong: %+v", lf)
	}
	if lf.ProviderName != "Artlist" || lf.AppliesTo != "intro music" {
		t.Fatalf("license fields wrong: %+v", lf)
	}

	// Download returns the bytes as an attachment with nosniff.
	dlURL := base + "/licenses/" + strconv.FormatInt(lf.ID, 10) + "/download"
	dl := httptest.NewRecorder()
	s.Handler().ServeHTTP(dl, httptest.NewRequest(http.MethodGet, dlURL, nil))
	if dl.Code != http.StatusOK {
		t.Fatalf("download = %d, want 200", dl.Code)
	}
	if cd := dl.Header().Get("Content-Disposition"); cd == "" || !bytes.Contains([]byte(cd), []byte("attachment")) {
		t.Errorf("content-disposition = %q, want attachment", cd)
	}
	if ct := dl.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("content-type = %q, want application/octet-stream", ct)
	}
	if xcto := dl.Header().Get("X-Content-Type-Options"); xcto != "nosniff" {
		t.Errorf("x-content-type-options = %q, want nosniff", xcto)
	}
	if !bytes.Equal(dl.Body.Bytes(), content) {
		t.Errorf("downloaded bytes differ from upload")
	}

	// Delete removes the row.
	del := postForm(t, s, base+"/licenses/"+strconv.FormatInt(lf.ID, 10)+"/delete", token, url.Values{})
	if del.Code != http.StatusSeeOther {
		t.Fatalf("delete = %d, want 303", del.Code)
	}
	if files, _ := store.ListLicenseFiles(ctx, s.db, item.ID); len(files) != 0 {
		t.Errorf("license file not deleted: %d remain", len(files))
	}
	// The bytes are gone too: a second download 404s.
	dl2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(dl2, httptest.NewRequest(http.MethodGet, dlURL, nil))
	if dl2.Code != http.StatusNotFound {
		t.Errorf("download after delete = %d, want 404", dl2.Code)
	}
}
