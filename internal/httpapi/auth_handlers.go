package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Taier05/InfraView/internal/service"
)

const maxLoginBodyBytes = 4 << 10

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionView struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !a.limiter.Allow(ip) {
		w.Header().Set("Retry-After", "60")
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", "登录尝试过于频繁，请稍后重试", true)
		return
	}

	var submitted loginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&submitted); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "登录请求格式无效", false)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "登录请求格式无效", false)
		return
	}

	session, ok := a.auth.Login(submitted.Username, submitted.Password)
	if !ok {
		a.limiter.RecordFailure(ip)
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误", false)
		return
	}
	a.limiter.Reset(ip)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/api/v1",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   a.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) session(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, r, sessionView{Authenticated: true, Username: a.config.Username}, service.Meta{})
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookieName)
	a.auth.Logout(cookie.Value)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/api/v1",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
