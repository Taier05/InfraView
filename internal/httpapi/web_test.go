package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Taier05/InfraView/internal/adapters/mock"
	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/cache"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/service"
)

func TestSPAServesIndexForRootAndClientRoutes(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	for _, path := range []string{"/", "/hosts", "/hosts/mock-host-001"} {
		t.Run(path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, path, "", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
				t.Fatalf("Content-Type = %q", contentType)
			}
			if response.Header().Get("Cache-Control") != "no-cache" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
			if !strings.Contains(response.Body.String(), `<div id="root"></div>`) {
				t.Fatalf("response is not the SPA index: %s", response.Body.String())
			}
			assertSecurityHeaders(t, response)
		})
	}
}

func TestSPAServesFingerprintedAssetsWithImmutableCache(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	index := request(t, handler, http.MethodGet, "/", "", nil)
	assetPattern := regexp.MustCompile(`/assets/[^"']+\.(?:js|css)`)
	assetPath := assetPattern.FindString(index.Body.String())
	if assetPath == "" {
		t.Fatalf("fingerprinted asset not found in index: %s", index.Body.String())
	}

	asset := request(t, handler, http.MethodGet, assetPath, "", nil)
	if asset.Code != http.StatusOK || asset.Body.Len() == 0 {
		t.Fatalf("asset response = %d %q", asset.Code, asset.Body.String())
	}
	if got, want := asset.Header().Get("Cache-Control"), "public, max-age=31536000, immutable"; got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
	if strings.HasSuffix(assetPath, ".js") && !strings.HasPrefix(asset.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("Content-Type = %q", asset.Header().Get("Content-Type"))
	}
	assertSecurityHeaders(t, asset)
}

func TestUnknownAPIRouteNeverFallsBackToSPA(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	response := request(t, handler, http.MethodGet, "/api/v1/not-real", "", nil)
	assertError(t, response, http.StatusNotFound, "not_found", "请求的接口不存在")
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q", got)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

func TestSPARejectsDotfilesAndPathTraversal(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	for _, path := range []string{
		"/.env",
		"/.git/config",
		"/assets/.secret",
		"/%2e%2e/.env",
		"/assets/%2e%2e/index.html",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusNotFound && response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), `<div id="root"></div>`) || strings.Contains(response.Body.String(), "INFRAVIEW_PASSWORD") {
				t.Fatalf("sensitive path received web content: %s", response.Body.String())
			}
		})
	}
}

func TestMissingStaticAssetsNeverFallBackToSPA(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	for _, path := range []string{"/assets", "/assets/not-real.js", "/favicon.ico"} {
		t.Run(path, func(t *testing.T) {
			response := request(t, handler, http.MethodGet, path, "", nil)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), `<div id="root"></div>`) {
				t.Fatalf("missing static asset fell back to SPA: %s", response.Body.String())
			}
		})
	}
}

func TestReadOnlyRouteSurfaceRejectsOperationalActions(t *testing.T) {
	handler := newTestAPI(t, mock.New(3, testNow))
	cookie := loginCookie(t, handler)
	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/command"},
		{method: http.MethodPost, path: "/api/v1/commands"},
		{method: http.MethodPost, path: "/api/v1/hosts/mock-host-001/restart"},
		{method: http.MethodDelete, path: "/api/v1/hosts/mock-host-001"},
		{method: http.MethodPatch, path: "/api/v1/hosts/mock-host-001"},
		{method: http.MethodGet, path: "/api/v1/proxy"},
		{method: http.MethodPost, path: "/api/v1/query"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := request(t, handler, test.method, test.path, `{}`, cookie)
			if response.Code != http.StatusNotFound && response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Fatalf("Content-Type = %q, body = %s", got, response.Body.String())
			}
			if strings.Contains(response.Body.String(), `<div id="root"></div>`) {
				t.Fatalf("operational API route fell back to SPA: %s", response.Body.String())
			}
		})
	}
}

func TestRequestLogsDoNotLeakCredentialsCookiesOrTokens(t *testing.T) {
	const (
		password = "known-password-value"
		cookie   = "known-cookie-value"
		token    = "known-token-value"
	)
	var logs bytes.Buffer
	cfg := config.Config{
		Username:          "admin",
		Password:          password,
		SessionTTL:        12 * time.Hour,
		DataSource:        "mock",
		InventoryTTL:      time.Minute,
		CurrentMetricsTTL: 20 * time.Second,
		RangeTTL:          time.Minute,
		HealthTTL:         15 * time.Second,
		MaxStale:          5 * time.Minute,
		WarningPercent:    80,
		CriticalPercent:   90,
	}
	handler := New(Dependencies{
		Config:  cfg,
		Auth:    auth.NewManager(cfg.Username, cfg.Password, cfg.SessionTTL, nil, testNow),
		Limiter: auth.NewLimiter(5, time.Minute, testNow),
		Service: service.New(mock.New(3, testNow), cache.New(testNow), service.Options{
			InventoryTTL:      cfg.InventoryTTL,
			CurrentMetricsTTL: cfg.CurrentMetricsTTL,
			RangeTTL:          cfg.RangeTTL,
			HealthTTL:         cfg.HealthTTL,
			MaxStale:          cfg.MaxStale,
			WarningPercent:    cfg.WarningPercent,
			CriticalPercent:   cfg.CriticalPercent,
			Clock:             testNow,
		}),
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	})

	login := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/session", strings.NewReader(`{"username":"admin","password":"`+password+`"}`))
	login.Header.Set("Cookie", sessionCookieName+"="+cookie)
	login.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(httptest.NewRecorder(), login)

	unknown := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/not-real?token="+token, nil)
	unknown.Header.Set("Cookie", sessionCookieName+"="+cookie)
	unknown.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(httptest.NewRecorder(), unknown)

	for name, secret := range map[string]string{"password": password, "cookie": cookie, "token": token} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("logs leak %s: %s", name, logs.String())
		}
	}
}
