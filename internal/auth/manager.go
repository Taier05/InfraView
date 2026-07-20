package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"io"
	"sync"
	"time"
)

const tokenBytes = 32

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type Manager struct {
	mu           sync.Mutex
	usernameHash [sha256.Size]byte
	passwordHash [sha256.Size]byte
	ttl          time.Duration
	random       io.Reader
	clock        func() time.Time
	sessions     map[[sha256.Size]byte]time.Time
}

func NewManager(username, password string, ttl time.Duration, random io.Reader, clock func() time.Time) *Manager {
	if random == nil {
		random = rand.Reader
	}
	if clock == nil {
		clock = time.Now
	}
	return &Manager{
		usernameHash: sha256.Sum256([]byte(username)),
		passwordHash: sha256.Sum256([]byte(password)),
		ttl:          ttl,
		random:       random,
		clock:        clock,
		sessions:     make(map[[sha256.Size]byte]time.Time),
	}
}

func (m *Manager) Login(username, password string) (Session, bool) {
	usernameHash := sha256.Sum256([]byte(username))
	passwordHash := sha256.Sum256([]byte(password))
	usernameOK := subtle.ConstantTimeCompare(usernameHash[:], m.usernameHash[:])
	passwordOK := subtle.ConstantTimeCompare(passwordHash[:], m.passwordHash[:])
	if usernameOK&passwordOK != 1 {
		return Session{}, false
	}

	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(m.random, raw); err != nil {
		return Session{}, false
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	tokenHash := sha256.Sum256([]byte(token))
	now := m.clock()
	expiresAt := now.Add(m.ttl)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpired(now)
	if _, exists := m.sessions[tokenHash]; exists {
		return Session{}, false
	}
	m.sessions[tokenHash] = expiresAt
	return Session{Token: token, ExpiresAt: expiresAt}, true
}

func (m *Manager) Validate(token string) bool {
	tokenHash := sha256.Sum256([]byte(token))
	now := m.clock()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpired(now)
	_, ok := m.sessions[tokenHash]
	return ok
}

func (m *Manager) Logout(token string) {
	tokenHash := sha256.Sum256([]byte(token))
	m.mu.Lock()
	delete(m.sessions, tokenHash)
	m.mu.Unlock()
}

func (m *Manager) pruneExpired(now time.Time) {
	for tokenHash, expiresAt := range m.sessions {
		if !now.Before(expiresAt) {
			delete(m.sessions, tokenHash)
		}
	}
}
