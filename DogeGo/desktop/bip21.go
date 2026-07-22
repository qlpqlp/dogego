// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"fmt"
	"net/url"
	"strings"

	"dogego/chain"
	"dogego/config"
)

// PaymentLink classifies a dogecoin: URI target.
type PaymentLink struct {
	Address string
	Amount  string
	Label   string
	Message string
	Network string // mainnet, testnet, or unknown
}

// ParsePaymentURI parses BIP21-style dogecoin: URIs (with or without //).
func ParsePaymentURI(raw string) (PaymentLink, bool) {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if !strings.HasPrefix(lower, DefaultURLScheme+":") {
		return PaymentLink{}, false
	}
	rest := strings.TrimPrefix(raw, DefaultURLScheme+":")
	rest = strings.TrimPrefix(rest, "//")
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.EqualFold(rest, "node") || strings.HasPrefix(strings.ToLower(rest), "node/") || strings.HasPrefix(strings.ToLower(rest), "node#") {
		return PaymentLink{}, false
	}
	addrPart := rest
	query := ""
	if i := strings.Index(rest, "?"); i >= 0 {
		addrPart = rest[:i]
		query = rest[i+1:]
	}
	addrPart = strings.TrimSpace(addrPart)
	if addrPart == "" {
		return PaymentLink{}, false
	}
	link := PaymentLink{Address: addrPart}
	if query != "" {
		if v, err := url.ParseQuery(query); err == nil {
			link.Amount = strings.TrimSpace(v.Get("amount"))
			link.Label = strings.TrimSpace(v.Get("label"))
			link.Message = strings.TrimSpace(v.Get("message"))
		}
	}
	link.Network = classifyDogecoinAddress(addrPart)
	if link.Network == "" {
		return PaymentLink{}, false
	}
	return link, true
}

func classifyDogecoinAddress(addr string) string {
	payload, err := chain.DecodeBase58CheckBytes(strings.TrimSpace(addr))
	if err != nil || len(payload) != 21 {
		return ""
	}
	ver := payload[0]
	switch ver {
	case 30, 22: // mainnet P2PKH / P2SH
		return "mainnet"
	case 0x41, 0x42: // reboot testnet
		return "testnet"
	case 0x71, 0xc4: // Core testnet3 P2PKH / P2SH
		return "testnet"
	default:
		return "unknown"
	}
}

func paymentDashboardURL(base string, link PaymentLink, conf config.File) string {
	q := url.Values{}
	q.Set("to", link.Address)
	if link.Amount != "" {
		q.Set("amount", link.Amount)
	}
	if link.Label != "" {
		q.Set("label", link.Label)
	}
	if link.Message != "" {
		q.Set("message", link.Message)
	}
	if link.Network != "" {
		q.Set("net", link.Network)
	}
	confNet := strings.ToLower(strings.TrimSpace(conf.Network))
	if confNet == "" {
		confNet = "testnet"
	}
	if link.Network != "" && link.Network != "unknown" && link.Network != confNet {
		q.Set("net_warn", link.Network)
	}
	return strings.TrimSuffix(base, "/") + "#send?" + q.Encode()
}

func resolveCustomScheme(raw string, f config.File) (string, error) {
	base := DashboardURL(f)
	if base == "" {
		return "", fmt.Errorf("web UI is disabled (nowebui)")
	}
	rest := strings.TrimPrefix(raw, DefaultURLScheme+":")
	rest = strings.TrimPrefix(rest, "//")
	rest = strings.TrimSpace(rest)
	if rest == "" || rest == "node" || strings.HasPrefix(rest, "node/") || strings.HasPrefix(rest, "node#") {
		path := strings.TrimPrefix(rest, "node")
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			return base, nil
		}
		if strings.HasPrefix(path, "#") {
			return strings.TrimSuffix(base, "/") + path, nil
		}
		frag := strings.TrimPrefix(path, "#")
		if frag != "" {
			return strings.TrimSuffix(base, "/") + "#" + frag, nil
		}
		return base, nil
	}
	if link, ok := ParsePaymentURI(raw); ok {
		return paymentDashboardURL(base, link, f), nil
	}
	return "", fmt.Errorf("unknown %s URI %q (use %s://node or %s:ADDRESS)", DefaultURLScheme, rest, DefaultURLScheme, DefaultURLScheme)
}
