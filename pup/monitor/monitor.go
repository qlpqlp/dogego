// DogeGo Dogebox metrics reporter — polls GET /api/summary and POSTs to /dbx/metrics
// (same contract as the CORE pup monitor).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type summaryAPI struct {
	Chain               string  `json:"chain"`
	Network             string  `json:"network"`
	NodeMode            string  `json:"node_mode"`
	TipHeight           int64   `json:"tip_height"`
	ChainActiveHeight   int64   `json:"chain_active_height"`
	HeaderCount         int64   `json:"header_count"`
	BestHash            string  `json:"best_hash"`
	VerificationProg    float64 `json:"verification_progress"`
	SyncPct             float64 `json:"sync_pct"`
	IBDActive           bool    `json:"ibd_active"`
	InitialBlockDL      bool    `json:"initialblockdownload"`
	ConnectionsIn       int     `json:"connections_in"`
	ConnectionsOut      int     `json:"connections_out"`
	MempoolTxs          int     `json:"mempool_txs"`
	BlocksBehind        int64   `json:"blocks_behind_headers"`
	BlocksPerMinute     float64 `json:"blocks_per_minute"`
	SyncStatusLine      string  `json:"sync_status_line"`
	SyncHealth          string  `json:"dogego_sync_health"`
	SyncPhase           string  `json:"sync_phase"`
	PrimaryPeer         string  `json:"primary_peer"`
	Peer                string  `json:"peer"`
	RawBlocks           int     `json:"raw_blocks"`
	ContiguousRawHeight int64   `json:"contiguous_raw_height"`
	SizeOnDisk          int64   `json:"size_on_disk"`
	DataDirBytes        int64   `json:"datadir_bytes"`
	ChainBytesTotal     int64   `json:"chain_bytes_total"`
}

func webUIBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("DOGEGO_METRICS_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	ip := strings.TrimSpace(os.Getenv("DBX_PUP_IP"))
	if ip == "" {
		ip = "127.0.0.1"
	}
	port := strings.TrimSpace(os.Getenv("DOGEGO_WEBUI_PORT"))
	if port == "" {
		port = "2013"
	}
	return fmt.Sprintf("http://%s:%s", ip, port)
}

func fetchSummary(client *http.Client) (summaryAPI, error) {
	url := webUIBaseURL() + "/api/summary"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return summaryAPI{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return summaryAPI{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return summaryAPI{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return summaryAPI{}, fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, truncate(string(body), 200))
	}
	var s summaryAPI
	if err := json.Unmarshal(body, &s); err != nil {
		return summaryAPI{}, err
	}
	return s, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func bytesToHuman(n int64) string {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)
	if n <= 0 {
		return "n/a"
	}
	if n < GB {
		return fmt.Sprintf("%.2f MB", float64(n)/MB)
	}
	return fmt.Sprintf("%.2f GB", float64(n)/GB)
}

func shortHash(h string) string {
	h = strings.TrimSpace(h)
	if len(h) <= 18 {
		return h
	}
	return h[:10] + "…" + h[len(h)-6:]
}

func submitMetrics(client *http.Client, s summaryAPI) {
	host := strings.TrimSpace(os.Getenv("DBX_HOST"))
	port := strings.TrimSpace(os.Getenv("DBX_PORT"))
	if host == "" || port == "" {
		log.Printf("DBX_HOST/DBX_PORT unset; skip metrics submit")
		return
	}

	chain := s.Chain
	if chain == "" {
		chain = s.Network
	}
	ibd := s.IBDActive || s.InitialBlockDL
	peers := s.ConnectionsIn + s.ConnectionsOut
	peerLabel := strings.TrimSpace(s.PrimaryPeer)
	if peerLabel == "" {
		peerLabel = strings.TrimSpace(s.Peer)
	}
	if peerLabel == "" {
		peerLabel = "none"
	}

	disk := s.ChainBytesTotal
	if disk <= 0 {
		disk = s.SizeOnDisk
	}
	if disk <= 0 {
		disk = s.DataDirBytes
	}

	headers := s.TipHeight
	if s.HeaderCount > headers {
		headers = s.HeaderCount
	}
	blocks := s.ChainActiveHeight
	if blocks < 0 {
		blocks = s.TipHeight
	}

	payload := map[string]interface{}{
		"chain":                  map[string]interface{}{"value": chain},
		"network":                map[string]interface{}{"value": s.Network},
		"node_mode":              map[string]interface{}{"value": s.NodeMode},
		"blocks":                 map[string]interface{}{"value": blocks},
		"headers":                map[string]interface{}{"value": headers},
		"peers":                  map[string]interface{}{"value": peers},
		"connections_in":         map[string]interface{}{"value": s.ConnectionsIn},
		"connections_out":        map[string]interface{}{"value": s.ConnectionsOut},
		"mempool_txs":            map[string]interface{}{"value": s.MempoolTxs},
		"blocks_behind":          map[string]interface{}{"value": s.BlocksBehind},
		"blocks_per_minute":      map[string]interface{}{"value": s.BlocksPerMinute},
		"verification_progress":  map[string]interface{}{"value": fmt.Sprintf("%.2f%%", s.VerificationProg*100)},
		"initial_block_download": map[string]interface{}{"value": yesNo(ibd)},
		"sync_status":            map[string]interface{}{"value": s.SyncStatusLine},
		"sync_health":            map[string]interface{}{"value": s.SyncHealth},
		"sync_phase":             map[string]interface{}{"value": s.SyncPhase},
		"primary_peer":           map[string]interface{}{"value": peerLabel},
		"best_hash":              map[string]interface{}{"value": shortHash(s.BestHash)},
		"raw_blocks":             map[string]interface{}{"value": s.RawBlocks},
		"contiguous_raw_height":  map[string]interface{}{"value": s.ContiguousRawHeight},
		"chain_size_human":       map[string]interface{}{"value": bytesToHuman(disk)},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("marshal metrics: %v", err)
		return
	}

	url := fmt.Sprintf("http://%s:%s/dbx/metrics", host, port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("metrics request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("metrics post: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("metrics post HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
		return
	}
	log.Printf("metrics ok: chain=%s blocks=%d headers=%d peers=%d sync=%.1f%% ibd=%v",
		chain, blocks, headers, peers, s.VerificationProg*100, ibd)
}

func main() {
	log.Println("DogeGo monitor: waiting for web UI…")
	time.Sleep(12 * time.Second)

	client := &http.Client{Timeout: 10 * time.Second}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		s, err := fetchSummary(client)
		if err != nil {
			log.Printf("summary: %v", err)
		} else {
			submitMetrics(client, s)
		}
		<-ticker.C
	}
}
