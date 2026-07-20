package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Taier05/InfraView/internal/auth"
	"github.com/Taier05/InfraView/internal/config"
	"github.com/Taier05/InfraView/internal/service"
)

const sessionCookieName = "infraview_session"

type Dependencies struct {
	Config  config.Config
	Auth    *auth.Manager
	Limiter *auth.Limiter
	Service *service.Service
	Logger  *slog.Logger
}

type api struct {
	config      config.Config
	auth        *auth.Manager
	limiter     *auth.Limiter
	service     *service.Service
	logger      *slog.Logger
	verifyLogin func(string, string) (auth.Session, bool)
}

func New(dependencies Dependencies) http.Handler {
	if dependencies.Auth == nil {
		dependencies.Auth = auth.NewManager(
			dependencies.Config.Username,
			dependencies.Config.Password,
			dependencies.Config.SessionTTL,
			nil,
			time.Now,
		)
	}
	if dependencies.Limiter == nil {
		dependencies.Limiter = auth.NewLimiter(5, time.Minute, time.Now)
	}
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	server := &api{
		config:      dependencies.Config,
		auth:        dependencies.Auth,
		limiter:     dependencies.Limiter,
		service:     dependencies.Service,
		logger:      dependencies.Logger,
		verifyLogin: dependencies.Auth.Login,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.healthz)
	mux.Handle("POST /api/v1/session", server.sameOrigin(http.HandlerFunc(server.login)))
	mux.Handle("GET /api/v1/session", server.requireAuthentication(http.HandlerFunc(server.session)))
	mux.Handle("DELETE /api/v1/session", server.sameOrigin(server.requireAuthentication(http.HandlerFunc(server.logout))))
	mux.Handle("GET /api/v1/overview", server.requireAuthentication(http.HandlerFunc(server.overview)))
	mux.Handle("GET /api/v1/hosts", server.requireAuthentication(http.HandlerFunc(server.hosts)))
	mux.Handle("GET /api/v1/hosts/{id}", server.requireAuthentication(http.HandlerFunc(server.host)))
	mux.Handle("GET /api/v1/hosts/{id}/metrics", server.requireAuthentication(http.HandlerFunc(server.metrics)))
	mux.Handle("GET /api/v1/datasource/status", server.requireAuthentication(http.HandlerFunc(server.datasourceStatus)))

	mux.HandleFunc("/api/v1/session", server.methodNotAllowed)
	mux.HandleFunc("/api/v1/overview", server.methodNotAllowed)
	mux.HandleFunc("/api/v1/hosts", server.methodNotAllowed)
	mux.HandleFunc("/api/v1/hosts/", server.hostsFallback)
	mux.HandleFunc("/api/v1/datasource/status", server.methodNotAllowed)
	mux.HandleFunc("/api/", server.notFound)
	mux.HandleFunc("/", server.notFound)

	return server.middleware(mux)
}

func (a *api) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", allowedMethods(r.URL.Path))
	writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "不支持此请求方法", false)
}

func (a *api) hostsFallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, r)
		return
	}
	a.notFound(w, r)
}

func (a *api) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotFound, "not_found", "请求的接口不存在", false)
}

func allowedMethods(path string) string {
	switch {
	case path == "/api/v1/session":
		return "GET, POST, DELETE"
	case strings.HasSuffix(path, "/metrics"):
		return "GET"
	default:
		return "GET"
	}
}
