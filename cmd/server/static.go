package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	frontendEntryCacheControl = "no-cache"
	frontendAssetCacheControl = "public, max-age=31536000, immutable"
)

var versionedAssetName = regexp.MustCompile(`^[A-Za-z0-9._-]+-[A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$`)

type staticHandler struct {
	root string
}

func newStaticHandler(root string) (http.Handler, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat frontend directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("frontend path is not a directory")
	}
	index := filepath.Join(root, "index.html")
	indexInfo, err := os.Stat(index)
	if err != nil {
		return nil, fmt.Errorf("stat frontend entry document: %w", err)
	}
	if indexInfo.IsDir() {
		return nil, errors.New("frontend entry document is a directory")
	}
	return &staticHandler{root: root}, nil
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if isReservedFrontendPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}

	requestPath := path.Clean("/" + r.URL.Path)
	if requestPath == "/" {
		h.serve(w, r, "/index.html", false)
		return
	}
	relative := strings.TrimPrefix(requestPath, "/")
	localPath := filepath.Join(h.root, filepath.FromSlash(relative))
	info, err := os.Stat(localPath)
	if err == nil && !info.IsDir() {
		h.serve(w, r, requestPath, path.Base(relative) == "index.html")
		return
	}
	if path.Ext(path.Base(relative)) == "" {
		h.serve(w, r, "/index.html", false)
		return
	}
	http.NotFound(w, r)
}

func (h *staticHandler) serve(w http.ResponseWriter, r *http.Request, requestPath string, entry bool) {
	cacheControl := frontendEntryCacheControl
	if !entry && requestPath != "/index.html" && versionedAssetName.MatchString(path.Base(requestPath)) {
		cacheControl = frontendAssetCacheControl
	}
	w.Header().Set("Cache-Control", cacheControl)
	filePath := filepath.Join(h.root, filepath.FromSlash(strings.TrimPrefix(requestPath, "/")))
	file, err := os.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func isReservedFrontendPath(requestPath string) bool {
	return requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") || requestPath == "/auth" || strings.HasPrefix(requestPath, "/auth/")
}
