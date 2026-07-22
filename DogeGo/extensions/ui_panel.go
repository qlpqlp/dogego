// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import "strings"

// DefaultUIStatusMethod is the inner RPC name extensions use for dashboard status panels.
const DefaultUIStatusMethod = "info"

// ManifestUI declares optional dashboard integration in dogego.extension.json.
type ManifestUI struct {
	// StatusMethod is the inner RPC method that returns a "ui" object (default "info").
	StatusMethod string `json:"status_method,omitempty"`
}

// ManifestUIStatusMethod returns the inner RPC for panel status (default info).
func ManifestUIStatusMethod(m Manifest) string {
	if strings.TrimSpace(m.UI.StatusMethod) != "" {
		return strings.TrimSpace(m.UI.StatusMethod)
	}
	return DefaultUIStatusMethod
}

// PanelStatusRPC builds the full JSON-RPC name for an extension status panel.
func PanelStatusRPC(extensionID, innerMethod string) string {
	if strings.TrimSpace(innerMethod) == "" {
		innerMethod = DefaultUIStatusMethod
	}
	return FullRPCName(extensionID, innerMethod)
}

// UIToolField is one input for extension-owned dashboard tools (generic host renderer).
// Type: text | number | textarea | checkbox | switch | select. Options only for select.
// checkbox and switch both render as a modern toggle; switch is preferred for new panels.
type UIToolField struct {
	Name        string              `json:"name"`
	Label       string              `json:"label,omitempty"`
	Type        string              `json:"type,omitempty"`
	Placeholder string              `json:"placeholder,omitempty"`
	Default     string              `json:"default,omitempty"`
	Hint        string              `json:"hint,omitempty"`
	Options     []map[string]string `json:"options,omitempty"`
}

// UITool is one callable action the web UI can render for an enabled extension.
type UITool struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Method   string        `json:"method"`
	Icon     string        `json:"icon,omitempty"`
	Hint     string        `json:"hint,omitempty"`
	Fields   []UIToolField `json:"fields,omitempty"`
	ParamsAs string        `json:"params_as,omitempty"` // "object" packs fields into one JSON object
}

// UINavItem is one left-menu entry in a workspace panel.
type UINavItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
}

// UISection is one workspace page (shown when its nav id is selected).
type UISection struct {
	Title        string                   `json:"title,omitempty"`
	Lead         string                   `json:"lead,omitempty"`
	Widgets      []map[string]interface{} `json:"widgets,omitempty"`
	QuickActions []map[string]interface{} `json:"quick_actions,omitempty"`
	Tools        []UITool                 `json:"tools,omitempty"`
	Wizards      []map[string]interface{} `json:"wizards,omitempty"`
}

// UIPanel is the secure, host-rendered dashboard contract returned under info.ui.
// Extensions must NOT return HTML/JS/CSS. The host only renders allowlisted fields
// as text (escaped) and allowlisted widget types (stats, proof_list, item_list, table, metric_chart, callout, progress).
type UIPanel struct {
	PanelTitle   string                   `json:"panel_title,omitempty"`
	Subtitle     string                   `json:"subtitle,omitempty"`
	Summary      string                   `json:"summary,omitempty"`
	Layout       string                   `json:"layout,omitempty"` // "workspace" preferred
	StatusChips  []map[string]interface{} `json:"status_chips,omitempty"`
	Nav          []UINavItem              `json:"nav,omitempty"`
	Sections     map[string]UISection     `json:"sections,omitempty"`
	Widgets      []map[string]interface{} `json:"widgets,omitempty"` // legacy flat
	Tools        []UITool                 `json:"tools,omitempty"`    // legacy flat
	QuickActions []map[string]interface{} `json:"quick_actions,omitempty"`
}

// AllowedUIWidgetTypes are the only widget.type values the host renders.
var AllowedUIWidgetTypes = map[string]struct{}{
	"stats":         {},
	"proof_list":    {},
	"item_list":     {},
	"table":         {},
	"metric_chart":  {},
	"callout":       {},
	"progress":      {},
}

// AllowedUIFieldTypes are safe input controls the host will draw.
var AllowedUIFieldTypes = map[string]struct{}{
	"text":     {},
	"number":   {},
	"textarea": {},
	"checkbox": {},
	"switch":   {},
	"select":   {},
	"":         {}, // default text
}

// ToolsFromManifest builds simple dashboard tools from manifest RPC declarations.
func ToolsFromManifest(m Manifest) []UITool {
	methods := m.AdvertisedRPCMethods()
	if len(methods) == 0 {
		return nil
	}
	skip := map[string]struct{}{"info": {}, "ui_status": {}}
	var out []UITool
	for _, rm := range methods {
		name := strings.TrimSpace(rm.Name)
		if name == "" {
			continue
		}
		if _, ok := skip[name]; ok {
			continue
		}
		tool := UITool{
			ID:     name,
			Label:  strings.ReplaceAll(name, "_", " "),
			Method: name,
			Icon:   "play_arrow",
			Hint:   strings.TrimSpace(rm.Help),
		}
		switch name {
		case "ping":
			tool.Label = "Ping"
			tool.Icon = "network_ping"
		case "greet":
			tool.Label = "Greet"
			tool.Icon = "waving_hand"
			tool.Fields = []UIToolField{{Name: "name", Label: "Name", Type: "text", Placeholder: "Doge", Default: "world"}}
		case "counter_inc":
			tool.Label = "Increment counter"
			tool.Icon = "add"
		case "counter_get":
			tool.Label = "Read counter"
			tool.Icon = "tag"
		case "chain_tip":
			tool.Label = "Chain tip"
			tool.Icon = "link"
		}
		out = append(out, tool)
	}
	return out
}

// DefaultWorkspaceUI builds a modern menu shell (Home / Tools / Settings) from title + tools.
// Prefer this over a flat tools list so every extension gets the same navigation pattern.
func DefaultWorkspaceUI(title, subtitle string, tools []UITool) map[string]interface{} {
	if strings.TrimSpace(title) == "" {
		title = "Extension"
	}
	if strings.TrimSpace(subtitle) == "" {
		subtitle = "Use the menu for tools and settings."
	}
	nav := []map[string]interface{}{
		{"id": "home", "label": "Home", "icon": "home"},
	}
	sections := map[string]interface{}{
		"home": map[string]interface{}{
			"title": "Overview",
			"lead":  "Status at a glance. Open Tools for actions, Settings for notes.",
			"quick_actions": []map[string]interface{}{
				{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
			},
		},
	}
	if len(tools) > 0 {
		nav = append(nav, map[string]interface{}{"id": "tools", "label": "Tools", "icon": "construction"})
		toolMaps := make([]map[string]interface{}, 0, len(tools))
		for _, t := range tools {
			toolMaps = append(toolMaps, toolToMap(t))
		}
		sections["tools"] = map[string]interface{}{
			"title": "Tools",
			"lead":  "Safe host-rendered forms. Results appear below.",
			"tools": toolMaps,
		}
	}
	nav = append(nav, map[string]interface{}{"id": "settings", "label": "Settings", "icon": "tune"})
	sections["settings"] = map[string]interface{}{
		"title": "Settings & security",
		"lead":  "Extensions add UI only via this JSON panel (no HTML/JS injection). Wallet access requires wallet_rpc permission, allowlisted methods, dashboard unlock, and usually an in-extension Settings toggle. Extension data/ survives upgrades; use Backup when available.",
		"widgets": []map[string]interface{}{
			{
				"type":  "callout",
				"tone":  "neutral",
				"icon":  "folder_special",
				"title": "Data survives upgrades",
				"body":  "Install/Update keeps extensions/<id>/data/ (databases and settings). Use Backup/Restore tools when the extension provides them.",
			},
		},
	}
	return map[string]interface{}{
		"panel_title": title,
		"subtitle":    subtitle,
		"layout":      "workspace",
		"nav":         nav,
		"sections":    sections,
	}
}

func toolToMap(t UITool) map[string]interface{} {
	m := map[string]interface{}{
		"id": t.ID, "label": t.Label, "method": t.Method,
	}
	if t.Icon != "" {
		m["icon"] = t.Icon
	}
	if t.Hint != "" {
		m["hint"] = t.Hint
	}
	if t.ParamsAs != "" {
		m["params_as"] = t.ParamsAs
	}
	if len(t.Fields) > 0 {
		fields := make([]map[string]interface{}, 0, len(t.Fields))
		for _, f := range t.Fields {
			fm := map[string]interface{}{"name": f.Name}
			if f.Label != "" {
				fm["label"] = f.Label
			}
			if f.Type != "" {
				fm["type"] = f.Type
			}
			if f.Placeholder != "" {
				fm["placeholder"] = f.Placeholder
			}
			if f.Default != "" {
				fm["default"] = f.Default
			}
			if f.Hint != "" {
				fm["hint"] = f.Hint
			}
			if len(f.Options) > 0 {
				fm["options"] = f.Options
			}
			fields = append(fields, fm)
		}
		m["fields"] = fields
	}
	return m
}
