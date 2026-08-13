package web_test

import (
	"acuity/pkg/config"
	"acuity/pkg/db"
	"acuity/pkg/web"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestServer(t *testing.T) (*web.Server, *http.ServeMux, *db.SQLiteClient, *db.VectorClient) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_server.db")
	sDB, err := db.NewSqliteDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite client: %v", err)
	}
	_ = sDB.InitTable()

	vDB := db.NewVectorClient(sDB, nil)
	_ = vDB.InitSchema()

	cfg := &config.Config{
		Port:             "3000",
		ImagesPerPage:    100,
		DefaultSortBy:    "name",
		DefaultSortOrder: "asc",
	}

	server := web.NewServer(sDB, vDB, cfg)
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	return server, mux, sDB, vDB
}

func TestAIStatusEndpoint(t *testing.T) {
	_, mux, _, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/ai/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if _, ok := body["ready"]; !ok {
		t.Errorf("Expected 'ready' field in response")
	}
	if _, ok := body["downloading"]; !ok {
		t.Errorf("Expected 'downloading' field in response")
	}
}

func TestGalleryAPIs(t *testing.T) {
	_, mux, sDB, _ := setupTestServer(t)

	// 1. Create Gallery via POST /api/gallery
	formData := url.Values{
		"name": {"TestGallery"},
		"path": {"/photos/test"},
	}
	req := httptest.NewRequest("POST", "/api/gallery", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/gallery failed with status %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Verify gallery exists in SQLite
	g, err := sDB.GetGalleryByName("TestGallery")
	if err != nil {
		t.Fatalf("Gallery was not found in DB: %v", err)
	}
	if g.Path != "/photos/test" {
		t.Errorf("Expected path '/photos/test', got '%s'", g.Path)
	}

	// 3. Count via GET /api/gallery/{name}/count
	countReq := httptest.NewRequest("GET", "/api/gallery/TestGallery/count", nil)
	countRec := httptest.NewRecorder()
	mux.ServeHTTP(countRec, countReq)

	if countRec.Code != http.StatusOK {
		t.Errorf("GET count failed with status %d", countRec.Code)
	}

	// 4. Delete gallery via DELETE /api/gallery/{name}
	delReq := httptest.NewRequest("DELETE", "/api/gallery/TestGallery", nil)
	delRec := httptest.NewRecorder()
	mux.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Errorf("DELETE gallery failed with status %d", delRec.Code)
	}

	_, err = sDB.GetGalleryByName("TestGallery")
	if err == nil {
		t.Errorf("Expected gallery to be deleted from DB")
	}
}

func TestSettingsAPI(t *testing.T) {
	_, mux, _, _ := setupTestServer(t)

	// 1. GET /api/settings
	req := httptest.NewRequest("GET", "/api/settings", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings failed with %d", rec.Code)
	}

	var settings map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
		t.Fatalf("Failed to parse settings JSON: %v", err)
	}
	if settings["default_sort_by"] != "name" {
		t.Errorf("Expected default_sort_by 'name', got %v", settings["default_sort_by"])
	}
}
