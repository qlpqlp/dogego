// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net"
	"strings"

	"dogego/chain"
)

// LanPeerHint helps operators pair two home-LAN nodes via addnode (DNS seeds do not discover LAN PCs).
type LanPeerHint struct {
	Network      string   `json:"network"`
	P2PPort      int      `json:"p2p_port"`
	LANIPv4      []string `json:"lan_ipv4"`
	ShareTargets []string `json:"share_targets"`
	Note         string   `json:"note"`
}

func p2pPortForNetwork(slug string) int {
	switch strings.ToLower(strings.TrimSpace(slug)) {
	case "main", "mainnet":
		if p, err := chain.ParamsFor(chain.MainnetDogecoin); err == nil {
			return p.Port
		}
		return 22556
	default:
		return chain.Port
	}
}

func isPrivateIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 10 {
		return true
	}
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

func localPrivateIPv4s() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}
			if !isPrivateIPv4(ip) {
				continue
			}
			s := ip.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// BuildLanPeerHint returns shareable host:port targets for manual addnode on a LAN.
func BuildLanPeerHint(network string) LanPeerHint {
	port := p2pPortForNetwork(network)
	ips := localPrivateIPv4s()
	targets := make([]string, 0, len(ips))
	for _, ip := range ips {
		targets = append(targets, net.JoinHostPort(ip, itoa(port)))
	}
	note := "Two PCs on the same home LAN usually need mutual addnode with each other's LAN IP (not the public IP). Reboot testnet P2P port is 44556; mainnet is 22556."
	if len(ips) == 0 {
		note = "No private IPv4 detected on this host. Use ipconfig / ifconfig to find your LAN address, then addnode HOST:PORT on both nodes."
	}
	return LanPeerHint{
		Network:      strings.TrimSpace(network),
		P2PPort:      port,
		LANIPv4:      ips,
		ShareTargets: targets,
		Note:         note,
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
