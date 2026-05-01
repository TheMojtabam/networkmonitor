// Package auth implements a minimal JWT-based authentication layer
// with a single admin user defined in config. Roles can be added later
// — for now everyone authenticated is "admin".
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/mojtaba/portsleuth/backend/internal/config"
)

type contextKey string

const userKey contextKey = "user"

// User is what we attach to a request after token validation.
type User struct {
	Username string
	Role     string
}

// Manager handles credential check and token issuing.
type Manager struct {
	cfg          config.AuthConfig
	hashedPasswd []byte
}

// New returns a manager. The admin password from cfg is bcrypted at startup.
func New(cfg config.AuthConfig) (*Manager, error) {
	if !cfg.Enabled {
		return &Manager{cfg: cfg}, nil
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("auth.jwt_secret must be set when auth is enabled")
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, errors.New("auth.jwt_secret must be at least 16 characters")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return &Manager{cfg: cfg, hashedPasswd: hashed}, nil
}

// Enabled returns true if auth is on.
func (m *Manager) Enabled() bool { return m.cfg.Enabled }

// Login verifies credentials and returns a signed JWT.
func (m *Manager) Login(username, password string) (string, error) {
	if !m.cfg.Enabled {
		return "", errors.New("auth disabled")
	}
	if username != m.cfg.AdminUsername {
		return "", errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword(m.hashedPasswd, []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	return m.issueToken(User{Username: username, Role: "admin"})
}

func (m *Manager) issueToken(u User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  u.Username,
		"role": u.Role,
		"exp":  time.Now().Add(m.cfg.SessionDuration()).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.cfg.JWTSecret))
}

// Verify parses and validates a token. Returns the user on success.
func (m *Manager) Verify(tokenStr string) (*User, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(m.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	sub, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	return &User{Username: sub, Role: role}, nil
}

// Middleware enforces auth on protected routes. When auth is disabled,
// it's a passthrough.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		// Allow login + health unauthenticated.
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		token := extractToken(r)
		if token == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		user, err := m.Verify(token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserFromCtx returns the authenticated user, or nil if anonymous.
func UserFromCtx(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

func extractToken(r *http.Request) string {
	// Authorization: Bearer <token>
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// ?token=... (for WebSocket where headers are awkward)
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}
