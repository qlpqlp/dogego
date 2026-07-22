// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"strings"
)

const (
	defaultDGRListen            = ":24433"
	defaultDGRRelayPort         = 24433
	defaultDGRMaxClients        = 256
	defaultDGRMaxRelayConn      = 3
	defaultDGRSessionFramesPerS = 60
	defaultDGRP2PProxyPerS      = 20
	defaultDGRRegisterPerMin    = 10
)

// DogeGoRelayCGNAT configures integrated QUIC reachability relay (NODE_DOGEGO_RELAY_CGNAT).
// Public rdogego nodes set inbound_relay; CGNAT clients set outbound_relay (auto on for cgnat mode when enabled).
type DogeGoRelayCGNAT struct {
	// Enabled is the master switch for the DGR subsystem (restart required).
	Enabled bool `json:"enabled,omitempty"`
	// InboundRelay listens for QUIC registrations from CGNAT clients (rdogego operator).
	InboundRelay bool `json:"inbound_relay,omitempty"`
	// OutboundRelay dials public relays when this node cannot accept inbound P2P (CGNAT client).
	OutboundRelay bool `json:"outbound_relay,omitempty"`
	// Listen is the UDP QUIC bind address for inbound relay (default :24433).
	Listen string `json:"listen,omitempty"`
	// RelayPort is the QUIC port assumed on peers advertising NODE_DOGEGO_RELAY_CGNAT (default 24433).
	RelayPort int `json:"relay_port,omitempty"`
	// RelaySeeds are static host:port QUIC targets (merged with P2P-learned relays).
	RelaySeeds []string `json:"relay_seeds,omitempty"`
	// RelayDNSSeed is an optional DNS hostname; TXT records list host:port lines.
	RelayDNSSeed string `json:"relay_dnsseed,omitempty"`
	// AuthToken is an optional shared secret for REGISTER (empty = open relay).
	AuthToken string `json:"auth_token,omitempty"`
	// MaxClients caps simultaneous inbound relay sessions (default 256).
	MaxClients int `json:"max_clients,omitempty"`
	// MaxRelayConns caps outbound relay QUIC sessions (default 3).
	MaxRelayConns int `json:"max_relay_conns,omitempty"`
	// AllowClients restricts inbound REGISTER sources (CIDR or IP; empty = allow all).
	AllowClients []string `json:"allow_clients,omitempty"`
	// RelayTLSPins are SHA-256 hex fingerprints of relay TLS certificates (leaf DER).
	// When set on outbound clients, dials reject relays whose cert does not match a pin.
	RelayTLSPins []string `json:"relay_tls_pins,omitempty"`
	// MaxSessionFramesPerSec caps inbound DGR frames per registered client session (default 60).
	MaxSessionFramesPerSec int `json:"max_session_frames_per_sec,omitempty"`
	// MaxP2PProxyPerSec caps inbound P2P_FRAME proxy requests per client session (default 20).
	MaxP2PProxyPerSec int `json:"max_p2p_proxy_per_sec,omitempty"`
	// MaxRegisterPerMin caps REGISTER attempts per client IP per minute on inbound relay (default 10).
	MaxRegisterPerMin int `json:"max_register_per_min,omitempty"`
}

// EffectiveListen returns the QUIC listen address.
func (c DogeGoRelayCGNAT) EffectiveListen() string {
	if s := strings.TrimSpace(c.Listen); s != "" {
		return s
	}
	return defaultDGRListen
}

// EffectiveRelayPort returns the QUIC port for relay discovery.
func (c DogeGoRelayCGNAT) EffectiveRelayPort() int {
	if c.RelayPort > 0 && c.RelayPort <= 65535 {
		return c.RelayPort
	}
	return defaultDGRRelayPort
}

// EffectiveMaxClients returns inbound session cap.
func (c DogeGoRelayCGNAT) EffectiveMaxClients() int {
	if c.MaxClients <= 0 {
		return defaultDGRMaxClients
	}
	if c.MaxClients > 4096 {
		return 4096
	}
	return c.MaxClients
}

// EffectiveMaxRelayConns returns outbound relay session cap.
func (c DogeGoRelayCGNAT) EffectiveMaxRelayConns() int {
	if c.MaxRelayConns <= 0 {
		return defaultDGRMaxRelayConn
	}
	if c.MaxRelayConns > 16 {
		return 16
	}
	return c.MaxRelayConns
}

// EffectiveMaxSessionFramesPerSec returns inbound frame rate limit per relay session.
func (c DogeGoRelayCGNAT) EffectiveMaxSessionFramesPerSec() float64 {
	if c.MaxSessionFramesPerSec <= 0 {
		return defaultDGRSessionFramesPerS
	}
	if c.MaxSessionFramesPerSec > 1000 {
		return 1000
	}
	return float64(c.MaxSessionFramesPerSec)
}

// EffectiveMaxP2PProxyPerSec returns inbound P2P proxy rate limit per relay session.
func (c DogeGoRelayCGNAT) EffectiveMaxP2PProxyPerSec() float64 {
	if c.MaxP2PProxyPerSec <= 0 {
		return defaultDGRP2PProxyPerS
	}
	if c.MaxP2PProxyPerSec > 500 {
		return 500
	}
	return float64(c.MaxP2PProxyPerSec)
}

// EffectiveMaxRegisterPerMin returns inbound REGISTER attempts allowed per client IP per minute.
func (c DogeGoRelayCGNAT) EffectiveMaxRegisterPerMin() int {
	if c.MaxRegisterPerMin <= 0 {
		return defaultDGRRegisterPerMin
	}
	if c.MaxRegisterPerMin > 120 {
		return 120
	}
	return c.MaxRegisterPerMin
}

// RoleInbound reports whether this node should run the public relay listener.
func (c DogeGoRelayCGNAT) RoleInbound() bool {
	return c.Enabled && c.InboundRelay
}

// RoleOutbound reports whether this node should dial relays as a CGNAT client.
func (c DogeGoRelayCGNAT) RoleOutbound(p2pMode string) bool {
	if !c.Enabled {
		return false
	}
	if c.OutboundRelay {
		return true
	}
	mode := strings.ToLower(strings.TrimSpace(p2pMode))
	return mode == "cgnat" || mode == "both"
}

// AdvertiseServiceBit reports NODE_DOGEGO_RELAY_CGNAT on P2P version/addr.
func (c DogeGoRelayCGNAT) AdvertiseServiceBit() bool {
	return c.RoleInbound()
}

// UserConfigured reports whether dogego_relay_cgnat was explicitly set in config (skip wizard auto-config).
func (c DogeGoRelayCGNAT) UserConfigured() bool {
	return c.Enabled || c.InboundRelay || c.OutboundRelay ||
		len(c.RelaySeeds) > 0 || strings.TrimSpace(c.RelayDNSSeed) != "" ||
		strings.TrimSpace(c.AuthToken) != "" || strings.TrimSpace(c.Listen) != "" ||
		c.RelayPort != 0 || c.MaxClients != 0 || c.MaxRelayConns != 0 || len(c.AllowClients) > 0 ||
		len(c.RelayTLSPins) > 0 || c.MaxSessionFramesPerSec != 0 || c.MaxP2PProxyPerSec != 0 ||
		c.MaxRegisterPerMin != 0
}

// EnsureDGRForP2P applies QUIC relay defaults when P2P mode expects DGR and the operator
// has not fully customized dogego_relay_cgnat. CGNAT mode always enables outbound client relay.
func EnsureDGRForP2P(f *File) {
	if f == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(f.P2PConnectivity))
	if mode == "" {
		mode = "both"
	}
	switch mode {
	case "cgnat":
		if !f.DogeGoRelayCGNAT.UserConfigured() {
			ApplyWizardDGRDefaults(f, true)
			return
		}
		if f.DogeGoRelayCGNAT.Enabled {
			f.DogeGoRelayCGNAT.InboundRelay = false
			f.DogeGoRelayCGNAT.OutboundRelay = true
		}
	case "both", "classic":
		if !f.DogeGoRelayCGNAT.UserConfigured() {
			ApplyWizardDGRDefaults(f, false)
		}
	}
}

// For both/classic/cgnat: DGR is enabled. Public inbound relay runs only when not behind CGNAT;
// outbound client relay runs when CGNAT is likely (inbound relay disabled on CGNAT).
func ApplyWizardDGRDefaults(f *File, likelyCGNAT bool) {
	if f == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(f.P2PConnectivity))
	switch mode {
	case "both", "classic", "cgnat":
	default:
		return
	}
	if mode == "cgnat" {
		likelyCGNAT = true
	}
	f.DogeGoRelayCGNAT.Enabled = true
	if likelyCGNAT {
		f.DogeGoRelayCGNAT.InboundRelay = false
		f.DogeGoRelayCGNAT.OutboundRelay = true
	} else {
		f.DogeGoRelayCGNAT.InboundRelay = true
		f.DogeGoRelayCGNAT.OutboundRelay = false
	}
}
