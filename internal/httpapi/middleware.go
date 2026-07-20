package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type contextKey string

const requestStateKey contextKey = "request-state"

type requestState struct {
	id    string
	stale bool
}

func (a *api) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &requestState{id: newRequestID()}
		w.Header().Set("X-Request-ID", state.id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestStateKey, state)))
	})
}

func (a *api) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *api) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("HTTP 请求处理发生异常", "request_id", requestIDFrom(r.Context()), "path", r.URL.Path)
				writeError(w, r, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求", true)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *api) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			state := requestStateFrom(r.Context())
			a.logger.Info("HTTP 请求完成",
				"request_id", requestIDFrom(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"stale", state != nil && state.stale,
				"data_source", a.config.DataSource,
			)
		}()
		next.ServeHTTP(recorder, r)
	})
}

func (a *api) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !a.auth.Validate(cookie.Value) {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "请先登录", false)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *api) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isSameOrigin(r) {
			writeError(w, r, http.StatusForbidden, "cross_origin_forbidden", "不允许跨域请求", false)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(parsed.Host, r.Host)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func requestIDFrom(ctx context.Context) string {
	if state := requestStateFrom(ctx); state != nil {
		return state.id
	}
	return ""
}

func requestStateFrom(ctx context.Context) *requestState {
	state, _ := ctx.Value(requestStateKey).(*requestState)
	return state
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}
