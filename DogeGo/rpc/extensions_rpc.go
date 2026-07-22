// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/extensions"
)

// ExtensionsManager is the extension host wired from the node.
type ExtensionsManager interface {
	List() []extensions.InstalledRow
	ListCatalog(ctx context.Context, forceRefresh bool) ([]extensions.CatalogRow, error)
	Enable(id string) error
	Disable(id string) error
	InstallZip(path string) (extensions.InstalledRow, error)
	InstallFromURL(ctx context.Context, url, sha256 string) (extensions.InstalledRow, error)
	InstallCatalogEntry(ctx context.Context, id string) (extensions.InstalledRow, error)
	Uninstall(id string, removeData bool) error
	HandleRPC(method string, params []json.RawMessage) (interface{}, error)
	SupportsMethod(method string) bool
	RPCHelp(method string) (string, bool)
	CatalogRPCMethods() []string
	EnabledRPCMethods() []string
	CatalogSources() []string
	AddCatalogSource(url string) ([]string, error)
	RemoveCatalogSource(url string) ([]string, error)
}

func execExtensionsRPC(paths *DataPaths, method string, params []json.RawMessage) (interface{}, int, string) {
	m := activeExtensionsManager(paths)
	if m == nil {
		return nil, -31, "Extensions not available. The extension host is not running in this DogeGo process. Stop any old dogego.exe, rebuild from this repo (go build ./cmd/dogego), and run dogego node again."
	}
	ctx := context.Background()
	switch method {
	case "dogego_listextensions":
		return map[string]interface{}{"extensions": m.List()}, 0, ""
	case "dogego_listextensioncatalog":
		force := false
		if len(params) > 0 {
			var f bool
			if json.Unmarshal(params[0], &f) == nil {
				force = f
			}
		}
		rows, err := m.ListCatalog(ctx, force)
		if err != nil {
			return nil, -1, err.Error()
		}
		return map[string]interface{}{"catalog": rows}, 0, ""
	case "dogego_listextensioncatalogsources":
		return map[string]interface{}{"sources": m.CatalogSources()}, 0, ""
	case "dogego_addextensioncatalogsource":
		if len(params) < 1 {
			return nil, -1, "addextensioncatalogsource requires https url"
		}
		var url string
		if err := json.Unmarshal(params[0], &url); err != nil {
			return nil, -1, "invalid catalog url"
		}
		sources, err := m.AddCatalogSource(strings.TrimSpace(url))
		if err != nil {
			return nil, -1, err.Error()
		}
		return map[string]interface{}{"sources": sources}, 0, ""
	case "dogego_removeextensioncatalogsource":
		if len(params) < 1 {
			return nil, -1, "removeextensioncatalogsource requires url"
		}
		var url string
		if err := json.Unmarshal(params[0], &url); err != nil {
			return nil, -1, "invalid catalog url"
		}
		sources, err := m.RemoveCatalogSource(strings.TrimSpace(url))
		if err != nil {
			return nil, -1, err.Error()
		}
		return map[string]interface{}{"sources": sources}, 0, ""
	case "dogego_enableextension":
		if len(params) < 1 {
			return nil, -1, "enableextension requires extension id"
		}
		var id string
		if err := json.Unmarshal(params[0], &id); err != nil {
			return nil, -1, "invalid extension id"
		}
		if err := m.Enable(strings.TrimSpace(id)); err != nil {
			return nil, -1, err.Error()
		}
		return map[string]interface{}{"enabled": true, "id": id}, 0, ""
	case "dogego_disableextension":
		if len(params) < 1 {
			return nil, -1, "disableextension requires extension id"
		}
		var id string
		if err := json.Unmarshal(params[0], &id); err != nil {
			return nil, -1, "invalid extension id"
		}
		if err := m.Disable(strings.TrimSpace(id)); err != nil {
			return nil, -1, err.Error()
		}
		return map[string]interface{}{"enabled": false, "id": id}, 0, ""
	case "dogego_instextensionzip":
		if len(params) < 1 {
			return nil, -1, "instextensionzip requires zip path"
		}
		var zipPath string
		if err := json.Unmarshal(params[0], &zipPath); err != nil {
			return nil, -1, "invalid zip path"
		}
		row, err := m.InstallZip(strings.TrimSpace(zipPath))
		if err != nil {
			return nil, -1, err.Error()
		}
		return row, 0, ""
	case "dogego_instextensionurl":
		if len(params) < 1 {
			return nil, -1, "instextensionurl requires https url"
		}
		var url string
		var sha string
		if err := json.Unmarshal(params[0], &url); err != nil {
			return nil, -1, "invalid url"
		}
		if len(params) > 1 {
			_ = json.Unmarshal(params[1], &sha)
		}
		row, err := m.InstallFromURL(ctx, url, sha)
		if err != nil {
			return nil, -1, err.Error()
		}
		return row, 0, ""
	case "dogego_instextension":
		if len(params) < 1 {
			return nil, -1, "instextension requires catalog id"
		}
		var id string
		if err := json.Unmarshal(params[0], &id); err != nil {
			return nil, -1, "invalid extension id"
		}
		row, err := m.InstallCatalogEntry(ctx, strings.TrimSpace(id))
		if err != nil {
			return nil, -1, err.Error()
		}
		return row, 0, ""
	case "dogego_uninstextension":
		if len(params) < 1 {
			return nil, -1, "uninstextension requires extension id"
		}
		var id string
		if err := json.Unmarshal(params[0], &id); err != nil {
			return nil, -1, "invalid extension id"
		}
		removeData := true
		if len(params) > 1 {
			_ = json.Unmarshal(params[1], &removeData)
		}
		if err := m.Uninstall(strings.TrimSpace(id), removeData); err != nil {
			return nil, -1, err.Error()
		}
		return map[string]interface{}{"uninstalled": true, "id": id}, 0, ""
	}
	if strings.HasPrefix(method, "dogego_ext_") {
		if !m.SupportsMethod(method) {
			return nil, -32601, fmt.Sprintf("extension RPC %q not available (enable the extension first)", method)
		}
		res, err := m.HandleRPC(method, params)
		if err != nil {
			return nil, -1, err.Error()
		}
		return res, 0, ""
	}
	return nil, -32601, fmt.Sprintf("unknown extension method %q", method)
}

func isExtensionMethod(method string) bool {
	for _, core := range extensions.CoreManagerRPC {
		if method == core {
			return true
		}
	}
	return strings.HasPrefix(method, "dogego_ext_")
}
