package httpapi

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

//go:embed webdist
var embeddedWeb embed.FS

const immutableCacheControl = "public, max-age=31536000, immutable"

var fingerprintedAssetPattern = regexp.MustCompile(`^assets/.+-[A-Za-z0-9_-]{8,}\.[^./]+$`)

func (a *api) web() http.Handler {
	webRoot, err := fs.Sub(embeddedWeb, "webdist")
	if err != nil {
		panic("嵌入 Web 静态资源失败")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			a.notFound(w, r)
			return
		}

		requestedPath, ok := safeWebPath(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}

		if requestedPath != "" {
			if info, statErr := fs.Stat(webRoot, requestedPath); statErr == nil && !info.IsDir() {
				serveWebFile(w, r, webRoot, requestedPath, webCacheControl(requestedPath))
				return
			}
		}

		if requestedPath == "assets" || strings.HasPrefix(requestedPath, "assets/") || path.Ext(requestedPath) != "" {
			http.NotFound(w, r)
			return
		}
		serveWebFile(w, r, webRoot, "index.html", "no-cache")
	})
}

func webCacheControl(requestedPath string) string {
	if fingerprintedAssetPattern.MatchString(requestedPath) {
		return immutableCacheControl
	}
	return "no-cache"
}

func safeWebPath(urlPath string) (string, bool) {
	if urlPath == "" || !strings.HasPrefix(urlPath, "/") {
		return "", false
	}
	requestedPath := strings.TrimPrefix(urlPath, "/")
	for _, segment := range strings.Split(requestedPath, "/") {
		if segment == "." || segment == ".." || strings.HasPrefix(segment, ".") {
			return "", false
		}
	}
	return requestedPath, true
}

func serveWebFile(w http.ResponseWriter, r *http.Request, webRoot fs.FS, name, cacheControl string) {
	content, err := fs.ReadFile(webRoot, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", cacheControl)
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(content))
}
