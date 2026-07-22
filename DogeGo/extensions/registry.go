// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"dogego/applog"
)

// InstalledRow is one extension on disk.
type InstalledRow struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Author       string   `json:"author,omitempty"`
	Description  string   `json:"description,omitempty"`
	Homepage     string   `json:"homepage,omitempty"`
	Repository   string   `json:"repository,omitempty"`
	EntryType    string   `json:"entry_type"`
	Builtin      bool     `json:"builtin"`
	Enabled      bool     `json:"enabled"`
	Installed    bool     `json:"installed"`
	Permissions  []string `json:"permissions,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	RPCMethods     []string `json:"rpc_methods,omitempty"`
	UIPanel        bool     `json:"ui_panel,omitempty"`
	UIStatusMethod string   `json:"ui_status_method,omitempty"`
	Icon           string   `json:"icon,omitempty"`
	DocsPath       string   `json:"docs_path,omitempty"`
	Status         string   `json:"status,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type stateFile struct {
	Enabled map[string]bool `json:"enabled"`
}

// Manager installs, lists, enables, and runs extensions.
type Manager struct {
	mu        sync.Mutex
	rootDir   string
	network   string
	chain     ChainReader
	builtins  map[string]BuiltinFactory
	statePath string
	enabled   map[string]bool
	active    map[string]Extension
	host      Host
	peerExts  *peerExtTable
	catalogURL     string
	catalogSources []string
	activeManifest map[string]Manifest
	peerOverlays   map[string]peerOverlayEntry
	// p2pBroadcast relays overlay commands to peers with negotiated protocol (wired from node).
	p2pBroadcast func(cmd string, payload []byte, excludePeer, protocolID string)
	// walletCaller runs whitelisted wallet JSON-RPC (wired from node when wallet enabled).
	walletCaller WalletRPCCaller
}

type peerOverlayEntry struct {
	protocols []string
	send      func(cmd string, payload []byte) error
}

// NewManager creates an extension manager under datadir/extensions/.
func NewManager(datadir, network string, chain ChainReader) *Manager {
	root := filepath.Join(datadir, "extensions")
	return &Manager{
		rootDir:   root,
		network:   strings.ToLower(strings.TrimSpace(network)),
		chain:     chain,
		builtins:  make(map[string]BuiltinFactory),
		statePath: filepath.Join(root, "state.json"),
		enabled:   make(map[string]bool),
		active:    make(map[string]Extension),
		activeManifest: make(map[string]Manifest),
		peerOverlays:   make(map[string]peerOverlayEntry),
		catalogURL: DefaultCatalogURL,
	}
}

// RegisterBuiltin registers a first-party compiled extension factory.
func (m *Manager) RegisterBuiltin(id string, factory BuiltinFactory) {
	if m == nil || factory == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.builtins == nil {
		m.builtins = make(map[string]BuiltinFactory)
	}
	m.builtins[strings.TrimSpace(id)] = factory
}

// Load reads state and enables configured extensions.
func (m *Manager) Load() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if err := os.MkdirAll(m.rootDir, 0o755); err != nil {
		m.mu.Unlock()
		return err
	}
	m.loadStateLocked()
	m.loadCatalogSourcesLocked()
	m.host = &managerHost{m: m}
	var toEnable []string
	for id, on := range m.enabled {
		if on {
			toEnable = append(toEnable, id)
		}
	}
	m.mu.Unlock()
	for _, id := range toEnable {
		if err := m.Enable(id); err != nil {
			applog.Line("extensions", fmt.Sprintf("enable %s failed: %v", id, err))
		}
	}
	return nil
}

func (m *Manager) loadStateLocked() {
	raw, err := os.ReadFile(m.statePath)
	if err != nil {
		return
	}
	var st stateFile
	if json.Unmarshal(raw, &st) == nil && st.Enabled != nil {
		m.enabled = st.Enabled
	}
}

func (m *Manager) saveStateLocked() error {
	st := stateFile{Enabled: m.enabled}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.statePath)
}

func (m *Manager) isInstalledLocked(id string) bool {
	if m == nil {
		return false
	}
	dir := filepath.Join(m.rootDir, id)
	if _, err := LoadManifest(dir); err != nil {
		return false
	}
	return true
}

// List returns catalog + installed extension rows.
func (m *Manager) List() []InstalledRow {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]struct{})
	var out []InstalledRow
	for id, factory := range m.builtins {
		if !m.isInstalledLocked(id) {
			continue
		}
		ext, err := factory(Manifest{ID: id, Entry: Entry{Type: EntryBuiltin, Module: id}})
		if err != nil {
			continue
		}
		man := ext.Manifest()
		row := manifestRow(man, true, true)
		attachRPCMethods(&row, ext)
		row.Enabled = m.enabled[id]
		if _, ok := m.active[id]; ok {
			row.Status = "running"
		} else if row.Enabled {
			row.Status = "enabled"
		}
		out = append(out, row)
		seen[id] = struct{}{}
	}
	entries, _ := os.ReadDir(m.rootDir)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		id := ent.Name()
		if _, ok := seen[id]; ok {
			continue
		}
		dir := filepath.Join(m.rootDir, id)
		man, err := LoadManifest(dir)
		if err != nil {
			out = append(out, InstalledRow{ID: id, Installed: true, Status: "invalid", Error: err.Error()})
			continue
		}
		row := manifestRow(man, false, true)
		if ext := m.extensionForCatalogLocked(dir, man); ext != nil {
			attachRPCMethods(&row, ext)
		}
		row.Enabled = m.enabled[id]
		if _, ok := m.active[id]; ok {
			row.Status = "running"
		} else if row.Enabled {
			row.Status = "enabled"
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func manifestRow(m Manifest, builtin, installed bool) InstalledRow {
	row := InstalledRow{
		ID:           m.ID,
		Name:         m.Name,
		Version:      m.Version,
		Author:       m.Author,
		Description:  m.Description,
		Homepage:     m.Homepage,
		Repository:   m.Repository,
		EntryType:    string(m.Entry.Type),
		Builtin:      builtin,
		Installed:    installed,
		Permissions:  append([]string(nil), m.Permissions...),
		Capabilities: append([]string(nil), m.Capabilities...),
		Icon:         NormalizeIconRel(m.Icon),
		DocsPath:     EnrichDocsPath(m.ID, m.DocsPath),
		Status:       "installed",
	}
	if m.HasPermission("ui_panel") {
		row.UIPanel = true
		row.UIStatusMethod = ManifestUIStatusMethod(m)
	}
	return row
}

func attachRPCMethods(row *InstalledRow, ext Extension) {
	if row == nil || ext == nil {
		return
	}
	rp, ok := ext.(RPCProvider)
	if !ok {
		return
	}
	for _, rm := range rp.RPCMethods() {
		row.RPCMethods = append(row.RPCMethods, FullRPCName(row.ID, rm.Name))
	}
}

func (m *Manager) extensionForCatalogLocked(dir string, man Manifest) Extension {
	switch man.Entry.Type {
	case EntrySubprocess:
		ext, err := NewSubprocessExtension(dir, man)
		if err != nil {
			return nil
		}
		return ext
	case EntryWasm:
		ext, err := NewWasmExtension(dir, man)
		if err != nil {
			return nil
		}
		return ext
	default:
		return nil
	}
}

// Enable turns on an extension by id.
func (m *Manager) Enable(id string) error {
	if m == nil {
		return fmt.Errorf("extensions unwired")
	}
	m.mu.Lock()
	if ext, ok := m.active[id]; ok && ext != nil {
		m.mu.Unlock()
		return nil
	}
	ext, man, host, err := m.prepareEnableLocked(id)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	if err := ext.OnEnable(nil, host); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ext2, ok := m.active[id]; ok && ext2 != nil && ext2 != ext {
		_ = ext.OnDisable()
		return nil
	}
	m.active[id] = ext
	m.activeManifest[id] = man
	m.enabled[id] = true
	applog.Line("extensions", fmt.Sprintf("enabled %s %s", id, man.Version))
	return m.saveStateLocked()
}

func (m *Manager) prepareEnableLocked(id string) (Extension, Manifest, Host, error) {
	if ext, ok := m.active[id]; ok && ext != nil {
		return nil, Manifest{}, nil, nil
	}
	if _, ok := m.builtins[id]; ok && !m.isInstalledLocked(id) {
		return nil, Manifest{}, nil, fmt.Errorf("extension %q is not installed; install from catalog or zip first", id)
	}
	ext, man, err := m.resolveExtensionLocked(id)
	if err != nil {
		return nil, Manifest{}, nil, err
	}
	if !man.SupportsNetwork(m.network) {
		return nil, Manifest{}, nil, fmt.Errorf("extension %q not supported on network %q", id, m.network)
	}
	if m.host == nil {
		m.host = &managerHost{m: m}
	}
	return ext, man, m.hostFor(id, man), nil
}

// Disable stops an extension.
func (m *Manager) Disable(id string) error {
	if m == nil {
		return fmt.Errorf("extensions unwired")
	}
	m.mu.Lock()
	ext := m.active[id]
	man := m.activeManifest[id]
	dir := filepath.Join(m.rootDir, id)
	delete(m.active, id)
	delete(m.activeManifest, id)
	delete(m.enabled, id)
	m.mu.Unlock()
	if ext != nil {
		_ = ext.OnDisable()
	} else {
		// Already inactive in registry, but a Windows subprocess may still hold the .exe.
		forceKillExtensionBinary(dir, man.Entry.Binary)
		killStaleSubprocess(filepath.Join(dir, "data"))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveStateLocked()
}

// HandleRPC dispatches extension-scoped RPC (dogego_ext_<id>_...).
func (m *Manager) HandleRPC(method string, params []json.RawMessage) (interface{}, error) {
	if m == nil {
		return nil, fmt.Errorf("extensions unwired")
	}
	m.mu.Lock()
	var (
		ext   Extension
		man   Manifest
		inner string
		id    string
	)
	for eid, e := range m.active {
		prefix := ExtRPCPrefix(eid)
		if !strings.HasPrefix(method, prefix) {
			continue
		}
		inner = strings.TrimPrefix(method, prefix)
		if inner == "" {
			continue
		}
		man = m.activeManifest[eid]
		if !man.HasPermission("rpc_register") {
			m.mu.Unlock()
			return nil, fmt.Errorf("extension %q lacks rpc_register permission", eid)
		}
		ext = e
		id = eid
		break
	}
	if ext == nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("unknown extension rpc %q", method)
	}
	if m.host == nil {
		m.host = &managerHost{m: m}
	}
	host := m.hostFor(id, man)
	m.mu.Unlock()
	return ext.HandleRPC(inner, params, host)
}

// SupportsMethod reports whether method is a core extension-manager RPC or an active extension RPC.
func (m *Manager) SupportsMethod(method string) bool {
	if m == nil {
		return false
	}
	for _, core := range CoreManagerRPC {
		if method == core {
			return true
		}
	}
	if !strings.HasPrefix(method, "dogego_ext_") {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.active {
		if strings.HasPrefix(method, ExtRPCPrefix(id)) {
			return true
		}
	}
	return false
}

// CatalogRPCMethods returns public RPC names for all registered extensions (builtin catalog).
func (m *Manager) CatalogRPCMethods() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	seen := make(map[string]struct{})
	for id, factory := range m.builtins {
		if !m.isInstalledLocked(id) {
			continue
		}
		ext, err := factory(Manifest{ID: id, Entry: Entry{Type: EntryBuiltin, Module: id}})
		if err != nil {
			continue
		}
		appendExtensionRPCNames(&out, seen, id, ext)
	}
	entries, _ := os.ReadDir(m.rootDir)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(m.rootDir, ent.Name())
		man, err := LoadManifest(dir)
		if err != nil {
			continue
		}
		if ext := m.extensionForCatalogLocked(dir, man); ext != nil {
			appendExtensionRPCNames(&out, seen, man.ID, ext)
			continue
		}
		for _, rm := range man.AdvertisedRPCMethods() {
			full := FullRPCName(man.ID, rm.Name)
			if _, dup := seen[full]; dup {
				continue
			}
			seen[full] = struct{}{}
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}

func appendExtensionRPCNames(out *[]string, seen map[string]struct{}, id string, ext Extension) {
	rp, ok := ext.(RPCProvider)
	if !ok {
		return
	}
	for _, rm := range rp.RPCMethods() {
		full := FullRPCName(id, rm.Name)
		if _, dup := seen[full]; dup {
			continue
		}
		seen[full] = struct{}{}
		*out = append(*out, full)
	}
}

// EnabledRPCMethods returns RPC names for currently active extensions.
func (m *Manager) EnabledRPCMethods() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for id, ext := range m.active {
		rp, ok := ext.(RPCProvider)
		if !ok {
			continue
		}
		for _, rm := range rp.RPCMethods() {
			out = append(out, FullRPCName(id, rm.Name))
		}
	}
	return out
}

// RPCHelp returns help text for an extension RPC method.
func (m *Manager) RPCHelp(method string) (string, bool) {
	if m == nil || !strings.HasPrefix(method, "dogego_ext_") {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, ext := range m.active {
		prefix := ExtRPCPrefix(id)
		if !strings.HasPrefix(method, prefix) {
			continue
		}
		inner := strings.TrimPrefix(method, prefix)
		rp, ok := ext.(RPCProvider)
		if !ok {
			return "", false
		}
		for _, rm := range rp.RPCMethods() {
			if rm.Name == inner {
				return rm.Help, true
			}
		}
	}
	// Installed manifest fallback when extension not enabled.
	entries, _ := os.ReadDir(m.rootDir)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(m.rootDir, ent.Name())
		man, err := LoadManifest(dir)
		if err != nil {
			continue
		}
		prefix := ExtRPCPrefix(man.ID)
		if !strings.HasPrefix(method, prefix) {
			continue
		}
		inner := strings.TrimPrefix(method, prefix)
		for _, rm := range man.AdvertisedRPCMethods() {
			if rm.Name == inner {
				return rm.Help + " (extension not enabled)", true
			}
		}
	}
	// Builtin catalog fallback when extension not enabled (installed package only).
	for id, factory := range m.builtins {
		if !m.isInstalledLocked(id) {
			continue
		}
		prefix := ExtRPCPrefix(id)
		if !strings.HasPrefix(method, prefix) {
			continue
		}
		inner := strings.TrimPrefix(method, prefix)
		ext, err := factory(Manifest{ID: id, Entry: Entry{Type: EntryBuiltin, Module: id}})
		if err != nil {
			return "", false
		}
		rp, ok := ext.(RPCProvider)
		if !ok {
			return "", false
		}
		for _, rm := range rp.RPCMethods() {
			if rm.Name == inner {
				return rm.Help + " (extension not enabled)", true
			}
		}
	}
	return "", false
}

func (m *Manager) resolveExtensionLocked(id string) (Extension, Manifest, error) {
	if factory, ok := m.builtins[id]; ok {
		man, err := m.builtinManifestLocked(id)
		if err != nil {
			return nil, Manifest{}, err
		}
		ext, err := factory(man)
		return ext, man, err
	}
	dir := filepath.Join(m.rootDir, id)
	man, err := LoadManifest(dir)
	if err != nil {
		return nil, Manifest{}, err
	}
	switch man.Entry.Type {
	case EntryBuiltin:
		factory, ok := m.builtins[man.Entry.Module]
		if !ok {
			return nil, Manifest{}, fmt.Errorf("unknown builtin module %q", man.Entry.Module)
		}
		ext, err := factory(man)
		return ext, man, err
	case EntrySubprocess:
		ext, err := NewSubprocessExtension(dir, man)
		if err != nil {
			return nil, Manifest{}, err
		}
		return ext, man, nil
	case EntryWasm:
		ext, err := NewWasmExtension(dir, man)
		if err != nil {
			return nil, Manifest{}, err
		}
		return ext, man, nil
	default:
		return nil, Manifest{}, fmt.Errorf("unsupported entry type")
	}
}

func (m *Manager) builtinManifestLocked(id string) (Manifest, error) {
	factory, ok := m.builtins[id]
	if !ok {
		return Manifest{}, fmt.Errorf("unknown builtin %q", id)
	}
	ext, err := factory(Manifest{ID: id, Entry: Entry{Type: EntryBuiltin, Module: id}})
	if err != nil {
		return Manifest{}, err
	}
	return ext.Manifest(), nil
}

type managerHost struct {
	m *Manager
}

func (h *managerHost) Network() string {
	if h.m == nil {
		return ""
	}
	return h.m.network
}

func (h *managerHost) TipHeight() (int64, error) {
	if h.m == nil || h.m.chain == nil {
		return -1, fmt.Errorf("chain unwired")
	}
	return h.m.chain.TipHeight()
}

func (h *managerHost) GetRawBlockByHeight(height int64) ([]byte, error) {
	if h.m == nil || h.m.chain == nil {
		return nil, fmt.Errorf("chain unwired")
	}
	return h.m.chain.GetRawBlockByHeight(height)
}

func (h *managerHost) LookupTxHex(txid string) (string, int64, bool) {
	if h.m == nil || h.m.chain == nil {
		return "", 0, false
	}
	return h.m.chain.LookupTxHex(txid)
}

func (h *managerHost) BlockHashAtHeight(height int64) (string, error) {
	if h.m == nil || h.m.chain == nil {
		return "", fmt.Errorf("chain unwired")
	}
	return h.m.chain.BlockHashAtHeight(height)
}

func (h *managerHost) ConfirmedTxInBlock(blockHash, txid string) (uint32, bool) {
	if h.m == nil || h.m.chain == nil {
		return 0, false
	}
	return h.m.chain.ConfirmedTxInBlock(blockHash, txid)
}

func (h *managerHost) DataDir() string {
	if h.m == nil {
		return ""
	}
	return filepath.Dir(h.m.rootDir)
}

func (h *managerHost) ExtensionDataDir(id string) (string, error) {
	if h.m == nil {
		return "", fmt.Errorf("extensions unwired")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid extension id")
	}
	dir := filepath.Join(h.m.rootDir, id, "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (h *managerHost) Log(line string) {
	applog.Line("ext", line)
}

func (h *managerHost) BroadcastOverlay(protocolID, cmd string, payload []byte, excludePeer string) error {
	if h.m == nil || h.m.p2pBroadcast == nil {
		return nil
	}
	h.m.p2pBroadcast(cmd, payload, excludePeer, protocolID)
	return nil
}

// OverlayHost extends Host with decentralized overlay broadcast (P2P extensions).
type OverlayHost interface {
	Host
	BroadcastOverlay(protocolID, cmd string, payload []byte, excludePeer string) error
	EachOverlayPeer(protocolID string, fn func(peer string, send func(string, []byte) error))
	OverlayPeerCount(protocolID string) int
}

func (h *managerHost) EachOverlayPeer(protocolID string, fn func(peer string, send func(string, []byte) error)) {
	if h.m == nil || fn == nil {
		return
	}
	h.m.eachOverlayPeer(protocolID, fn)
}

func (h *managerHost) OverlayPeerCount(protocolID string) int {
	if h.m == nil {
		return 0
	}
	return h.m.OverlayPeerCount(protocolID)
}

// SetP2PBroadcast wires overlay relay from the node (peerMgr filtered by negotiated protocol).
func (m *Manager) SetP2PBroadcast(fn func(cmd string, payload []byte, excludePeer, protocolID string)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.p2pBroadcast = fn
	m.mu.Unlock()
}

// SetWalletRPC wires whitelisted wallet JSON-RPC for extensions with wallet_rpc permission.
func (m *Manager) SetWalletRPC(c WalletRPCCaller) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.walletCaller = c
	m.mu.Unlock()
}

// NotifyBlockConnected indexes new chain tips for active extensions.
func (m *Manager) NotifyBlockConnected(height int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	active := make(map[string]Extension, len(m.active))
	manifests := make(map[string]Manifest, len(m.activeManifest))
	for id, ext := range m.active {
		active[id] = ext
		manifests[id] = m.activeManifest[id]
	}
	m.mu.Unlock()
	for id, ext := range active {
		if bi, ok := ext.(BlockIndexExtension); ok {
			_ = bi.OnBlockConnected(height, m.hostFor(id, manifests[id]))
		}
	}
}

// NotifyPeerNegotiated informs extensions that a peer agreed on overlay protocols.
func (m *Manager) NotifyPeerNegotiated(peerAddr string, protocols []string, send func(string, []byte) error) {
	if m == nil || peerAddr == "" {
		return
	}
	if send != nil {
		m.registerPeerOverlay(peerAddr, protocols, send)
	}
	m.mu.Lock()
	active := make([]Extension, 0, len(m.active))
	for _, ext := range m.active {
		active = append(active, ext)
	}
	m.mu.Unlock()
	for _, ext := range active {
		if ps, ok := ext.(PeerSyncExtension); ok {
			ps.OnPeerConnected(peerAddr, protocols, send)
		}
	}
}

func (m *Manager) registerPeerOverlay(peerAddr string, protocols []string, send func(string, []byte) error) {
	if m == nil || send == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.peerOverlays == nil {
		m.peerOverlays = make(map[string]peerOverlayEntry)
	}
	m.peerOverlays[peerAddr] = peerOverlayEntry{
		protocols: append([]string(nil), protocols...),
		send:      send,
	}
}

func (m *Manager) UnregisterPeerOverlay(peerAddr string) {
	if m == nil || peerAddr == "" {
		return
	}
	m.mu.Lock()
	delete(m.peerOverlays, peerAddr)
	m.mu.Unlock()
}

func (m *Manager) eachOverlayPeer(protocolID string, fn func(peer string, send func(string, []byte) error)) {
	if m == nil || fn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for addr, ent := range m.peerOverlays {
		if ent.send == nil {
			continue
		}
		for _, p := range ent.protocols {
			if p == protocolID {
				fn(addr, ent.send)
				break
			}
		}
	}
}

// OverlayPeerCount returns peers with negotiated protocol.
func (m *Manager) OverlayPeerCount(protocolID string) int {
	if m == nil {
		return 0
	}
	n := 0
	m.mu.Lock()
	for _, ent := range m.peerOverlays {
		for _, p := range ent.protocols {
			if p == protocolID {
				n++
				break
			}
		}
	}
	m.mu.Unlock()
	return n
}

// BlockIndexExtension receives new block heights for L1 indexing.
type BlockIndexExtension interface {
	OnBlockConnected(height int64, host Host) error
}

// PeerSyncExtension starts overlay sync when a compatible peer connects.
type PeerSyncExtension interface {
	OnPeerConnected(peerAddr string, protocols []string, send func(string, []byte) error)
}

// RootDir is the extensions install directory.
func (m *Manager) RootDir() string {
	if m == nil {
		return ""
	}
	return m.rootDir
}

// ActiveExtension returns a running extension by id.
func (m *Manager) ActiveExtension(id string) (Extension, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ext, ok := m.active[id]
	return ext, ok
}
