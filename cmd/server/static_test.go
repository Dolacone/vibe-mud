package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testFrontendFiles(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"index.html":                     "<!doctype html><title>Vibe MUD</title>",
		"manifest.webmanifest":           `{"name":"Vibe MUD","start_url":"/","scope":"/","display":"standalone"}`,
		"assets/app-Abcdef12.js":         "console.log('versioned')",
		"assets/plain.css":               "body{}",
		"assets/short-1234567.js":        "console.log('short')",
		"assets/underscored-ABC_def-.js": "console.log('underscored')",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestStaticHandlerServesManifestWithManifestContentType(t *testing.T) {
	root := testFrontendFiles(t)
	handler, err := newStaticHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	response := requestStatic(t, handler, http.MethodGet, "/manifest.webmanifest")
	if response.Code != http.StatusOK {
		t.Fatalf("manifest status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "application/manifest+json" {
		t.Fatalf("manifest content type = %q", got)
	}
	if !strings.Contains(response.Body.String(), `"display":"standalone"`) {
		t.Fatalf("manifest body = %q", response.Body.String())
	}
}

func requestStatic(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
	return response
}

func TestStaticHandlerServesEntryClientRoutesAndOnlyVersionedAssetsImmutable(t *testing.T) {
	root := testFrontendFiles(t)
	handler, err := newStaticHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path      string
		wantBody  string
		wantCache string
	}{
		{"/", "<!doctype html><title>Vibe MUD</title>", frontendEntryCacheControl},
		{"/play/room", "<!doctype html><title>Vibe MUD</title>", frontendEntryCacheControl},
		{"/index.html", "<!doctype html><title>Vibe MUD</title>", frontendEntryCacheControl},
		{"/assets/app-Abcdef12.js", "console.log('versioned')", frontendAssetCacheControl},
		{"/assets/underscored-ABC_def-.js", "console.log('underscored')", frontendAssetCacheControl},
		{"/assets/plain.css", "body{}", frontendEntryCacheControl},
		{"/assets/short-1234567.js", "console.log('short')", frontendEntryCacheControl},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			response := requestStatic(t, handler, http.MethodGet, test.path)
			if response.Code != http.StatusOK || response.Body.String() != test.wantBody {
				t.Fatalf("status/body = %d/%q", response.Code, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != test.wantCache {
				t.Fatalf("cache control = %q, want %q", response.Header().Get("Cache-Control"), test.wantCache)
			}
		})
	}
}

func TestStaticHandlerRevalidatesUnchangedFiles(t *testing.T) {
	root := testFrontendFiles(t)
	handler, err := newStaticHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	first := requestStatic(t, handler, http.MethodGet, "/assets/app-Abcdef12.js")
	lastModified := first.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Fatal("static asset did not include Last-Modified")
	}
	request := httptest.NewRequest(http.MethodGet, "/assets/app-Abcdef12.js", nil)
	request.Header.Set("If-Modified-Since", lastModified)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatalf("conditional response = %d/%q, want 304 with no body", second.Code, second.Body.String())
	}
	if second.Header().Get("Cache-Control") != frontendAssetCacheControl {
		t.Fatalf("conditional cache control = %q", second.Header().Get("Cache-Control"))
	}
}

func TestStaticHandlerNeverServesFrontendForReservedPaths(t *testing.T) {
	root := testFrontendFiles(t)
	handler, err := newStaticHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/me", "/api/client.js", "/auth/google/login", "/auth/client.js"} {
		response := requestStatic(t, handler, http.MethodGet, path)
		body, _ := io.ReadAll(response.Result().Body)
		if response.Code != http.StatusNotFound || strings.Contains(string(body), "Vibe MUD") {
			t.Fatalf("reserved path %q returned frontend: status=%d body=%q", path, response.Code, body)
		}
	}
}

func TestStaticHandlerRejectsMissingAssetAndUnsupportedMethod(t *testing.T) {
	root := testFrontendFiles(t)
	handler, err := newStaticHandler(root)
	if err != nil {
		t.Fatal(err)
	}
	missing := requestStatic(t, handler, http.MethodGet, "/assets/missing.js")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d", missing.Code)
	}
	post := requestStatic(t, handler, http.MethodPost, "/play")
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", post.Code)
	}
}

func TestNewStaticHandlerRequiresPrebuiltEntryDocument(t *testing.T) {
	if _, err := newStaticHandler(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing frontend directory was accepted")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "index.html"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := newStaticHandler(root); err == nil {
		t.Fatal("directory entry document was accepted")
	}
}
