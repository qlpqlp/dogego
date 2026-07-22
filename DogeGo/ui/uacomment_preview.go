// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"dogego/chain"
	"dogego/config"
	"dogego/wallet"
)

type uacommentPreviewRequest struct {
	UAComment           string `json:"uacomment"`
	UACommentTipAddress string `json:"uacomment_tip_address"`
	UACommentUseNodeTip *bool  `json:"uacomment_use_node_tip"`
	PublishTip          bool   `json:"publish_tip"`
	DataDir             string `json:"datadir"`
	Network             string `json:"network"`
	NoWallet            bool   `json:"nowallet"`
}

func registerUACommentPreview(mux *http.ServeMux) {
	mux.HandleFunc("/api/config/uacomment-preview", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req uacommentPreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		out, err := buildUACommentPreview(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(out)
	})
}

func buildUACommentPreview(req uacommentPreviewRequest) (map[string]any, error) {
	f := config.File{
		UAComment:           strings.TrimSpace(req.UAComment),
		UACommentTipAddress: strings.TrimSpace(req.UACommentTipAddress),
		DataDir:             strings.TrimSpace(req.DataDir),
		Network:             strings.TrimSpace(req.Network),
		NoWallet:            req.NoWallet,
	}
	if req.UACommentUseNodeTip != nil {
		v := *req.UACommentUseNodeTip
		f.UACommentUseNodeTip = &v
	}
	if !req.PublishTip {
		f.UACommentTipAddress = ""
		f.UACommentUseNodeTip = nil
	} else if f.UACommentUseNodeTipEnabled() {
		if f.NoWallet {
			return nil, fmt.Errorf("node tip address requires wallet")
		}
		wpath, err := walletPathForConfig(&f)
		if err != nil {
			return nil, err
		}
		net, err := chain.ParseNetwork(strings.TrimSpace(f.Network))
		if err != nil {
			return nil, err
		}
		p, err := chain.ParamsFor(net)
		if err != nil {
			return nil, err
		}
		addr, err := wallet.PreviewNodeTipFromPath(wpath, p.PubkeyHashAddrID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "hd wallet") {
				return map[string]any{
					"effective_uacomment": f.EffectiveUAComment(),
					"subversion":          chain.BuildSubVersion(f.EffectiveUAComment()),
					"tip_address":         "",
					"publish_tip":         false,
					"tip_preview_error":   err.Error(),
				}, nil
			}
			return nil, err
		}
		f.UACommentTipAddress = addr
	} else if err := config.ValidateUACommentTip(f.UACommentTipAddress, f.Network); err != nil {
		return nil, err
	}
	comment := f.EffectiveUAComment()
	sub := chain.BuildSubVersion(comment)
	return map[string]any{
		"effective_uacomment": comment,
		"subversion":          sub,
		"tip_address":         strings.TrimSpace(f.UACommentTipAddress),
		"publish_tip":         req.PublishTip && strings.TrimSpace(f.UACommentTipAddress) != "",
	}, nil
}
