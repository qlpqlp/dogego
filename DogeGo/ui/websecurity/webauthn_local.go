// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package websecurity

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	waSessionTTL     = 5 * time.Minute
	waLocalUserName  = "dogego-owner"
	waLocalDisplay   = "DogeGo wallet"
)

type waPending struct {
	data    webauthn.SessionData
	expires time.Time
}

type localWAUser struct {
	id          []byte
	credentials []webauthn.Credential
}

func (u *localWAUser) WebAuthnID() []byte                         { return u.id }
func (u *localWAUser) WebAuthnName() string                       { return waLocalUserName }
func (u *localWAUser) WebAuthnDisplayName() string                { return waLocalDisplay }
func (u *localWAUser) WebAuthnIcon() string                       { return "" }
func (u *localWAUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func (g *Gate) waUserLocked() *localWAUser {
	if len(g.data.WebAuthnUserID) == 0 {
		var id [16]byte
		_, _ = rand.Read(id[:])
		g.data.WebAuthnUserID = id[:]
	}
	return &localWAUser{
		id:          append([]byte(nil), g.data.WebAuthnUserID...),
		credentials: append([]webauthn.Credential(nil), g.data.WebAuthnCredentials...),
	}
}

func webAuthnFromRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	rpID, origins := rpFromRequest(r)
	if rpID == "" {
		return nil, errors.New("invalid host for WebAuthn")
	}
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "DogeGo",
		RPID:          rpID,
		RPOrigins:     origins,
	})
}

func rpFromRequest(r *http.Request) (rpID string, origins []string) {
	if r == nil {
		return "", nil
	}
	host := strings.TrimSpace(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" {
		return "", nil
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	origin := scheme + "://" + r.Host
	return host, []string{origin}
}

func (g *Gate) pruneWaSessionsLocked(now time.Time) {
	for id, p := range g.waPending {
		if now.After(p.expires) {
			delete(g.waPending, id)
		}
	}
}

func (g *Gate) putWaSession(data webauthn.SessionData) (string, error) {
	var b [18]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(b[:])
	if g.waPending == nil {
		g.waPending = make(map[string]waPending)
	}
	now := time.Now()
	g.pruneWaSessionsLocked(now)
	g.waPending[id] = waPending{data: data, expires: now.Add(waSessionTTL)}
	return id, nil
}

func (g *Gate) takeWaSession(id string) (webauthn.SessionData, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneWaSessionsLocked(time.Now())
	p, ok := g.waPending[id]
	if !ok {
		return webauthn.SessionData{}, false
	}
	delete(g.waPending, id)
	return p.data, true
}

// BeginWebAuthnRegister starts platform credential registration (PIN session must be unlocked).
func (g *Gate) BeginWebAuthnRegister(r *http.Request) (sessionID string, publicKey any, err error) {
	if !g.Enabled() {
		return "", nil, errors.New("configure PIN before biometrics")
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		return "", nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	user := g.waUserLocked()
	creation, session, err := wa.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.Platform,
			RequireResidentKey:      protocol.ResidentKeyNotRequired(),
			UserVerification:        protocol.VerificationRequired,
		}),
	)
	if err != nil {
		return "", nil, err
	}
	sessionID, err = g.putWaSession(*session)
	if err != nil {
		return "", nil, err
	}
	return sessionID, creation.Response, nil
}

// FinishWebAuthnRegister verifies the authenticator response and stores the credential.
func (g *Gate) FinishWebAuthnRegister(r *http.Request, sessionID string, credentialJSON json.RawMessage) error {
	if !g.Enabled() {
		return errors.New("PIN not configured")
	}
	session, ok := g.takeWaSession(sessionID)
	if !ok {
		return errors.New("registration session expired; try again")
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(credentialJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	g.mu.Lock()
	defer g.mu.Unlock()
	user := g.waUserLocked()
	cred, err := wa.FinishRegistration(user, session, req)
	if err != nil {
		return err
	}
	user.credentials = append(user.credentials, *cred)
	g.data.WebAuthnCredentials = user.credentials
	g.data.WebAuthnUserID = user.id
	g.data.WebAuthnEnabled = true
	return g.saveLocked()
}

// BeginWebAuthnLogin starts a biometric unlock challenge.
func (g *Gate) BeginWebAuthnLogin(r *http.Request) (sessionID string, publicKey any, err error) {
	if !g.Enabled() {
		return "", nil, errors.New("PIN not configured")
	}
	g.mu.Lock()
	hasCred := g.data.WebAuthnEnabled && len(g.data.WebAuthnCredentials) > 0
	g.mu.Unlock()
	if !hasCred {
		return "", nil, errors.New("biometrics not registered")
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		return "", nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	user := g.waUserLocked()
	assertion, session, err := wa.BeginLogin(user, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return "", nil, err
	}
	sessionID, err = g.putWaSession(*session)
	if err != nil {
		return "", nil, err
	}
	return sessionID, assertion.Response, nil
}

// FinishWebAuthnLogin verifies the assertion and returns a dashboard session token.
func (g *Gate) FinishWebAuthnLogin(r *http.Request, sessionID string, credentialJSON json.RawMessage) (string, error) {
	session, ok := g.takeWaSession(sessionID)
	if !ok {
		return "", errors.New("login session expired; try again")
	}
	wa, err := webAuthnFromRequest(r)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(credentialJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.pinEnabledLocked() || !g.data.WebAuthnEnabled || len(g.data.WebAuthnCredentials) == 0 {
		return "", errors.New("biometrics not registered")
	}
	now := time.Now().Unix()
	if g.data.LockedUntil > now {
		return "", fmt.Errorf("PIN locked; try again in %d seconds", g.data.LockedUntil-now)
	}
	user := g.waUserLocked()
	if _, err := wa.FinishLogin(user, session, req); err != nil {
		return "", err
	}
	g.data.FailedAttempts = 0
	g.data.LockedUntil = 0
	_ = g.saveLocked()
	return g.newSessionLocked(), nil
}

// ClearWebAuthnCredentials removes registered platform credentials (requires unlocked PIN session).
func (g *Gate) ClearWebAuthnCredentials() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.data.WebAuthnCredentials = nil
	g.data.WebAuthnEnabled = false
	return g.saveLocked()
}
