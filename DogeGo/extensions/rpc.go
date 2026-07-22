// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import "strings"

// RPCMethod is one extension-owned JSON-RPC entry (inner name + help).
type RPCMethod struct {
	Name string `json:"name"`
	Help string `json:"help,omitempty"`
}

// RPCProvider extensions implement this to advertise RPC methods without editing DogeGo core.
type RPCProvider interface {
	RPCMethods() []RPCMethod
}

// Core manager RPCs (always in DogeGo binary; not extension-specific).
var CoreManagerRPC = []string{
	"dogego_listextensions",
	"dogego_listextensioncatalog",
	"dogego_listextensioncatalogsources",
	"dogego_addextensioncatalogsource",
	"dogego_removeextensioncatalogsource",
	"dogego_enableextension",
	"dogego_disableextension",
	"dogego_instextensionzip",
	"dogego_instextensionurl",
	"dogego_instextension",
	"dogego_uninstextension",
}

// ExtRPCPrefix returns dogego_ext_<id>_ with dots → underscores.
func ExtRPCPrefix(extensionID string) string {
	return "dogego_ext_" + strings.ReplaceAll(strings.TrimSpace(extensionID), ".", "_") + "_"
}

// AdvertisedRPCMethods returns manifest-declared RPC or entry-type defaults.
func (m Manifest) AdvertisedRPCMethods() []RPCMethod {
	if len(m.RPC) > 0 {
		out := make([]RPCMethod, 0, len(m.RPC))
		for _, rm := range m.RPC {
			name := strings.TrimSpace(rm.Name)
			if name == "" {
				continue
			}
			out = append(out, RPCMethod{Name: name, Help: rm.Help})
		}
		return out
	}
	switch m.Entry.Type {
	case EntrySubprocess, EntryWasm:
		return []RPCMethod{
			{Name: "info", Help: "Extension status (runtime + optional ui panel)."},
			{Name: "ping", Help: "Round-trip health check."},
		}
	default:
		return nil
	}
}

// FullRPCName builds the public RPC name for an extension inner method.
func FullRPCName(extensionID, inner string) string {
	return ExtRPCPrefix(extensionID) + strings.TrimSpace(inner)
}

// ParseExtRPC splits dogego_ext_<slug>_<inner> into extension id guess and inner method.
// Slug uses underscores; extension ids use dots (dogego.zkl2 → dogego_zkl2).
func ParseExtRPC(method string) (extensionID, inner string, ok bool) {
	const p = "dogego_ext_"
	if !strings.HasPrefix(method, p) {
		return "", "", false
	}
	rest := strings.TrimPrefix(method, p)
	i := strings.LastIndex(rest, "_")
	if i <= 0 {
		return "", "", false
	}
	slug := rest[:i]
	inner = rest[i+1:]
	if inner == "" {
		return "", "", false
	}
	// dogego_zkl2 → dogego.zkl2 (first underscore after dogego → dot)
	extensionID = strings.Replace(slug, "_", ".", 1)
	if !strings.Contains(extensionID, ".") {
		// fallback: only one segment; treat whole slug as id with dots restored lazily
		extensionID = slug
	}
	return extensionID, inner, true
}
