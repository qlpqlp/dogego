// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package signer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Client talks to an HWI-compatible external signer process (stdin/stdout JSON lines).
type Client struct {
	argv    []string
	Timeout time.Duration // 0 uses SignTimeout
}

// ParseCommandLine splits a signer_cmd config value into argv (simple whitespace split).
func ParseCommandLine(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	return strings.Fields(cmd)
}

// New returns a client when argv is non-empty.
func New(argv []string) (*Client, error) {
	if len(argv) == 0 {
		return nil, nil
	}
	if err := ValidateCommand(argv); err != nil {
		return nil, err
	}
	out := make([]string, len(argv))
	copy(out, argv)
	return &Client{argv: out}, nil
}

// ValidateCommand checks signer_cmd argv before execution.
func ValidateCommand(argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	bin := strings.TrimSpace(argv[0])
	if bin == "" {
		return fmt.Errorf("signer: empty command")
	}
	if strings.ContainsAny(bin, ";&|$`<>\"'\n\r") {
		return fmt.Errorf("signer: invalid command path")
	}
	for _, arg := range argv[1:] {
		if strings.ContainsAny(arg, "\n\r\x00") {
			return fmt.Errorf("signer: invalid argument")
		}
	}
	return nil
}

// Available reports whether an external signer command is configured.
func (c *Client) Available() bool {
	return c != nil && len(c.argv) > 0
}

func (c *Client) callTimeout() time.Duration {
	if c != nil && c.Timeout > 0 {
		return c.Timeout
	}
	return SignTimeout
}

// Call sends one JSON request line and reads one JSON response line.
func (c *Client) Call(method string, params map[string]interface{}) (json.RawMessage, error) {
	if !c.Available() {
		return nil, fmt.Errorf("signer not configured")
	}
	req := map[string]interface{}{"method": method, "id": 1}
	if len(params) > 0 {
		req["params"] = params
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.callTimeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, c.argv[0], c.argv[1:]...)
	cmd.Stdin = bytes.NewReader(append(body, '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("signer: timeout after %s", c.callTimeout())
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("signer: %s", msg)
	}
	line := strings.TrimSpace(stdout.String())
	if line == "" {
		return nil, fmt.Errorf("signer: empty response")
	}
	var wrap struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &wrap); err != nil {
		return nil, fmt.Errorf("signer: bad json: %w", err)
	}
	if wrap.Error != nil && wrap.Error.Message != "" {
		return nil, fmt.Errorf("signer: %s", wrap.Error.Message)
	}
	if len(wrap.Result) == 0 {
		return json.RawMessage("null"), nil
	}
	return wrap.Result, nil
}

// Enumerate returns HWI device descriptors (best-effort).
func (c *Client) Enumerate() ([]map[string]interface{}, error) {
	raw, err := c.Call("enumerate", nil)
	if err != nil {
		return nil, err
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// SignPSBT asks the external signer to sign a base64 PSBT and returns the updated PSBT base64.
func (c *Client) SignPSBT(psbtB64 string) (string, error) {
	raw, err := c.Call("signpsbt", map[string]interface{}{"psbt": psbtB64})
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("signer: unexpected signpsbt result")
	}
	if psbt, ok := obj["psbt"].(string); ok && psbt != "" {
		return psbt, nil
	}
	return "", fmt.Errorf("signer: signpsbt returned no psbt")
}

// DisplayAddress returns a receive address from the signer for a descriptor (HWI displayaddress).
func (c *Client) DisplayAddress(descriptor string) (string, error) {
	raw, err := c.Call("displayaddress", map[string]interface{}{"descriptor": descriptor})
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s, nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("signer: unexpected displayaddress result")
	}
	if addr, ok := obj["address"].(string); ok {
		return addr, nil
	}
	return "", fmt.Errorf("signer: displayaddress returned no address")
}

// SignTimeout is the maximum time to wait for hardware user approval.
const SignTimeout = 120 * time.Second
