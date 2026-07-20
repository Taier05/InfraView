package auth

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManagerLoginValidatesCredentialsAndCreatesURLSafeSession(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	manager := NewManager("admin", "correct-password", 12*time.Hour, nil, func() time.Time { return now })

	if _, ok := manager.Login("admin", "wrong-password"); ok {
		t.Fatal("invalid password was accepted")
	}
	if _, ok := manager.Login("wrong-user", "correct-password"); ok {
		t.Fatal("invalid username was accepted")
	}

	first, ok := manager.Login("admin", "correct-password")
	if !ok {
		t.Fatal("valid credentials were rejected")
	}
	second, ok := manager.Login("admin", "correct-password")
	if !ok {
		t.Fatal("second valid login was rejected")
	}
	if first.Token == second.Token {
		t.Fatal("session tokens must be unique")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first.Token)
	if err != nil {
		t.Fatalf("token is not raw URL-safe base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded token length = %d, want 32", len(decoded))
	}
	if strings.ContainsAny(first.Token, "+/=") {
		t.Fatalf("token is not URL safe: %q", first.Token)
	}
	if !first.ExpiresAt.Equal(now.Add(12 * time.Hour)) {
		t.Fatalf("expiry = %s, want %s", first.ExpiresAt, now.Add(12*time.Hour))
	}
	if !manager.Validate(first.Token) {
		t.Fatal("new session is not valid")
	}
}

func TestManagerExpiresAndLogsOutSessions(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	manager := NewManager("admin", "correct-password", 12*time.Hour, nil, func() time.Time { return now })

	loggedOut, ok := manager.Login("admin", "correct-password")
	if !ok {
		t.Fatal("login failed")
	}
	manager.Logout(loggedOut.Token)
	if manager.Validate(loggedOut.Token) {
		t.Fatal("logged out session is still valid")
	}

	expiring, ok := manager.Login("admin", "correct-password")
	if !ok {
		t.Fatal("second login failed")
	}
	now = now.Add(12 * time.Hour)
	if manager.Validate(expiring.Token) {
		t.Fatal("expired session is still valid")
	}
}

func TestManagerSupportsConcurrentValidation(t *testing.T) {
	manager := NewManager("admin", "correct-password", 12*time.Hour, nil, time.Now)
	session, ok := manager.Login("admin", "correct-password")
	if !ok {
		t.Fatal("login failed")
	}

	const validators = 64
	var wait sync.WaitGroup
	wait.Add(validators)
	for range validators {
		go func() {
			defer wait.Done()
			for range 100 {
				if !manager.Validate(session.Token) {
					t.Error("session became invalid during concurrent validation")
					return
				}
			}
		}()
	}
	wait.Wait()
}
