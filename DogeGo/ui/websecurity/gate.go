// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package websecurity gates sensitive Web UI actions (wallet send, balances)
// with an optional 6-digit PIN, rate limiting, and short-lived session tokens.
package websecurity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	maxPINFailures   = 3
	lockDuration     = time.Hour
	sessionTTL       = 30 * time.Minute
	bcryptCost       = 12
	securityFileName = "web_security.json"
	cookieName       = "dogego_ui_sess"
)

var pinDigits = regexp.MustCompile(`^\d{6}$`)

// Gate holds PIN state, active sessions, and persistence path.
type Gate struct {
	mu   sync.Mutex
	path string
	data securityFile
	// token -> expiry unix
	sessions map[string]time.Time
	waPending map[string]waPending
}

type securityFile struct {
	Version        int    `json:"version"`
	PINHash        string `json:"pin_hash,omitempty"`
	FailedAttempts int    `json:"failed_attempts,omitempty"`
	LockedUntil    int64  `json:"locked_until,omitempty"` // unix sec, 0 = not locked
	// WebAuthnEnabled is set when a platform credential is registered.
	WebAuthnEnabled bool `json:"webauthn_enabled,omitempty"`
	WebAuthnUserID  []byte                `json:"webauthn_user_id,omitempty"`
	WebAuthnCredentials []webauthn.Credential `json:"webauthn_credentials,omitempty"`
}

// NewGate loads or creates security state under chainDataDir.
func NewGate(chainDataDir string) (*Gate, error) {
	if chainDataDir == "" {
		return nil, errors.New("empty chain data dir")
	}
	g := &Gate{
		path:     filepath.Join(chainDataDir, securityFileName),
		sessions: make(map[string]time.Time),
	}
	if err := g.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return g, nil
}

func (g *Gate) load() error {
	b, err := os.ReadFile(g.path)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return json.Unmarshal(b, &g.data)
}

func (g *Gate) saveLocked() error {
	b, err := json.MarshalIndent(g.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(g.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := g.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, g.path)
}

func (g *Gate) pinEnabledLocked() bool {
	return strings.TrimSpace(g.data.PINHash) != ""
}

// Enabled reports whether a PIN is configured.
func (g *Gate) Enabled() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pinEnabledLocked()
}

// Status returns public lock/session info for the API.
func (g *Gate) Status(r *http.Request) map[string]interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().Unix()
	locked := g.data.LockedUntil > now
	rem := int64(0)
	if locked {
		rem = g.data.LockedUntil - now
	}
	tok := sessionTokenFromRequest(r)
	pinEnabled := g.pinEnabledLocked()
	unlocked := !pinEnabled || (tok != "" && g.sessionValidLocked(tok))
	return map[string]interface{}{
		"pin_enabled":      pinEnabled,
		"locked":           locked,
		"locked_seconds":   rem,
		"failed_attempts":  g.data.FailedAttempts,
		"max_failures":     maxPINFailures,
		"unlocked":         unlocked,
		"webauthn_enabled": g.data.WebAuthnEnabled,
	}
}

func (g *Gate) sessionValidLocked(tok string) bool {
	exp, ok := g.sessions[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(g.sessions, tok)
		return false
	}
	return true
}

// SetupPIN sets or replaces the PIN (requires current PIN when already enabled).
func (g *Gate) SetupPIN(currentPIN, newPIN string) error {
	newPIN = strings.TrimSpace(newPIN)
	if !pinDigits.MatchString(newPIN) {
		return fmt.Errorf("PIN must be exactly 6 digits")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().Unix()
	if g.data.LockedUntil > now {
		return fmt.Errorf("PIN locked; try again in %d seconds", g.data.LockedUntil-now)
	}
	if g.pinEnabledLocked() {
		currentPIN = strings.TrimSpace(currentPIN)
		if currentPIN == "" {
			return fmt.Errorf("current PIN required")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(g.data.PINHash), []byte(currentPIN)); err != nil {
			g.recordFailureLocked(now)
			if saveErr := g.saveLocked(); saveErr != nil {
				return saveErr
			}
			return fmt.Errorf("wrong current PIN")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPIN), bcryptCost)
	if err != nil {
		return err
	}
	g.data.PINHash = string(hash)
	g.data.FailedAttempts = 0
	g.data.LockedUntil = 0
	g.clearSessionsLocked()
	return g.saveLocked()
}

// UnlockPIN verifies PIN and returns a session token.
func (g *Gate) UnlockPIN(pin string) (token string, err error) {
	pin = strings.TrimSpace(pin)
	if !pinDigits.MatchString(pin) {
		return "", fmt.Errorf("PIN must be exactly 6 digits")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().Unix()
	if g.data.LockedUntil > now {
		return "", fmt.Errorf("too many attempts; locked for %d seconds", g.data.LockedUntil-now)
	}
	if !g.pinEnabledLocked() {
		return "", fmt.Errorf("PIN not configured")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(g.data.PINHash), []byte(pin)); err != nil {
		g.recordFailureLocked(now)
		_ = g.saveLocked()
		left := maxPINFailures - g.data.FailedAttempts
		if left < 0 {
			left = 0
		}
		if g.data.LockedUntil > now {
			return "", fmt.Errorf("too many attempts; locked for 1 hour")
		}
		return "", fmt.Errorf("wrong PIN (%d attempts left)", left)
	}
	g.data.FailedAttempts = 0
	g.data.LockedUntil = 0
	_ = g.saveLocked()
	return g.newSessionLocked(), nil
}

func (g *Gate) recordFailureLocked(now int64) {
	g.data.FailedAttempts++
	if g.data.FailedAttempts >= maxPINFailures {
		g.data.LockedUntil = now + int64(lockDuration.Seconds())
		g.data.FailedAttempts = 0
	}
}

func (g *Gate) newSessionLocked() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	tok := base64.RawURLEncoding.EncodeToString(b[:])
	g.sessions[tok] = time.Now().Add(sessionTTL)
	return tok
}

func (g *Gate) clearSessionsLocked() {
	g.sessions = make(map[string]time.Time)
}

// Lock clears server sessions (PIN remains configured).
func (g *Gate) Lock() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clearSessionsLocked()
}

// DisablePIN removes PIN protection (requires correct PIN).
func (g *Gate) DisablePIN(pin string) error {
	pin = strings.TrimSpace(pin)
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.pinEnabledLocked() {
		return nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(g.data.PINHash), []byte(pin)); err != nil {
		return fmt.Errorf("wrong PIN")
	}
	g.data.PINHash = ""
	g.data.FailedAttempts = 0
	g.data.LockedUntil = 0
	g.data.WebAuthnEnabled = false
	g.data.WebAuthnCredentials = nil
	g.data.WebAuthnUserID = nil
	g.clearSessionsLocked()
	return g.saveLocked()
}

// RequireUnlocked returns false and writes 401 JSON if PIN is enabled but session invalid.
func (g *Gate) RequireUnlocked(w http.ResponseWriter, r *http.Request) bool {
	if !g.Enabled() {
		return true
	}
	tok := sessionTokenFromRequest(r)
	g.mu.Lock()
	ok := g.sessionValidLocked(tok)
	g.mu.Unlock()
	if ok {
		return true
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":      "wallet_locked",
		"pin_required": true,
	})
	return false
}

// SetSessionCookie attaches the session token to the response.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	c := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	}
	if secure {
		c.Secure = true
	}
	http.SetCookie(w, c)
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

// ConstantTimeTokenCompare compares session tokens safely.
func ConstantTimeTokenCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
