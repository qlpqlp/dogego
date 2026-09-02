// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"dogego/applog"
	"dogego/bloom"
	"dogego/chain"
	"dogego/extensions"
	"dogego/mempool"
	"dogego/node/dgr"
	"dogego/store"
	"dogego/wire"
)

// PeerMgr maintains extra inbound/outbound P2P sessions for relay (primary sync peer stays on main loop).
type PeerMgr struct {
	mu sync.Mutex

	p2p           P2PModeSettings
	params        chain.Params
	userAgent     string
	localServices uint64 // NODE_* bits on version/addr (0 → params.NodeNetwork)
	relayEnv      RelayEnv

	sessions map[string]*peerLink
	primary  string
	order    []string

	addrs         *AddrBook
	addrStorePath string

	banCheck func(net.IP) bool

	dialer      net.Dialer
	blockScorer *BlockPeerScorer

	extMgr *extensions.Manager

	discoveryFeed  *PeerDiscoveryFeed
	preferredAddrs []string // addnode host:ports (dial before other learned peers)

	listenHost string // P2P inbound bind address (from net.Listener)
	listenPort int

	mappedExtHost string // UPnP/NAT-PMP public address (when mapped)
	mappedExtPort int
	mappedMethod  string
}

type peerLink struct {
	id       int
	addr     string
	inbound  bool
	conn     net.Conn
	mw       *MsgWriter
	ctr      *netByteCounter
	peer     *wire.DecodedVersion
	since    time.Time
	lastRecv time.Time
	primary  bool
	cancel   context.CancelFunc
	// cmpctHBFrom: peer sendcmpct had announce=true (peer sends us cmpctblock).
	cmpctHBFrom       bool
	// cmpctHBTo: we sent peer sendcmpct announce=true (we send them cmpctblock).
	cmpctHBTo         bool
	cmpctPending      *cmpctPending
	cmpctWireIgnored  bool
	ping              peerPingTracker
	peerFeeFilter     uint64 // BIP133 feefilter from this peer (0 = none)
	timeOffset        int32  // peer version nTime − local at connect (Core getpeerinfo)
	lastBlockRecv     time.Time
	lastTxRecv        time.Time
	lastSend          time.Time
	lastBodyPump      time.Time // relay lane proactive getdata during body IBD
	msgStats          *peerMsgStats
	addrProcessed     uint64
	addrRateLimited   uint64
	addrTokenBucket   float64
	addrTokenMicros   int64
	dgrTunneled       bool
	bestHeaderHeight  int64  // Core getpeerinfo synced_headers (-1 unknown)
	bestHeaderHash    string // tip hash hex for bestHeaderHeight (empty unknown)
	tipUpdatedUnix    int64  // last time best header tip was updated
	commonBlockHeight int64  // Core getpeerinfo synced_blocks (-1 unknown)
	bloom             *bloom.Filter // BIP37 per-peer filter (nil = none)
}

// NewPeerMgr creates a peer manager; call RegisterPrimary, SetRelayEnv, then Start.
func NewPeerMgr(p2p P2PModeSettings, p chain.Params, userAgent string, dialer net.Dialer) *PeerMgr {
	return &PeerMgr{
		p2p:       p2p,
		params:    p,
		userAgent: userAgent,
		dialer:    dialer,
		sessions:  make(map[string]*peerLink),
		addrs:     NewAddrBook(),
	}
}

// SetLocalServices sets NODE_* bits advertised on outbound/inbound handshakes (BIP157 COMPACT_FILTERS when filters enabled).
func (pm *PeerMgr) SetLocalServices(services uint64) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	pm.localServices = services
	pm.mu.Unlock()
}

func (pm *PeerMgr) advertisedServices() uint64 {
	if pm == nil {
		return 0
	}
	pm.mu.Lock()
	s := pm.localServices
	pm.mu.Unlock()
	if s == 0 {
		return pm.params.NodeNetwork
	}
	return s
}

// SetExtensionManager wires extension overlay negotiation for relay/inbound peers.
func (pm *PeerMgr) SetExtensionManager(mgr *extensions.Manager) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	pm.extMgr = mgr
	pm.mu.Unlock()
}

func (pm *PeerMgr) extensionManager() *extensions.Manager {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.extMgr
}

// SetRelayEnv configures handlers for non-primary peer loops (call before Start).
func (pm *PeerMgr) SetRelayEnv(env RelayEnv) {
	if pm != nil {
		pm.relayEnv = env
	}
}

// SetAddrStorePath enables load/save of learned_addrs.json under the chain data directory.
func (pm *PeerMgr) SetAddrStorePath(path string) {
	if pm != nil {
		pm.addrStorePath = path
	}
}

// SetAddrBook installs a pre-loaded addrbook (e.g. after startup header probe); skips disk load when non-empty.
func (pm *PeerMgr) SetAddrBook(book *AddrBook) {
	if pm == nil || book == nil {
		return
	}
	pm.mu.Lock()
	pm.addrs = book
	pm.mu.Unlock()
}

// SetPreferredPeers sets addnode host:ports to prefer when dialing outbound relays.
func (pm *PeerMgr) SetPreferredPeers(addrs []string) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	pm.preferredAddrs = append([]string(nil), addrs...)
	pm.mu.Unlock()
}

// SetBanChecker rejects inbound peers when the callback returns true (Core setban on accept).
func (pm *PeerMgr) SetBanChecker(fn func(net.IP) bool) {
	if pm != nil {
		pm.banCheck = fn
	}
}

// SetBlockPeerScorer ranks outbound relay dials using block-download history (Core addrman-lite).
func (pm *PeerMgr) SetBlockPeerScorer(scorer *BlockPeerScorer) {
	if pm != nil {
		pm.blockScorer = scorer
	}
}

// SaveLearnedAddrsNow flushes the address pool to disk when a store path is configured.
func (pm *PeerMgr) SaveLearnedAddrsNow() {
	if pm != nil {
		pm.saveLearnedAddrsIfDirty()
	}
}

// MaxPeerStartHeight returns the highest nStartingHeight among connected P2P sessions (0 if none).
func (pm *PeerMgr) MaxPeerStartHeight() int32 {
	if pm == nil {
		return 0
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var max int32
	for _, l := range pm.sessions {
		if l.peer != nil && l.peer.StartHeight > max {
			max = l.peer.StartHeight
		}
	}
	return max
}

// DropPrimary removes the primary session entry (connection already closed by run.go).
func (pm *PeerMgr) DropPrimary(addr string) {
	if pm == nil || addr == "" {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[addr]; ok && l.primary {
		delete(pm.sessions, addr)
	}
	if pm.primary == addr {
		pm.primary = ""
	}
	var order []string
	for _, a := range pm.order {
		if a != addr {
			order = append(order, a)
		}
	}
	pm.order = order
}

// RegisterPrimary records the main sync peer (owned by run.go read loop).
func (pm *PeerMgr) RegisterPrimary(addr string, conn net.Conn, mw *MsgWriter, ctr *netByteCounter, dv *wire.DecodedVersion) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.primary = addr
	if mw != nil {
		mw.PeerAddr = addr
	}
	link := &peerLink{
		id: 1, addr: addr, conn: conn, mw: mw, ctr: ctr, peer: dv,
		since: time.Now(), primary: true,
		timeOffset: wire.TimeOffsetSeconds(dv, time.Now().Unix()),
	}
	attachPeerMsgStats(link, mw)
	link.grantAddrTokens(maxAddrToSend)
	pm.sessions[addr] = link
	pm.order = []string{addr}
}

// Start launches inbound listener and outbound maintainer when mode requires them.
func (pm *PeerMgr) Start(ctx context.Context, candidates []string, exclude string) {
	if pm == nil {
		return
	}
	if pm.p2p.MaxOutbound <= 1 && !pm.p2p.Listen {
		return
	}
	pm.loadLearnedAddrsFromDisk()
	pm.seedCandidates(candidates, exclude)
	if pm.addrStorePath != "" {
		go pm.addrSaveLoop(ctx)
	}
	if pm.p2p.Listen {
		go pm.listenLoop(ctx)
	}
	if pm.p2p.MaxOutbound > 1 {
		go pm.outboundMaintainer(ctx)
	}
}

func (pm *PeerMgr) loadLearnedAddrsFromDisk() {
	if pm == nil || pm.addrStorePath == "" {
		return
	}
	pm.mu.Lock()
	already := pm.addrs != nil
	pm.mu.Unlock()
	if already {
		return
	}
	book, err := LoadAddrBook(pm.addrStorePath)
	if err != nil {
		applog.Line("net", "learned_addrs load: "+err.Error())
		return
	}
	if book == nil || len(book.Snapshot()) == 0 {
		return
	}
	pm.mu.Lock()
	pm.addrs = book
	pm.mu.Unlock()
	applog.Line("net", fmt.Sprintf("loaded %d learned peer address(es) from disk (addrbook v2)", len(book.Snapshot())))
}

func (pm *PeerMgr) addrSaveLoop(ctx context.Context) {
	tick := time.NewTicker(2 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			pm.saveLearnedAddrsIfDirty()
			return
		case <-tick.C:
			pm.saveLearnedAddrsIfDirty()
		}
	}
}

func (pm *PeerMgr) saveLearnedAddrsIfDirty() {
	pm.mu.Lock()
	if pm.addrStorePath == "" || pm.addrs == nil || !pm.addrs.takeDirty() {
		pm.mu.Unlock()
		return
	}
	book := pm.addrs
	path := pm.addrStorePath
	pm.mu.Unlock()
	if err := SaveAddrBook(path, book); err != nil {
		applog.Line("net", "learned_addrs save: "+err.Error())
		book.markDirty()
	}
}

func (pm *PeerMgr) seedCandidates(candidates []string, exclude string) {
	if pm != nil && pm.blockScorer != nil {
		candidates = pm.blockScorer.MergeDiscoveryCandidates(candidates, -1)
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, a := range candidates {
		if a == "" || a == exclude {
			continue
		}
		if pm.addrs != nil && IsHostPortRoutable(a) {
			pm.addrs.AddSeen(a)
		}
	}
}

// SetDiscoveryFeed links inbound addr learning to the shared discovery feed (assist / redial).
func (pm *PeerMgr) SetDiscoveryFeed(f *PeerDiscoveryFeed) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	pm.discoveryFeed = f
	pm.mu.Unlock()
}

// AddrBookStats returns tried vs new learned address counts (Core addrman analogue).
func (pm *PeerMgr) AddrBookStats() (tried, newAddrs int) {
	if pm == nil {
		return 0, 0
	}
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book == nil {
		return 0, 0
	}
	return book.AddrBookStats()
}

// AddrBookBucketStats returns Core addrman bucket spread stats from the learned addrbook.
func (pm *PeerMgr) AddrBookBucketStats() (triedBucketsUsed, newBucketsUsed, triedMaxFill, newMaxFill int) {
	if pm == nil {
		return 0, 0, 0, 0
	}
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book == nil {
		return 0, 0, 0, 0
	}
	return book.AddrBookBucketStats()
}

// HasAddrmanKey reports whether the learned addrbook has a Core-style nKey loaded.
func (pm *PeerMgr) HasAddrmanKey() bool {
	if pm == nil {
		return false
	}
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	return book != nil && book.HasAddrmanKey()
}

// AddrManInfo returns a Core getaddrmaninfo-shaped summary for RPC.
func (b *AddrBook) AddrManInfo() map[string]interface{} {
	if b == nil {
		return nil
	}
	tried, newAddrs := b.AddrBookStats()
	tbUsed, nbUsed, tbMax, nbMax := b.AddrBookBucketStats()
	total := tried + newAddrs
	return map[string]interface{}{
		"all": map[string]interface{}{
			"total": total,
			"new":   newAddrs,
			"tried": tried,
		},
		"dogego_buckets": map[string]interface{}{
			"n_key_set":                b.HasAddrmanKey(),
			"tried_buckets_total":      addrTriedBucketCount,
			"new_buckets_total":        addrNewBucketCount,
			"bucket_slot_cap":          addrBucketSlotCap,
			"tried_buckets_used":       tbUsed,
			"new_buckets_used":         nbUsed,
			"tried_bucket_max_fill":    tbMax,
			"new_bucket_max_fill":      nbMax,
			"tried_table_max":          maxAddrBookTried,
			"new_table_max":            maxAddrBookNew,
		},
	}
}

// AddrManInfo returns Core getaddrmaninfo summary from the learned addrbook.
func (pm *PeerMgr) AddrManInfo() map[string]interface{} {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book == nil {
		return map[string]interface{}{
			"all": map[string]interface{}{"total": 0, "new": 0, "tried": 0},
		}
	}
	return book.AddrManInfo()
}

// NodeAddressRows returns Core getnodeaddresses rows from the learned addrbook.
func (pm *PeerMgr) NodeAddressRows(count int, networkFilter string) []map[string]interface{} {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	book := pm.addrs
	svc := pm.localServices
	pm.mu.Unlock()
	if book == nil {
		return nil
	}
	if svc == 0 {
		svc = pm.params.NodeNetwork
	}
	return book.NodeAddressRows(count, networkFilter, svc)
}

// RelayAddrBook returns addrman for DGR relay QUIC target reputation.
func (pm *PeerMgr) RelayAddrBook() *AddrBook {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	return book
}

// NoteAddr learns a peer address from an "addr" message.
func (pm *PeerMgr) NoteAddr(hostport string) {
	if hostport == "" {
		return
	}
	pm.mu.Lock()
	if pm.addrs != nil {
		pm.addrs.AddSeen(hostport)
	}
	feed := pm.discoveryFeed
	pm.mu.Unlock()
	if feed != nil {
		feed.Note(hostport)
	}
}

// NoteAddrFrom learns host:port plus CAddress time/services from a decoded addr entry.
func (pm *PeerMgr) NoteAddrFrom(a wire.NetAddress, fromPeer string) {
	hp := a.HostPort()
	if hp == "" || !IsIPPortRoutable(a.IP, a.Port) {
		return
	}
	pm.mu.Lock()
	if pm.addrs != nil {
		pm.addrs.AddSeenFrom(hp, a.Services, int64(a.Time), fromPeer)
	}
	feed := pm.discoveryFeed
	pm.mu.Unlock()
	if feed != nil {
		feed.Note(hp)
	}
}

// NoteAddrsFromPeer ingests a decoded addr message with Core token-bucket rate limiting.
func (pm *PeerMgr) NoteAddrsFromPeer(fromPeer string, addrs []wire.NetAddress) {
	if pm == nil || fromPeer == "" || len(addrs) == 0 {
		return
	}
	shuffled := append([]wire.NetAddress(nil), addrs...)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	pm.mu.Lock()
	whitelisted := pm.peerWhitelistedLocked(fromPeer)
	link := pm.sessions[fromPeer]
	nowMicro := time.Now().UnixMicro()
	if link != nil {
		link.refillAddrTokensLocked(nowMicro)
	}
	pm.mu.Unlock()

	var processed, rateLimited uint64
	for _, a := range shuffled {
		pm.mu.Lock()
		link = pm.sessions[fromPeer]
		if link == nil {
			pm.mu.Unlock()
			break
		}
		if !whitelisted && link.addrTokenBucket < 1.0 {
			rateLimited++
			pm.mu.Unlock()
			continue
		}
		if !whitelisted {
			link.addrTokenBucket -= 1.0
		}
		processed++
		pm.mu.Unlock()

		hp := a.HostPort()
		if hp == "" || !IsIPPortRoutable(a.IP, a.Port) {
			continue
		}
		pm.NoteAddrFrom(a, fromPeer)
	}

	pm.mu.Lock()
	if l := pm.sessions[fromPeer]; l != nil {
		l.addrProcessed += processed
		l.addrRateLimited += rateLimited
	}
	pm.mu.Unlock()
}

// BroadcastGetAddr asks all non-primary relay peers for more addresses.
func (pm *PeerMgr) BroadcastGetAddr() {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	writers := make([]*MsgWriter, 0, len(pm.sessions))
	for _, link := range pm.sessions {
		if link.primary || link.mw == nil {
			continue
		}
		writers = append(writers, link.mw)
	}
	pm.mu.Unlock()
	for _, w := range writers {
		if err := w.Write("getaddr", nil); err == nil {
			pm.NoteOutboundGetAddr(w.PeerAddr)
		}
	}
}

// DisconnectAllRelays closes every non-primary session (setnetworkactive false analogue).
func (pm *PeerMgr) DisconnectAllRelays() {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	var addrs []string
	for addr, l := range pm.sessions {
		if l != nil && !l.primary {
			addrs = append(addrs, addr)
		}
	}
	pm.mu.Unlock()
	for _, addr := range addrs {
		pm.removeSession(addr)
	}
}

// DisconnectPeer closes a non-primary session matching addr (Core disconnectnode).
func (pm *PeerMgr) DisconnectPeer(addr string) error {
	if pm == nil {
		return fmt.Errorf("multi-peer P2P is not active")
	}
	pm.mu.Lock()
	if addnodeMatchesSession(addr, pm.primary) {
		pm.mu.Unlock()
		return fmt.Errorf("cannot disconnect the primary sync peer")
	}
	target := ""
	for sessionAddr := range pm.sessions {
		if addnodeMatchesSession(addr, sessionAddr) {
			if sessionAddr == pm.primary {
				pm.mu.Unlock()
				return fmt.Errorf("cannot disconnect the primary sync peer")
			}
			target = sessionAddr
			break
		}
	}
	pm.mu.Unlock()
	if target == "" {
		return fmt.Errorf("Node not found in connected nodes")
	}
	pm.removeSession(target)
	return nil
}

// DialOnce attempts a single outbound connection to addr (Core addnode onetry).
func (pm *PeerMgr) DialOnce(ctx context.Context, addr string) error {
	if pm == nil {
		return fmt.Errorf("multi-peer P2P is not active")
	}
	book := addrBookFromPeerMgr(pm)
	RecordOutboundDialTry(book, addr)
	c, viaDGR, err := pm.dialOutbound(ctx, addr)
	if err != nil {
		RecordOutboundHandshakeResult(book, addr, err)
		return err
	}
	dv, err := Handshake(ctx, c, pm.params, pm.userAgent, pm.advertisedServices())
	if err != nil {
		_ = c.Close()
		RecordOutboundHandshakeResult(book, addr, err)
		return err
	}
	RecordOutboundHandshakeResult(book, addr, nil)
	ctr := newNetByteCounter()
	wrapped := &countingConn{Conn: c, ctr: ctr}
	mw := NewMsgWriter(wrapped, pm.params.Magic)
	mw.PeerAddr = addr
	if !pm.attachSession(ctx, addr, wrapped, mw, ctr, dv, false, viaDGR) {
		_ = wrapped.Close()
		return fmt.Errorf("could not attach onetry peer (at connection limit?)")
	}
	pm.SaveLearnedAddrsNow()
	applog.Line("net", "onetry peer connected "+addr)
	return nil
}

func (pm *PeerMgr) listenLoop(ctx context.Context) {
	addr := fmt.Sprintf(":%d", pm.params.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		applog.Line("net", "P2P inbound listen "+addr+": "+err.Error())
		return
	}
	applog.Line("net", fmt.Sprintf("P2P inbound listening on %s (%s)", addr, pm.p2p.Mode))
	if tcp, ok := ln.Addr().(*net.TCPAddr); ok {
		pm.mu.Lock()
		pm.listenHost = tcp.IP.String()
		pm.listenPort = tcp.Port
		pm.mu.Unlock()
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if ctx.Err() != nil {
				return
			}
			applog.Line("net", "P2P accept: "+err.Error())
			time.Sleep(time.Second)
			continue
		}
		remote := conn.RemoteAddr().String()
		if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			remote = net.JoinHostPort(tcp.IP.String(), fmt.Sprintf("%d", tcp.Port))
		}
		go pm.acceptInbound(ctx, conn, remote)
	}
}

func (pm *PeerMgr) acceptInbound(ctx context.Context, conn net.Conn, addr string) {
	if pm.banCheck != nil {
		if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok && pm.banCheck(tcp.IP) {
			_ = conn.Close()
			applog.Line("net", "inbound rejected (banned) "+addr)
			return
		}
	}
	if pm.inboundCount() >= pm.p2p.MaxInbound {
		if !pm.acceptInboundOrEvict(addr) {
			_ = conn.Close()
			return
		}
	}
	dv, err := Handshake(ctx, conn, pm.params, pm.userAgent, pm.advertisedServices())
	if err != nil {
		_ = conn.Close()
		applog.Line("net", "inbound handshake "+addr+": "+err.Error())
		return
	}
	ctr := newNetByteCounter()
	wrapped := &countingConn{Conn: conn, ctr: ctr}
	mw := NewMsgWriter(wrapped, pm.params.Magic)
	mw.PeerAddr = addr
	if !pm.attachSession(ctx, addr, wrapped, mw, ctr, dv, true, false) {
		_ = wrapped.Close()
		return
	}
	pm.noteInboundPeerLearned(addr, dv)
	applog.Line("net", "inbound peer connected "+addr+" ("+pm.p2p.Mode+")")
}

// noteInboundPeerLearned records a routable inbound peer in the addrbook (new table) and discovery feed.
// Core learns reachable peers from addr gossip; DogeGo also seeds from successful inbound handshakes.
func (pm *PeerMgr) noteInboundPeerLearned(addr string, dv *wire.DecodedVersion) {
	if pm == nil || addr == "" || !IsHostPortRoutable(addr) {
		return
	}
	var svc uint64
	if dv != nil {
		svc = dv.Services
	}
	pm.mu.Lock()
	if pm.addrs != nil {
		pm.addrs.AddSeenMeta(addr, svc, time.Now().Unix())
	}
	feed := pm.discoveryFeed
	pm.mu.Unlock()
	if feed != nil {
		feed.Note(addr)
	}
}

// SetDGRTunnel is a no-op; global DGR dial policy is set in bootDGR via ConfigureDGRTunnelDial.
func (pm *PeerMgr) SetDGRTunnel(_ DGRTunnelRelay, _ func() bool) {}

func (pm *PeerMgr) dialOutbound(ctx context.Context, addr string) (net.Conn, bool, error) {
	return DialP2POutbound(ctx, pm.dialer, addr)
}

func (pm *PeerMgr) outboundMaintainer(ctx context.Context) {
	pm.maintainerOnce(ctx)
	for {
		interval := pm.maintainerTickInterval()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			pm.maintainerOnce(ctx)
		}
	}
}

func (pm *PeerMgr) tryDialMore(ctx context.Context) {
	want := pm.p2p.MaxOutbound - 1
	if want < 1 {
		return
	}
	// During deep body IBD, dedicated assist/header sockets already carry getdata.
	// Filling MaxOutbound with idle full-relay peers wastes dials and archival slots.
	if pm.ibdBlockSyncActive() && want > 8 {
		want = 8
	}
	for pm.outboundCount() < want {
		addr := pm.pickDialCandidate()
		if addr == "" {
			return
		}
		book := addrBookFromPeerMgr(pm)
		RecordOutboundDialTry(book, addr)
		c, viaDGR, err := pm.dialOutbound(ctx, addr)
		if err != nil {
			RecordOutboundHandshakeResult(book, addr, err)
			if pm.blockScorer != nil {
				pm.blockScorer.NoteDialFailure(addr)
			}
			applog.Line("net", "outbound dial "+addr+": "+err.Error())
			continue
		}
		dv, err := Handshake(ctx, c, pm.params, pm.userAgent, pm.advertisedServices())
		if err != nil {
			_ = c.Close()
			RecordOutboundHandshakeResult(book, addr, err)
			if pm.blockScorer != nil {
				pm.blockScorer.NoteDialFailure(addr)
			}
			applog.Line("net", "outbound handshake "+addr+": "+err.Error())
			continue
		}
		RecordOutboundHandshakeResult(book, addr, nil)
		ctr := newNetByteCounter()
		wrapped := &countingConn{Conn: c, ctr: ctr}
		mw := NewMsgWriter(wrapped, pm.params.Magic)
		mw.PeerAddr = addr
		if !pm.attachSession(ctx, addr, wrapped, mw, ctr, dv, false, viaDGR) {
			_ = wrapped.Close()
			continue
		}
		applog.Line("net", fmt.Sprintf("outbound relay peer %s (%s)", addr, pm.p2p.Mode))
		if viaDGR {
			applog.Line("dgr", "P2P session "+addr+" via QUIC tunnel")
		}
	}
}

// AddrPoolSnapshot returns learned peer host:ports (for block-assist candidate refresh).
func (pm *PeerMgr) AddrPoolSnapshot() []string {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.addrs == nil {
		return nil
	}
	return pm.addrs.Snapshot()
}

func (pm *PeerMgr) pickDialCandidate() string {
	pm.mu.Lock()
	primary := pm.primary
	skip := make(map[string]struct{}, len(pm.sessions))
	for a := range pm.sessions {
		skip[a] = struct{}{}
	}
	book := pm.addrs
	scorer := pm.blockScorer
	prefer := append([]string(nil), pm.preferredAddrs...)
	pm.mu.Unlock()
	if book == nil {
		return ""
	}
	return book.PickBest(skip, primary, scorer, prefer)
}

func (pm *PeerMgr) noteAddrTry(addr string) {
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book != nil {
		book.NoteTry(addr)
	}
}

func (pm *PeerMgr) noteAddrSuccess(addr string) {
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book != nil {
		book.NoteSuccess(addr)
	}
}

func (pm *PeerMgr) noteAddnodePersistent(addr string) {
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book != nil {
		book.NoteAddnodePersistent(addr)
	}
}

func (pm *PeerMgr) noteAddrFailure(addr string) {
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book != nil {
		book.NoteFailure(addr)
	}
}

func (pm *PeerMgr) attachSession(ctx context.Context, addr string, conn net.Conn, mw *MsgWriter, ctr *netByteCounter, dv *wire.DecodedVersion, inbound, dgrTunneled bool) bool {
	pm.mu.Lock()
	if _, dup := pm.sessions[addr]; dup {
		pm.mu.Unlock()
		return false
	}
	if inbound && pm.inboundCountLocked() >= pm.p2p.MaxInbound {
		pm.mu.Unlock()
		return false
	}
	id := len(pm.order) + 1
	sessCtx, cancel := context.WithCancel(ctx)
	link := &peerLink{
		id: id, addr: addr, inbound: inbound, conn: conn, mw: mw, ctr: ctr,
		peer: dv, since: time.Now(), cancel: cancel, dgrTunneled: dgrTunneled,
		timeOffset: wire.TimeOffsetSeconds(dv, time.Now().Unix()),
	}
	initPeerSyncFromVersion(link, dv)
	attachPeerMsgStats(link, mw)
	if !inbound {
		link.grantAddrTokens(maxAddrToSend)
	}
	pm.sessions[addr] = link
	pm.order = append(pm.order, addr)
	env := pm.relayEnv
	pm.mu.Unlock()
	if env.Pool != nil {
		_ = SendFeeFilter(mw, env.Pool)
	}
	if !inbound {
		_ = SendCmpctOnConnect(pm, link, mw)
	}
	if extMgr := pm.extensionManager(); extMgr != nil && conn != nil && mw != nil {
		maybeNegotiateExtensions(conn, pm.params.Magic, addr, extMgr, mw)
	}
	go runRelayPeerSession(sessCtx, env, pm, link)
	return true
}

func (pm *PeerMgr) removeSession(addr string) {
	pm.mu.Lock()
	link, ok := pm.sessions[addr]
	env := pm.relayEnv
	if ok {
		delete(pm.sessions, addr)
		if link.cancel != nil {
			link.cancel()
		}
		if link.conn != nil {
			_ = link.conn.Close()
		}
	}
	var next []string
	for _, a := range pm.order {
		if a != addr {
			next = append(next, a)
		}
	}
	pm.order = next
	pm.mu.Unlock()
	if ok && env.PeerFeeFilter != nil {
		env.PeerFeeFilter.Remove(addr)
	}
	if ok {
		if extMgr := pm.extensionManager(); extMgr != nil {
			extMgr.UnregisterPeer(addr)
		}
	}
	if ok && env.Orphans != nil {
		if n := env.Orphans.RemoveByPeer(addr); n > 0 {
			applog.Line("mempool", fmt.Sprintf("removed %d orphan tx(s) from disconnected peer %s", n, addr))
		}
	}
}

// NotePeerFeeFilter records a peer's BIP133 feefilter and updates the aggregate set.
func (pm *PeerMgr) NotePeerFeeFilter(addr string, rate uint64) {
	if pm == nil || addr == "" {
		return
	}
	pm.mu.Lock()
	if l, ok := pm.sessions[addr]; ok {
		l.peerFeeFilter = rate
	}
	env := pm.relayEnv
	pm.mu.Unlock()
	if env.PeerFeeFilter != nil {
		env.PeerFeeFilter.Set(addr, rate)
	}
}

// SetPeerBloom installs or replaces the BIP37 filter for a peer session.
func (pm *PeerMgr) SetPeerBloom(addr string, f *bloom.Filter) {
	if pm == nil || addr == "" {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[addr]; ok {
		l.bloom = f
	}
}

// ClearPeerBloom removes the BIP37 filter for a peer.
func (pm *PeerMgr) ClearPeerBloom(addr string) {
	pm.SetPeerBloom(addr, nil)
}

// PeerBloom returns the peer's bloom filter (may be nil).
func (pm *PeerMgr) PeerBloom(addr string) *bloom.Filter {
	if pm == nil || addr == "" {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[addr]; ok {
		return l.bloom
	}
	return nil
}

// PeerRelayTxes reports whether the peer's version fRelay allows unsolicited tx inv.
func (pm *PeerMgr) PeerRelayTxes(addr string) bool {
	if pm == nil || addr == "" {
		return true
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[addr]; ok && l.peer != nil {
		return l.peer.RelayTxes
	}
	return true
}

func (pm *PeerMgr) inboundCount() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.inboundCountLocked()
}

func (pm *PeerMgr) inboundCountLocked() int {
	n := 0
	for _, l := range pm.sessions {
		if l.inbound && !l.primary {
			n++
		}
	}
	return n
}

func (pm *PeerMgr) outboundCount() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	n := 0
	for _, l := range pm.sessions {
		if !l.inbound && !l.primary {
			n++
		}
	}
	return n
}

// SessionCount is total P2P TCP sessions including primary.
func (pm *PeerMgr) SessionCount() int {
	total, _, _ := pm.ConnectionBreakdown()
	return total
}

// ConnectionBreakdown returns total sessions, inbound relay count, and outbound relay count (excludes primary).
func (pm *PeerMgr) ConnectionBreakdown() (total, inbound, outboundRelay int) {
	if pm == nil {
		return 0, 0, 0
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, l := range pm.sessions {
		total++
		if l.primary {
			continue
		}
		if l.inbound {
			inbound++
		} else {
			outboundRelay++
		}
	}
	return total, inbound, outboundRelay
}

// CloseAllConnections closes every peer TCP/tunnel connection so blocked reads
// unblock during shutdown instead of waiting on long read deadlines.
func (pm *PeerMgr) CloseAllConnections() {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	conns := make([]net.Conn, 0, len(pm.sessions))
	cancels := make([]context.CancelFunc, 0, len(pm.sessions))
	for _, l := range pm.sessions {
		if l == nil {
			continue
		}
		if l.conn != nil {
			conns = append(conns, l.conn)
		}
		if l.cancel != nil {
			cancels = append(cancels, l.cancel)
		}
	}
	pm.mu.Unlock()
	for _, c := range cancels {
		c()
	}
	for _, c := range conns {
		_ = c.Close()
	}
}

// PingAll queues an immediate outbound ping on every active session (Core ping RPC).
func (pm *PeerMgr) PingAll() {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	type pingTarget struct {
		mw   *MsgWriter
		ping *peerPingTracker
	}
	var targets []pingTarget
	for _, l := range pm.sessions {
		if l == nil || l.mw == nil {
			continue
		}
		targets = append(targets, pingTarget{mw: l.mw, ping: &l.ping})
	}
	pm.mu.Unlock()
	for _, t := range targets {
		t.ping.forcePing(t.mw)
	}
}

// MaybePingPrimary sends an outbound ping on the primary link when the interval elapsed.
func (pm *PeerMgr) MaybePingPrimary(mw *MsgWriter) {
	if pm == nil || mw == nil {
		return
	}
	pm.mu.Lock()
	link := pm.sessions[pm.primary]
	pm.mu.Unlock()
	if link != nil {
		link.ping.maybePing(mw)
	}
}

// NotePeerPong records RTT for a connected peer when pong echoes our ping nonce.
func (pm *PeerMgr) NotePeerPong(addr string, payload []byte) {
	if pm == nil || addr == "" {
		return
	}
	pm.mu.Lock()
	link := pm.sessions[addr]
	pm.mu.Unlock()
	if link != nil {
		link.ping.notePong(payload)
	}
}

// NoteCmpctIgnored handles unexpected compact-block wire on peers that did not negotiate BIP152 HB.
func (pm *PeerMgr) NoteCmpctIgnored(addr, cmd string, mw *MsgWriter, mb *MisbehaviorTracker) {
	if pm == nil || addr == "" {
		return
	}
	pm.mu.Lock()
	link := pm.sessions[addr]
	pm.mu.Unlock()
	if link != nil {
		link.NoteCmpctWireIgnored(mw, cmd, mb)
	}
}

// NotePeerMsg records inbound wire bytes for one P2P command (Core bytesrecv_per_msg).
func (pm *PeerMgr) NotePeerMsg(addr, cmd string, payloadLen int) {
	if pm == nil || addr == "" || cmd == "" {
		return
	}
	n := p2pFrameBytes(payloadLen)
	pm.mu.Lock()
	stats := (*peerMsgStats)(nil)
	if l := pm.sessions[addr]; l != nil {
		stats = l.msgStats
	}
	pm.mu.Unlock()
	if stats != nil {
		stats.addRecv(cmd, n)
	}
}

// NotePeerRecv marks activity on a connected peer (updates lastrecv for getpeerinfo).
func (pm *PeerMgr) NotePeerRecv(addr string) {
	if pm == nil || addr == "" {
		return
	}
	pm.mu.Lock()
	if l, ok := pm.sessions[addr]; ok {
		l.lastRecv = time.Now()
	}
	pm.mu.Unlock()
}

// NotePeerBlock records that addr delivered a full block (Core getpeerinfo last_block).
func (pm *PeerMgr) NotePeerBlock(addr string) {
	if pm == nil || addr == "" {
		return
	}
	now := time.Now()
	pm.mu.Lock()
	if l, ok := pm.sessions[addr]; ok {
		l.lastBlockRecv = now
		l.lastRecv = now
	}
	pm.mu.Unlock()
	pm.noteAddrTouch(addr)
}

func (pm *PeerMgr) noteAddrTouch(addr string) {
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	if book != nil {
		book.TouchSeen(addr)
	}
}

// MedianTimeOffset returns the median peer version nTime offset across connected sessions (Core getnetworkinfo timeoffset).
func (pm *PeerMgr) MedianTimeOffset() int32 {
	if pm == nil {
		return 0
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	offsets := make([]int32, 0, len(pm.sessions))
	for _, l := range pm.sessions {
		if l == nil {
			continue
		}
		offsets = append(offsets, l.timeOffset)
	}
	return medianInt32(offsets)
}

func medianInt32(v []int32) int32 {
	n := len(v)
	if n == 0 {
		return 0
	}
	cp := append([]int32(nil), v...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	mid := n / 2
	if n%2 == 1 {
		return cp[mid]
	}
	return int32((int64(cp[mid-1]) + int64(cp[mid])) / 2)
}

// NotePeerTx records that addr delivered an accepted transaction (Core getpeerinfo last_transaction).
func (pm *PeerMgr) NotePeerTx(addr string) {
	if pm == nil || addr == "" {
		return
	}
	now := time.Now()
	pm.mu.Lock()
	if l, ok := pm.sessions[addr]; ok {
		l.lastTxRecv = now
		l.lastRecv = now
	}
	pm.mu.Unlock()
}

// LinkByAddr returns the MsgWriter for a connected peer (primary or relay).
func (pm *PeerMgr) LinkByAddr(addr string) *MsgWriter {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[addr]; ok {
		return l.mw
	}
	return nil
}

// PrimaryWriter returns the primary peer MsgWriter.
func (pm *PeerMgr) PrimaryWriter() *MsgWriter {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if l, ok := pm.sessions[pm.primary]; ok {
		return l.mw
	}
	return nil
}

// BroadcastTx relays a tx via inv to non-primary peers (excluding excludeAddr, the gossip source).
func (pm *PeerMgr) BroadcastTx(raw []byte, excludeAddr string, pool *mempool.Pool, txIx *store.TxIndex, rawBlocks *store.RawBlockStore) {
	if pm == nil || len(raw) == 0 {
		return
	}
	type target struct {
		mw        *MsgWriter
		fee       uint64
		bloom     *bloom.Filter
		relayTxes bool
	}
	pm.mu.Lock()
	targets := make([]target, 0, len(pm.sessions))
	for addr, l := range pm.sessions {
		if l.primary || l.mw == nil || addr == excludeAddr {
			continue
		}
		relay := true
		if l.peer != nil {
			relay = l.peer.RelayTxes
		}
		targets = append(targets, target{mw: l.mw, fee: l.peerFeeFilter, bloom: l.bloom, relayTxes: relay})
	}
	pm.mu.Unlock()
	for _, t := range targets {
		_ = RelayTxToPeerBloom(t.mw, raw, t.fee, t.bloom, t.relayTxes, pool, txIx, rawBlocks)
	}
}

// BroadcastCmdFiltered relays a P2P message to peers matching include(addr).
func (pm *PeerMgr) BroadcastCmdFiltered(cmd string, payload []byte, excludeAddr string, include func(addr string) bool) {
	if pm == nil || cmd == "" || include == nil {
		return
	}
	pm.mu.Lock()
	links := make([]struct {
		addr string
		mw   *MsgWriter
	}, 0, len(pm.sessions))
	for addr, l := range pm.sessions {
		if l.mw == nil || addr == excludeAddr {
			continue
		}
		if !include(addr) {
			continue
		}
		links = append(links, struct {
			addr string
			mw   *MsgWriter
		}{addr, l.mw})
	}
	pm.mu.Unlock()
	for _, l := range links {
		_ = l.mw.Write(cmd, payload)
	}
}

// BroadcastCmd relays a P2P message to non-primary peers (excluding excludeAddr, the source).
func (pm *PeerMgr) BroadcastCmd(cmd string, payload []byte, excludeAddr string) {
	if pm == nil || cmd == "" {
		return
	}
	pm.mu.Lock()
	links := make([]*MsgWriter, 0, len(pm.sessions))
	for addr, l := range pm.sessions {
		if l.primary || l.mw == nil || addr == excludeAddr {
			continue
		}
		links = append(links, l.mw)
	}
	pm.mu.Unlock()
	for _, mw := range links {
		_ = mw.Write(cmd, payload)
	}
}

// AddrSample returns up to n addresses for getaddr replies (tried-first addrbook sample; block scorer tie-break on dials).
func (pm *PeerMgr) AddrSample(n int) []wire.NetAddress {
	if pm == nil || n <= 0 {
		return nil
	}
	pm.mu.Lock()
	book := pm.addrs
	svc := pm.advertisedServices()
	pm.mu.Unlock()
	if book != nil {
		if out := book.AddrSample(n, svc); len(out) > 0 {
			return out
		}
	}
	return nil
}

// SetMaxConnections updates the outbound peer cap (includes primary). Core setmaxconnections minimum is 8.
func (pm *PeerMgr) SetMaxConnections(max int) error {
	if pm == nil {
		return fmt.Errorf("multi-peer P2P is not active (set p2p_connectivity to cgnat or both)")
	}
	if max < 8 || max > 32 {
		return fmt.Errorf("maxconnectioncount must be between 8 and 32 for DogeGo")
	}
	pm.mu.Lock()
	pm.p2p.MaxOutbound = max
	pm.mu.Unlock()
	applog.Line("net", fmt.Sprintf("setmaxconnections: max outbound peers now %d", max))
	return nil
}

// P2PModeSummary returns a short status string for logs and RPC notes.
func (pm *PeerMgr) P2PModeSummary() string {
	if pm == nil {
		return ""
	}
	return pm.p2p.Description
}

// RelayCGNATPeers lists connected P2P peers advertising NODE_DOGEGO_RELAY_CGNAT.
func (pm *PeerMgr) RelayCGNATPeers() []dgr.P2PRelayPeer {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var out []dgr.P2PRelayPeer
	for addr, link := range pm.sessions {
		if link == nil || link.peer == nil {
			continue
		}
		if !chain.HasDogeGoRelayCGNAT(link.peer.Services) {
			continue
		}
		out = append(out, dgr.P2PRelayPeer{TCPAddr: addr, Services: link.peer.Services})
	}
	return out
}
