// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultRPCTimeout = 12 * time.Second

// Client talks JSON-RPC to a DogeOS EVM endpoint.
type Client struct {
	RPCURL string
	HTTP   *http.Client
}

func NewClient(rpcURL string) *Client {
	return &Client{
		RPCURL: strings.TrimSpace(rpcURL),
		HTTP:   &http.Client{Timeout: defaultRPCTimeout},
	}
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "rpc error"
	}
	return fmt.Sprintf("rpc %d: %s", e.Code, e.Message)
}

func (c *Client) Call(ctx context.Context, method string, params []interface{}) (json.RawMessage, time.Duration, error) {
	if c == nil || strings.TrimSpace(c.RPCURL) == "" {
		return nil, 0, fmt.Errorf("rpc url required")
	}
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RPCURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := c.HTTP.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, latency, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, latency, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var out rpcResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, latency, fmt.Errorf("decode rpc: %w", err)
	}
	if out.Error != nil {
		return nil, latency, out.Error
	}
	return out.Result, latency, nil
}

func (c *Client) CallDecoded(ctx context.Context, method string, params []interface{}, dest interface{}) (time.Duration, error) {
	raw, lat, err := c.Call(ctx, method, params)
	if err != nil {
		return lat, err
	}
	if dest == nil {
		return lat, nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return lat, err
	}
	return lat, nil
}

// ProbeResult is a health snapshot of the DogeOS RPC.
type ProbeResult struct {
	OK           bool   `json:"ok"`
	RPCURL       string `json:"rpc_url"`
	LatencyMS    int64  `json:"latency_ms"`
	ChainIDHex   string `json:"chain_id_hex,omitempty"`
	ChainID      int64  `json:"chain_id,omitempty"`
	BlockNumber  int64  `json:"block_number,omitempty"`
	BlockHex     string `json:"block_hex,omitempty"`
	GasPriceWei  string `json:"gas_price_wei,omitempty"`
	GasPriceGwei string `json:"gas_price_gwei,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
	Syncing      bool   `json:"syncing"`
	Error        string `json:"error,omitempty"`
	ProbedAt     int64  `json:"probed_at"`
	ExpectedChainID int64 `json:"expected_chain_id,omitempty"`
	ChainIDMatch bool   `json:"chain_id_match"`
}

func (c *Client) Probe(ctx context.Context, expectedChainID int64) ProbeResult {
	out := ProbeResult{
		RPCURL:          c.RPCURL,
		ProbedAt:        time.Now().Unix(),
		ExpectedChainID: expectedChainID,
	}
	var chainHex string
	lat, err := c.CallDecoded(ctx, "eth_chainId", nil, &chainHex)
	out.LatencyMS = lat.Milliseconds()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.ChainIDHex = chainHex
	out.ChainID = parseHexInt64(chainHex)

	var blockHex string
	if _, err := c.CallDecoded(ctx, "eth_blockNumber", nil, &blockHex); err == nil {
		out.BlockHex = blockHex
		out.BlockNumber = parseHexInt64(blockHex)
	}

	var gasHex string
	if _, err := c.CallDecoded(ctx, "eth_gasPrice", nil, &gasHex); err == nil {
		out.GasPriceWei = parseHexBig(gasHex)
		out.GasPriceGwei = weiToGwei(out.GasPriceWei)
	}

	var ver string
	if _, err := c.CallDecoded(ctx, "web3_clientVersion", nil, &ver); err == nil {
		out.ClientVersion = ver
	}

	var syncRaw json.RawMessage
	if _, err := c.CallDecoded(ctx, "eth_syncing", nil, &syncRaw); err == nil {
		s := strings.TrimSpace(string(syncRaw))
		out.Syncing = s != "false" && s != "" && s != "null"
	}

	out.OK = out.ChainID > 0 && out.BlockNumber >= 0
	if expectedChainID > 0 {
		out.ChainIDMatch = out.ChainID == expectedChainID
		if !out.ChainIDMatch {
			out.OK = false
			if out.Error == "" {
				out.Error = fmt.Sprintf("chain id mismatch: got %d want %d", out.ChainID, expectedChainID)
			}
		}
	} else {
		out.ChainIDMatch = true
	}
	return out
}

func (c *Client) GetBalance(ctx context.Context, address string) (string, string, error) {
	addr := normalizeAddress(address)
	if addr == "" {
		return "", "", fmt.Errorf("address required")
	}
	var hexBal string
	if _, err := c.CallDecoded(ctx, "eth_getBalance", []interface{}{addr, "latest"}, &hexBal); err != nil {
		return "", "", err
	}
	wei := parseHexBig(hexBal)
	return wei, weiToDOGE(wei), nil
}

func (c *Client) GetCode(ctx context.Context, address string) (string, bool, error) {
	addr := normalizeAddress(address)
	if addr == "" {
		return "", false, fmt.Errorf("address required")
	}
	var code string
	if _, err := c.CallDecoded(ctx, "eth_getCode", []interface{}{addr, "latest"}, &code); err != nil {
		return "", false, err
	}
	code = strings.TrimSpace(code)
	isContract := code != "" && code != "0x" && code != "0X"
	return code, isContract, nil
}

func (c *Client) GetTransactionReceipt(ctx context.Context, txHash string) (map[string]interface{}, error) {
	h := strings.TrimSpace(txHash)
	if h == "" {
		return nil, fmt.Errorf("tx hash required")
	}
	if !strings.HasPrefix(h, "0x") && !strings.HasPrefix(h, "0X") {
		h = "0x" + h
	}
	raw, _, err := c.Call(ctx, "eth_getTransactionReceipt", []interface{}{h})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Client) GetBlockByNumber(ctx context.Context, number string, fullTx bool) (map[string]interface{}, error) {
	n := strings.TrimSpace(number)
	if n == "" {
		n = "latest"
	}
	if n != "latest" && n != "earliest" && n != "pending" && !strings.HasPrefix(n, "0x") {
		if v, err := strconv.ParseInt(n, 10, 64); err == nil {
			n = fmt.Sprintf("0x%x", v)
		}
	}
	raw, _, err := c.Call(ctx, "eth_getBlockByNumber", []interface{}{n, fullTx})
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseHexInt64(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseHexBig(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return "0"
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, 16); !ok {
		return "0"
	}
	return n.String()
}

func weiToGwei(wei string) string {
	n, ok := new(big.Int).SetString(wei, 10)
	if !ok {
		return "0"
	}
	f := new(big.Float).SetInt(n)
	f.Quo(f, big.NewFloat(1e9))
	return f.Text('f', 4)
}

func weiToDOGE(wei string) string {
	n, ok := new(big.Int).SetString(wei, 10)
	if !ok {
		return "0"
	}
	f := new(big.Float).SetInt(n)
	f.Quo(f, big.NewFloat(1e18))
	return f.Text('f', 6)
}

func normalizeAddress(addr string) string {
	a := strings.TrimSpace(addr)
	if a == "" {
		return ""
	}
	if !strings.HasPrefix(a, "0x") && !strings.HasPrefix(a, "0X") {
		a = "0x" + a
	}
	if len(a) != 42 {
		return ""
	}
	for _, c := range a[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return strings.ToLower(a)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
