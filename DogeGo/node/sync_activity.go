// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"sync"
	"time"

	"dogego/applog"
)

// SyncActivityInput is live node context when building the operator activity snapshot.
type SyncActivityInput struct {
	HeaderTip              int64
	PeerStartHeight        int32
	HeaderCatchUpPending   bool
	BodyIBDHeaderPaused    bool
	HeaderRecoveryRunning  bool
	DedicatedHeaderRunning bool
	PrimaryPeer            string
	PeerDialing            bool
	ConnectionsTotal       int
	ConnectionsOutbound    int
	BlockAssistActive      bool
	BlockAssistConnections int
	ContiguousBodies       int64
	ChainActiveHeight      int64
	ConnectLag             int64
	ConnectBlocksPerMinute float64
	LowestMissing          int64
	NextProbe              int64
	InFlightBatches        int
	BlocksPerMinute        float64
	HeaderRecoveryHint     string
}

var syncActivity syncActivityState

type syncActivityState struct {
	mu sync.Mutex

	lastProgressAt time.Time
	lastProgress   string
	lastKind       string // headers, blocks, p2p, recovery

	headerRecoveryPass   int
	headerRecoveryDetail string
	headerRecoverySince  time.Time

	dedicatedAddr   string
	dedicatedDetail string

	lastBlockFetch   string
	lastBlockFetchAt time.Time

	recoveryKickSuppressed int
	lastRecoveryKickLog  time.Time
	lastRecoveryForceLog time.Time

	watchdogStallTip   int64
	watchdogStallCount int
	lastWatchdogLog    time.Time
}

func syncActivityNote(kind, msg string) {
	if msg == "" {
		return
	}
	syncActivity.mu.Lock()
	syncActivity.lastProgressAt = time.Now()
	syncActivity.lastProgress = msg
	syncActivity.lastKind = kind
	syncActivity.mu.Unlock()
}

// NoteHeadersAppended records validated header batches.
func NoteHeadersAppended(count int, tip int64) {
	msg := fmt.Sprintf("Validated %d headers (local tip height %d)", count, tip)
	syncActivityNote("headers", msg)
	applog.Line("headers", msg)
}

// NoteBlockStored records a stored raw block (body on disk; connect may still be pending during IBD).
func NoteBlockStored(height int64) {
	msg := fmt.Sprintf("Stored block height %d", height)
	syncActivityNote("blocks", msg)
}

// NoteBlockConnected records a successful ConnectBlock / chainActive advance.
func NoteBlockConnected(height int64) {
	msg := fmt.Sprintf("Connected block height %d", height)
	syncActivityNote("blocks", msg)
	applog.Line("block", msg)
	RecordIBDConnectAdvance(height)
}

// NoteBlockPeerDisconnect records Core-style disconnect after block stall or download timeout.
func NoteBlockPeerDisconnect(peer string, reason string) {
	if peer == "" {
		return
	}
	syncActivityNote("blocks", fmt.Sprintf("Disconnected %s (%s)", peer, reason))
}

// NoteBlockGetdata records an outbound block batch request.
func NoteBlockGetdata(lo, hi int64, lane int) {
	syncActivity.mu.Lock()
	syncActivity.lastBlockFetch = fmt.Sprintf("Requested blocks %d-%d (lane %d)", lo, hi, lane)
	syncActivity.lastBlockFetchAt = time.Now()
	syncActivity.mu.Unlock()
}

// NoteHeaderRecoveryPass updates background header recovery status.
func NoteHeaderRecoveryPass(pass int, detail string) {
	syncActivity.mu.Lock()
	syncActivity.headerRecoveryPass = pass
	syncActivity.headerRecoveryDetail = detail
	if syncActivity.headerRecoverySince.IsZero() {
		syncActivity.headerRecoverySince = time.Now()
	}
	syncActivity.mu.Unlock()
	syncActivityNote("recovery", fmt.Sprintf("Header recovery pass %d: %s", pass, detail))
}

// NoteHeaderRecoveryKickSuppressed counts watchdog kicks while recovery already runs.
func NoteHeaderRecoveryKickSuppressed() {
	syncActivity.mu.Lock()
	syncActivity.recoveryKickSuppressed++
	syncActivity.mu.Unlock()
}

// ClearHeaderRecoveryRuntime resets pass timers after a forced recovery restart.
func ClearHeaderRecoveryRuntime() {
	syncActivity.mu.Lock()
	syncActivity.headerRecoverySince = time.Time{}
	syncActivity.headerRecoveryPass = 0
	syncActivity.headerRecoveryDetail = ""
	syncActivity.recoveryKickSuppressed = 0
	syncActivity.mu.Unlock()
}

// NoteDedicatedHeaderSync updates dedicated header peer status.
func NoteDedicatedHeaderSync(addr, detail string) {
	syncActivity.mu.Lock()
	syncActivity.dedicatedAddr = addr
	syncActivity.dedicatedDetail = detail
	syncActivity.mu.Unlock()
	if detail != "" {
		syncActivityNote("headers", "Dedicated header peer "+addr+": "+detail)
	}
}

// NoteDedicatedHeaderSyncDone clears dedicated header status when the goroutine exits.
func NoteDedicatedHeaderSyncDone(addr string, err error) {
	syncActivity.mu.Lock()
	if syncActivity.dedicatedAddr == addr {
		syncActivity.dedicatedAddr = ""
		if err != nil {
			syncActivity.dedicatedDetail = err.Error()
		} else {
			syncActivity.dedicatedDetail = "finished"
		}
	}
	syncActivity.mu.Unlock()
	if err != nil {
		syncActivityNote("headers", "Dedicated header peer "+addr+" ended: "+err.Error())
	}
}

// NoteWatchdogHeaderStall logs at most once per tip per 2 minutes (see RecordWatchdogHeaderStall).
func NoteWatchdogHeaderStall(tip int64, peerH int32, kickStarted bool) bool {
	logNow, _ := RecordWatchdogHeaderStall(tip, peerH)
	return logNow
}

// syncActivityBodyDownloadActive is true when block body IBD is actively making progress.
func syncActivityBodyDownloadActive(in SyncActivityInput) bool {
	if in.BlocksPerMinute > 0 || in.ConnectBlocksPerMinute > 0 {
		return true
	}
	if in.InFlightBatches > 0 {
		return true
	}
	if in.BlockAssistActive && in.ConnectLag > 0 {
		return true
	}
	if in.LowestMissing >= 0 && in.LowestMissing > in.ContiguousBodies {
		return true
	}
	return false
}

// BuildSyncActivitySnapshot returns JSON for /api/summary and the Sync tab.
func BuildSyncActivitySnapshot(in SyncActivityInput) map[string]any {
	syncActivity.mu.Lock()
	lastAt := syncActivity.lastProgressAt
	lastMsg := syncActivity.lastProgress
	lastKind := syncActivity.lastKind
	recPass := syncActivity.headerRecoveryPass
	recDetail := syncActivity.headerRecoveryDetail
	recSince := syncActivity.headerRecoverySince
	dedAddr := syncActivity.dedicatedAddr
	dedDetail := syncActivity.dedicatedDetail
	lastFetch := syncActivity.lastBlockFetch
	lastFetchAt := syncActivity.lastBlockFetchAt
	kickSupp := syncActivity.recoveryKickSuppressed
	syncActivity.mu.Unlock()

	now := time.Now()
	var secSinceProgress int64 = -1
	if !lastAt.IsZero() {
		secSinceProgress = int64(now.Sub(lastAt).Seconds())
	}

	headline, detail := syncActivityHeadline(in, secSinceProgress, lastKind, lastMsg, recPass, recDetail, dedAddr, dedDetail, lastFetch, kickSupp)
	tasks := syncActivityTasks(in, recPass, recDetail, recSince, dedAddr, dedDetail, lastFetch, lastFetchAt, kickSupp)

	stalled := secSinceProgress >= 300 && in.HeaderTip > 0 && in.ContiguousBodies >= 0 &&
		in.HeaderTip > in.ContiguousBodies+32 && !syncActivityBodyDownloadActive(in)

	out := map[string]any{
		"headline":               headline,
		"detail":                 detail,
		"last_progress_at":       unixOrZero(lastAt),
		"last_progress_message":  lastMsg,
		"last_progress_kind":     lastKind,
		"seconds_since_progress": secSinceProgress,
		"stalled":                stalled,
		"tasks":                  tasks,
		"header_recovery_running":  in.HeaderRecoveryRunning,
		"dedicated_header_running": in.DedicatedHeaderRunning,
		"recovery_kicks_suppressed": kickSupp,
	}
	if !lastFetchAt.IsZero() {
		out["last_block_fetch_at"] = lastFetchAt.Unix()
		out["last_block_fetch"] = lastFetch
	}
	return out
}

func syncActivityHeadline(in SyncActivityInput, secSince int64, lastKind, lastMsg string, recPass int, recDetail, dedAddr, dedDetail, lastFetch string, kickSupp int) (string, string) {
	if in.PeerDialing && in.ConnectionsOutbound == 0 && in.BlockAssistConnections == 0 && !in.DedicatedHeaderRunning {
		return "Connecting to peers", "Dialing DNS seeds and addrbook - block and header sync start after the first handshakes."
	}
	var detail string
	if lastMsg != "" && secSince >= 0 && secSince < 120 {
		detail = "Last activity (" + formatSecAgo(secSince) + "): " + lastMsg
	} else if secSince >= 120 && lastMsg != "" {
		detail = "No recent progress for " + formatSecAgo(secSince) + ". Last: " + lastMsg
	}

	// Download-first IBD: headline body download while headers still lead; connect after bodies land.
	if in.HeaderTip <= in.ContiguousBodies+blockDownloadWindow && in.ConnectLag > 256 && in.ChainActiveHeight >= 0 {
		head := fmt.Sprintf("Connecting stored blocks (%d ahead of chainActive)", in.ConnectLag)
		var parts []string
		parts = append(parts, fmt.Sprintf("chainActive through %d", in.ChainActiveHeight))
		if in.ContiguousBodies >= 0 {
			parts = append(parts, fmt.Sprintf("stored through %d", in.ContiguousBodies))
		}
		if in.ConnectBlocksPerMinute > 0 {
			parts = append(parts, fmt.Sprintf("%.1f connect/min", in.ConnectBlocksPerMinute))
		} else if in.BlocksPerMinute > 0 {
			parts = append(parts, fmt.Sprintf("%.1f download/min", in.BlocksPerMinute))
		}
		return head, joinParts(detail, stringsJoin(parts, " · "))
	}
	if in.LowestMissing >= 0 && in.HeaderTip > in.ContiguousBodies {
		head := fmt.Sprintf("Downloading block bodies from height %d", in.LowestMissing)
		var parts []string
		if in.HeaderTip > 0 {
			parts = append(parts, fmt.Sprintf("headers through %d", in.HeaderTip))
		}
		if in.ChainActiveHeight >= 0 {
			parts = append(parts, fmt.Sprintf("chainActive through %d", in.ChainActiveHeight))
		}
		if in.ContiguousBodies >= 0 {
			parts = append(parts, fmt.Sprintf("stored through %d", in.ContiguousBodies))
		}
		if in.BlocksPerMinute > 0 {
			parts = append(parts, fmt.Sprintf("%.1f blocks/min", in.BlocksPerMinute))
		} else if lastFetch != "" {
			parts = append(parts, lastFetch)
		}
		if in.InFlightBatches > 0 {
			parts = append(parts, fmt.Sprintf("%d batch(es) in flight", in.InFlightBatches))
		}
		if len(parts) > 0 {
			detail = joinParts(detail, stringsJoin(parts, " · "))
		}
		return head, detail
	}

	if !in.BodyIBDHeaderPaused && (in.HeaderCatchUpPending || in.HeaderRecoveryRunning || in.DedicatedHeaderRunning) {
		head := "Catching up headers"
		if in.PeerStartHeight > 0 && int64(in.PeerStartHeight) > in.HeaderTip+headerCatchUpPeerLead {
			head = fmt.Sprintf("Catching up headers (%d / ~%d)", in.HeaderTip, in.PeerStartHeight)
		}
		var parts []string
		if in.DedicatedHeaderRunning && dedAddr != "" {
			parts = append(parts, "dedicated peer "+dedAddr)
			if dedDetail != "" {
				parts = append(parts, dedDetail)
			}
		}
		if in.HeaderRecoveryRunning {
			if recPass > 0 {
				parts = append(parts, fmt.Sprintf("background header sync pass %d", recPass))
			} else {
				parts = append(parts, "background header sync active")
			}
			if recDetail != "" {
				parts = append(parts, recDetail)
			}
			if kickSupp > 0 {
				parts = append(parts, fmt.Sprintf("watchdog queued %d× (one recovery at a time)", kickSupp))
			}
		}
		if in.HeaderRecoveryHint != "" {
			parts = append(parts, in.HeaderRecoveryHint)
		}
		return head, joinParts(detail, stringsJoin(parts, " · "))
	}

	if in.HeaderTip > 0 && in.ContiguousBodies >= 0 && in.HeaderTip <= in.ContiguousBodies+1 {
		return "Synced through local header tip", detail
	}
	return "Running", detail
}

func syncActivityTasks(in SyncActivityInput, recPass int, recDetail string, recSince time.Time, dedAddr, dedDetail string, lastFetch string, lastFetchAt time.Time, kickSupp int) []map[string]string {
	var tasks []map[string]string
	tasks = append(tasks, map[string]string{
		"name":   "p2p",
		"state":  syncActivityP2PState(in),
		"detail": syncActivityP2PDetail(in),
	})
	if in.HeaderTip > in.ContiguousBodies+blockDownloadWindow {
		tasks = append(tasks, map[string]string{
			"name":   "connect_catchup",
			"state":  "deferred",
			"detail": "waiting until block bodies catch headers (download-first IBD)",
		})
	} else if in.ConnectLag > 256 && in.ChainActiveHeight >= 0 {
		d := fmt.Sprintf("replay stored bodies from height %d", in.ChainActiveHeight+1)
		if in.ConnectBlocksPerMinute > 0 {
			d += fmt.Sprintf("; %.1f connect/min", in.ConnectBlocksPerMinute)
		}
		tasks = append(tasks, map[string]string{"name": "connect_catchup", "state": "active", "detail": d})
	}
	if in.LowestMissing >= 0 && in.HeaderTip > in.ContiguousBodies {
		d := fmt.Sprintf("forward sync from height %d", in.LowestMissing)
		if lastFetch != "" {
			d += "; " + lastFetch
		}
		if in.InFlightBatches > 0 {
			d += fmt.Sprintf("; %d in flight", in.InFlightBatches)
		}
		tasks = append(tasks, map[string]string{"name": "block_download", "state": "active", "detail": d})
	} else {
		tasks = append(tasks, map[string]string{"name": "block_download", "state": "idle", "detail": "bodies caught up to headers"})
	}
	if in.DedicatedHeaderRunning {
		d := dedDetail
		if d == "" {
			d = "getheaders on dedicated connection"
		}
		tasks = append(tasks, map[string]string{"name": "dedicated_headers", "state": "running", "detail": dedAddr + " - " + d})
	}
	if in.HeaderRecoveryRunning {
		d := recDetail
		if d == "" {
			d = "probing peers and appending headers"
		}
		if recPass > 0 {
			d = fmt.Sprintf("pass %d: %s", recPass, d)
		}
		if !recSince.IsZero() {
			d += fmt.Sprintf(" (running %s)", formatDur(time.Since(recSince)))
		}
		if kickSupp > 0 {
			d += fmt.Sprintf("; %d redundant watchdog kick(s) ignored", kickSupp)
		}
		tasks = append(tasks, map[string]string{"name": "header_sync", "state": "running", "detail": d})
	} else if in.BodyIBDHeaderPaused {
		tasks = append(tasks, map[string]string{"name": "header_sync", "state": "paused", "detail": "header getheaders deferred while block bodies catch up (after assumevalid height on tip)"})
	} else if in.HeaderCatchUpPending && in.DedicatedHeaderRunning {
		tasks = append(tasks, map[string]string{"name": "header_sync", "state": "standby", "detail": "dedicated peer owns headers; background sync only if stalled"})
	} else if in.HeaderCatchUpPending {
		tasks = append(tasks, map[string]string{"name": "header_sync", "state": "pending", "detail": "will start when dedicated peer unavailable or stalled"})
	} else {
		tasks = append(tasks, map[string]string{"name": "header_sync", "state": "idle", "detail": "headers caught up to peer height"})
	}
	return tasks
}

func syncActivityP2PState(in SyncActivityInput) string {
	if in.PeerDialing && in.ConnectionsOutbound == 0 && in.BlockAssistConnections == 0 && !in.DedicatedHeaderRunning {
		return "connecting"
	}
	if in.ConnectionsOutbound == 0 && in.PrimaryPeer == "" && in.BlockAssistConnections == 0 && !in.DedicatedHeaderRunning {
		return "waiting"
	}
	if in.ConnectionsTotal >= 2 {
		return "ok"
	}
	return "warming"
}

func syncActivityP2PDetail(in SyncActivityInput) string {
	if in.PeerDialing && in.ConnectionsOutbound == 0 && in.BlockAssistConnections == 0 && !in.DedicatedHeaderRunning {
		return "handshaking with discovered peers"
	}
	if in.BlockAssistConnections > 0 || in.DedicatedHeaderRunning {
		s := fmt.Sprintf("%d outbound sync link(s)", in.ConnectionsOutbound)
		if in.BlockAssistConnections > 0 {
			s += fmt.Sprintf("; block-assist %d", in.BlockAssistConnections)
		}
		if in.DedicatedHeaderRunning {
			s += "; dedicated headers"
		}
		if in.PrimaryPeer != "" {
			s += "; primary " + in.PrimaryPeer
		}
		return s
	}
	if in.PrimaryPeer != "" {
		s := "primary " + in.PrimaryPeer
		if in.ConnectionsTotal > 0 {
			s += fmt.Sprintf("; %d connection(s)", in.ConnectionsTotal)
		}
		if in.BlockAssistActive {
			s += fmt.Sprintf("; block-assist %d", in.BlockAssistConnections)
		}
		return s
	}
	if in.ConnectionsTotal > 0 {
		return fmt.Sprintf("%d connection(s), no primary yet", in.ConnectionsTotal)
	}
	return "no peers - check firewall and that Dogecoin Core is not using port 22556"
}

func formatSecAgo(sec int64) string {
	if sec < 60 {
		return fmt.Sprintf("%ds ago", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm ago", sec/60)
	}
	return fmt.Sprintf("%dh ago", sec/3600)
}

func formatDur(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func joinParts(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " · " + b
}

func stringsJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	s := parts[0]
	for i := 1; i < len(parts); i++ {
		s += sep + parts[i]
	}
	return s
}

// headerRecoveryRunning reports whether the background recovery goroutine flag is set.
func headerRecoveryRunning() bool {
	return headerSyncBGRecoveryRunning.Load() > 0
}

func dedicatedHeaderRunning() bool {
	return dedicatedHeaderSyncRunning.Load() > 0
}

// DedicatedHeaderPeerAddr returns the host:port of the active dedicated header-sync peer, if any.
func DedicatedHeaderPeerAddr() string {
	syncActivity.mu.Lock()
	addr := syncActivity.dedicatedAddr
	syncActivity.mu.Unlock()
	return addr
}
