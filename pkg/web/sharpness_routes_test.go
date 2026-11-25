package web_test

import (
	"acuity/pkg/db"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func formRequest(t *testing.T, url string, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestSharpnessRoutes(t *testing.T) {
	_, mux, sDB, _ := setupTestServer(t)
	if err := sDB.InsertGallery("g", "/"); err != nil {
		t.Fatal(err)
	}
	f := func(v float64) *float64 { return &v }
	if err := sDB.InsertImage([]db.SImage{
		{GalleryID: 1, FilePath: "/a.jpg", Blurhash: "x", Sharpness: f(5)},
		{GalleryID: 1, FilePath: "/b.jpg", Blurhash: "x", Sharpness: f(10)},
		{GalleryID: 1, FilePath: "/pending.jpg", Blurhash: "x"},
		{GalleryID: 1, FilePath: "/sub/f.jpg", Blurhash: "x", Sharpness: f(2)},
	}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		req      *http.Request
		wantCode int
		wantBody string
	}{
		{"stats", httptest.NewRequest(http.MethodGet, "/api/gallery/g/sharpness", nil), http.StatusOK, `{"scored":3,"pending":1}`},
		{"stats folder", httptest.NewRequest(http.MethodGet, "/api/gallery/g/sharpness?folder=/sub/", nil), http.StatusOK, `{"scored":1,"pending":0}`},
		{"stats global", httptest.NewRequest(http.MethodGet, "/api/gallery/global/sharpness", nil), http.StatusBadRequest, ""},
		{"stats unknown", httptest.NewRequest(http.MethodGet, "/api/gallery/nope/sharpness", nil), http.StatusNotFound, ""},
		{"reject garbage percent", formRequest(t, "/api/gallery/g/reject-blurry", map[string]string{"percent": "abc"}), http.StatusBadRequest, ""},
		{"reject missing percent", formRequest(t, "/api/gallery/g/reject-blurry", nil), http.StatusBadRequest, ""},
		{"reject out of range", formRequest(t, "/api/gallery/g/reject-blurry", map[string]string{"percent": "150"}), http.StatusBadRequest, ""},
		{"reject global", formRequest(t, "/api/gallery/global/reject-blurry", map[string]string{"percent": "10"}), http.StatusBadRequest, ""},
		{"reject unknown", formRequest(t, "/api/gallery/nope/reject-blurry", map[string]string{"percent": "10"}), http.StatusNotFound, ""},
		{"reject folder from body", formRequest(t, "/api/gallery/g/reject-blurry", map[string]string{"percent": "100", "folder": "/sub/"}), http.StatusOK, `{"rejected":1}`},
		{"reject rest", formRequest(t, "/api/gallery/g/reject-blurry", map[string]string{"percent": "100"}), http.StatusOK, `{"rejected":2}`},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, tt.req)
		if rec.Code != tt.wantCode {
			t.Fatalf("%s: got status %d (%s), want %d", tt.name, rec.Code, rec.Body.String(), tt.wantCode)
		}
		if tt.wantBody == "" {
			continue
		}
		var got, want any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("%s: invalid JSON %q: %v", tt.name, rec.Body.String(), err)
		}
		if err := json.Unmarshal([]byte(tt.wantBody), &want); err != nil {
			t.Fatal(err)
		}
		if gotJSON, _ := json.Marshal(got); string(gotJSON) != mustJSON(t, want) {
			t.Fatalf("%s: got body %s, want %s", tt.name, gotJSON, tt.wantBody)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
