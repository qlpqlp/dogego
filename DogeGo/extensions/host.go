// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
)

// ChainReader is the read-only chain surface exposed to extensions (no wallet).
type ChainReader interface {
	Network() string
	TipHeight() (int64, error)
	GetRawBlockByHeight(height int64) ([]byte, error)
	LookupTxHex(txid string) (hex string, height int64, ok bool)
	// BlockHashAtHeight returns the display block hash at height.
	BlockHashAtHeight(height int64) (string, error)
	// ConfirmedTxInBlock reports whether txid is in blockHash and returns its index.
	ConfirmedTxInBlock(blockHash, txid string) (txIndex uint32, ok bool)
}

// Host is the sandbox API for enabled extensions. Direct wallet/key access is never exposed;
// optional wallet operations use WalletRPCHost with the wallet_rpc manifest permission.
type Host interface {
	ChainReader
	DataDir() string
	ExtensionDataDir(id string) (string, error)
	Log(line string)
}

// Extension is a loadable DogeGo extension module.
type Extension interface {
	Manifest() Manifest
	OnEnable(ctx context.Context, host Host) error
	OnDisable() error
	HandleRPC(method string, params []json.RawMessage, host Host) (interface{}, error)
}

// BuiltinFactory builds a builtin extension instance from manifest metadata.
type BuiltinFactory func(m Manifest) (Extension, error)
