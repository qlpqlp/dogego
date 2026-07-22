// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/wallet"
)

func probeInvokePsbtSkip(method string) (map[string]interface{}, bool) {
	if method == "getblockchaininfo" {
		return map[string]interface{}{"result": map[string]interface{}{
			"initialblockdownload": false,
		}}, true
	}
	if method == "walletcreatefundedpsbt" {
		return map[string]interface{}{"error": map[string]interface{}{"code": float64(-6), "message": "Insufficient funds"}}, true
	}
	if method == "listtransactions" {
		return map[string]interface{}{"result": []interface{}{}}, true
	}
	if method == "gettransaction" {
		return map[string]interface{}{"error": map[string]interface{}{"code": float64(-5), "message": "not found"}}, true
	}
	if method == "decodepsbt" {
		return map[string]interface{}{"result": map[string]interface{}{"inputs": []interface{}{}}}, true
	}
	return nil, false
}

func TestProbeCoreWalletSkippedWhenDisabled(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		if method == "getwalletinfo" {
			return map[string]interface{}{
				"error": map[string]interface{}{"code": float64(-1), "message": "built-in wallet not enabled"},
			}
		}
		return map[string]interface{}{"result": nil}
	})
	if !out.Skipped || out.Reason != "wallet not enabled" {
		t.Fatalf("expected skip: %+v", out)
	}
}

func TestProbeCoreWalletOk(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{"walletname": "main", "txcount": float64(1), "spendable_utxo_count": float64(820), "pq_commitments_enabled": true}}
		case "getbalance":
			return map[string]interface{}{"result": float64(10)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"address": "DTestAddr", "hdpath": "m/44'/3'/0'/0/0"},
			}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DTestAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{coreWalletProbeLabel}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{map[string]interface{}{"type": "mock"}}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if !out.OK || out.Skipped || out.Address != "DTestAddr" {
		t.Fatalf("expected ok: %+v", out)
	}
	if out.SpendableUtxoCount == nil || *out.SpendableUtxoCount != 820 {
		t.Fatalf("spendable_utxo_count=%v want 820", out.SpendableUtxoCount)
	}
	if out.AddressBookCount == nil || *out.AddressBookCount != 1 {
		t.Fatalf("address_book_count=%v want 1", out.AddressBookCount)
	}
	if !out.LabelRoundTripOK {
		t.Fatalf("label_roundtrip_ok=false")
	}
	if !out.LabelListOK {
		t.Fatalf("label_list_ok=false")
	}
	if !out.EnumerateSignersOK || !out.SignerConfigured {
		t.Fatalf("signer probe: configured=%v ok=%v count=%v", out.SignerConfigured, out.EnumerateSignersOK, out.SignerCount)
	}
	if !out.PqCommitmentsOK || out.PqCommitmentsEnabled == nil || !*out.PqCommitmentsEnabled {
		t.Fatalf("pq_commitments_ok=%v enabled=%v", out.PqCommitmentsOK, out.PqCommitmentsEnabled)
	}
}

func TestProbeCoreWalletAddressBookKeypoolCounts(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "txcount": float64(1),
				"pool_core_indices_stored": float64(1),
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DPoolAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{
				"isvalid": true, "iskeypool": true, "hd_keypool_core_index": float64(77),
			}}
		case "getaddressinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"address": "DPoolAddr", "iskeypool": true, "hd_keypool_core_index": float64(77),
			}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{
					"address": "DPoolAddr", "hdpath": "m/44'/3'/0'/0/0", "iskeypool": true,
					"hd_keypool_core_index": float64(77),
				},
				map[string]interface{}{"address": "DChange", "hdpath": "m/44'/3'/0'/1/0", "ischange": true},
			}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DPoolAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{coreWalletProbeLabel}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	if out.AddressBookCount == nil || *out.AddressBookCount != 2 {
		t.Fatalf("address_book_count=%v want 2", out.AddressBookCount)
	}
	if out.AddressBookKeypoolCount == nil || *out.AddressBookKeypoolCount != 1 {
		t.Fatalf("address_book_keypool_count=%v want 1", out.AddressBookKeypoolCount)
	}
	if out.AddressBookCorePoolIndicesStored == nil || *out.AddressBookCorePoolIndicesStored != 1 {
		t.Fatalf("address_book_core_pool_indices_stored=%v want 1", out.AddressBookCorePoolIndicesStored)
	}
	if !out.KeypoolValidateAddressOK || !out.KeypoolGetAddressInfoOK {
		t.Fatalf("keypool rpc ok validate=%v info=%v issues=%v", out.KeypoolValidateAddressOK, out.KeypoolGetAddressInfoOK, out.Issues)
	}
}

func TestProbeCoreWalletNodeTipRoundTrip(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{"walletname": "main"}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DRecv"}
		case "validateaddress":
			var addr string
			if len(params) > 0 {
				_ = json.Unmarshal(params[0], &addr)
			}
			res := map[string]interface{}{"isvalid": true}
			if addr == "DTipAddr" {
				res["isnodetip"] = true
			}
			return map[string]interface{}{"result": res}
		case "getaddressinfo":
			var addr string
			if len(params) > 0 {
				_ = json.Unmarshal(params[0], &addr)
			}
			res := map[string]interface{}{"address": addr}
			if addr == "DTipAddr" {
				res["isnodetip"] = true
			}
			return map[string]interface{}{"result": res}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"address": "DRecv", "hdpath": "m/44'/3'/0'/0/0"},
				map[string]interface{}{
					"address": "DTipAddr", "hdpath": "m/44'/3'/0'/2/0", "isnodetip": true,
				},
			}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	if out.AddressBookNodeTipCount == nil || *out.AddressBookNodeTipCount != 1 {
		t.Fatalf("address_book_node_tip_count=%v want 1", out.AddressBookNodeTipCount)
	}
	if !out.NodeTipValidateAddressOK || !out.NodeTipGetAddressInfoOK {
		t.Fatalf("nodetip rpc ok validate=%v info=%v issues=%v", out.NodeTipValidateAddressOK, out.NodeTipGetAddressInfoOK, out.Issues)
	}
}

func TestProbeCoreWalletKeypoolValidateMismatch(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DUsed"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true, "iskeypool": false}}
		case "getaddressinfo":
			return map[string]interface{}{"result": map[string]interface{}{"address": "DPool", "iskeypool": false}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"address": "DPool", "iskeypool": true},
			}}
		case "setlabel", "listlabels", "getaddressesbylabel", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	if out.OK {
		t.Fatalf("expected failure: %+v", out)
	}
	found := false
	for _, iss := range out.Issues {
		if iss == "keypool_validateaddress_mismatch" || iss == "keypool_getaddressinfo_mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected keypool mismatch issue, got %v", out.Issues)
	}
}

func TestProbeCoreWalletPoolCoreIndicesCountMismatch(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"pool_core_indices_stored": float64(3),
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{"address": "DPool", "iskeypool": true, "hd_keypool_core_index": float64(1)},
			}}
		case "getaddressinfo":
			return map[string]interface{}{"result": map[string]interface{}{"address": "DPool", "iskeypool": true}}
		case "setlabel", "listlabels", "getaddressesbylabel", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	found := false
	for _, iss := range out.Issues {
		if iss == "pool_core_indices_count_mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pool_core_indices_count_mismatch, got %v", out.Issues)
	}
}

func TestProbeCoreWalletListLabelsMismatch(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{map[string]interface{}{"address": "DTestAddr"}}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DTestAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{"other-label"}}
		default:
			return map[string]interface{}{"result": nil}
		}
	})
	if out.OK || len(out.Issues) != 1 || out.Issues[0] != "listlabels_roundtrip_mismatch" {
		t.Fatalf("expected listlabels mismatch: %+v", out)
	}
}

func TestProbeCoreWalletLabelRoundTripMismatch(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{map[string]interface{}{"address": "DTestAddr"}}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": []interface{}{"DOther"}}
		default:
			return map[string]interface{}{"result": nil}
		}
	})
	if out.OK || len(out.Issues) != 1 || out.Issues[0] != "setlabel_roundtrip_mismatch" {
		t.Fatalf("expected roundtrip mismatch: %+v", out)
	}
}

func TestProbeCoreWalletCountsTypedAddressBookRows(t *testing.T) {
	type addressRow struct {
		Address string
		HDPath  string
	}
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []addressRow{
				{Address: "DTestAddr", HDPath: "m/44'/3'/0'/0/0"},
				{Address: "DChange", HDPath: "m/44'/3'/0'/1/0"},
			}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DTestAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{coreWalletProbeLabel}}
		default:
			return map[string]interface{}{"result": nil}
		}
	})
	if out.AddressBookCount == nil || *out.AddressBookCount != 2 {
		t.Fatalf("address_book_count=%v want 2", out.AddressBookCount)
	}
}

func TestProbeCoreWalletInvalidAddress(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": false}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		default:
			return map[string]interface{}{"result": nil}
		}
	})
	if out.OK || len(out.Issues) != 1 || out.Issues[0] != "validateaddress_invalid" {
		t.Fatalf("expected invalid address issue: %+v", out)
	}
}

func TestProbeCoreWalletPsbtRoundTrip(t *testing.T) {
	psbtB64 := "cHNidP8BAHECAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAA"
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "format": "hd", "keypoolsize": float64(99),
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(10)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "walletcreatefundedpsbt":
			return map[string]interface{}{"result": map[string]interface{}{"psbt": psbtB64, "fee": float64(0.001)}}
		case "decodepsbt":
			return map[string]interface{}{"result": map[string]interface{}{
				"inputs": []interface{}{
					map[string]interface{}{"bip32_derivs": []interface{}{map[string]interface{}{"pubkey": "02aa"}}},
				},
			}}
		case "walletprocesspsbt":
			return map[string]interface{}{"result": map[string]interface{}{"complete": true, "psbt": psbtB64}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if !out.PsbtCreateFundedOK || !out.PsbtBIP32DerivOK || !out.PsbtProcessComplete || !out.PsbtRoundTripOK {
		t.Fatalf("psbt probe: create=%v bip32=%v process=%v roundtrip=%v", out.PsbtCreateFundedOK, out.PsbtBIP32DerivOK, out.PsbtProcessComplete, out.PsbtRoundTripOK)
	}
	if !out.KeypoolTopupOK || out.KeypoolSizeAfter == nil || *out.KeypoolSizeAfter != 99 {
		t.Fatalf("keypool topup: ok=%v size=%v", out.KeypoolTopupOK, out.KeypoolSizeAfter)
	}
	if out.HardwarePsbtHint != "" {
		t.Fatalf("no signer_cmd: expect empty hardware hint, got %q", out.HardwarePsbtHint)
	}
}

func TestProbeCoreWalletHardwarePsbtHintRoundTripOK(t *testing.T) {
	psbtB64 := "cHNidP8BAHECAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAA"
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "format": "hd", "keypoolsize": float64(99),
				"signer_cmd_configured": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(10)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "walletcreatefundedpsbt":
			return map[string]interface{}{"result": map[string]interface{}{"psbt": psbtB64, "fee": float64(0.001)}}
		case "decodepsbt":
			return map[string]interface{}{"result": map[string]interface{}{
				"inputs": []interface{}{
					map[string]interface{}{"bip32_derivs": []interface{}{map[string]interface{}{"pubkey": "02aa"}}},
				},
			}}
		case "walletprocesspsbt":
			return map[string]interface{}{"result": map[string]interface{}{"complete": true, "psbt": psbtB64}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{map[string]interface{}{"type": "mock"}}}
		case "setlabel", "getaddressesbylabel", "listlabels":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if !out.PsbtRoundTripOK || !out.SignerCmdConfigured {
		t.Fatalf("roundtrip=%v signer_cmd=%v", out.PsbtRoundTripOK, out.SignerCmdConfigured)
	}
	if !strings.Contains(out.HardwarePsbtHint, "HWI signpsbt not exercised") {
		t.Fatalf("hardware_psbt_hint=%q", out.HardwarePsbtHint)
	}
}

func TestProbeCoreWalletHardwarePsbtHint(t *testing.T) {
	psbtB64 := "cHNidP8BAHECAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAA"
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "signer_cmd_configured": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(10)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DTestAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "walletcreatefundedpsbt":
			return map[string]interface{}{"result": map[string]interface{}{"psbt": psbtB64, "fee": float64(0.001)}}
		case "decodepsbt":
			return map[string]interface{}{"result": map[string]interface{}{
				"inputs": []interface{}{
					map[string]interface{}{"bip32_derivs": []interface{}{map[string]interface{}{"pubkey": "02aa"}}},
				},
			}}
		case "walletprocesspsbt":
			return map[string]interface{}{"result": map[string]interface{}{"complete": false, "psbt": psbtB64}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if !out.PsbtCreateFundedOK || out.PsbtRoundTripOK {
		t.Fatalf("create=%v roundtrip=%v", out.PsbtCreateFundedOK, out.PsbtRoundTripOK)
	}
	if !strings.Contains(out.HardwarePsbtHint, "HWI device") {
		t.Fatalf("hardware_psbt_hint=%q", out.HardwarePsbtHint)
	}
}

func TestPsbtHasBIP32Derivations(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method != "decodepsbt" {
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected"}}
		}
		return map[string]interface{}{"result": map[string]interface{}{
			"outputs": []interface{}{
				map[string]interface{}{"bip32_derivs": []interface{}{map[string]interface{}{}}},
			},
		}}
	}
	if !psbtHasBIP32Derivations(invoke, "cHNidP8=") {
		t.Fatal("expected derivs")
	}
}

func TestProbeCoreWalletTxMetadata(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{"walletname": "main", "format": "hd", "keypoolsize": float64(99)}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		case "listtransactions":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{
					"txid": "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
					"category": "send", "confirmations": float64(10),
				},
			}}
		case "gettransaction":
			return map[string]interface{}{"result": map[string]interface{}{
				"hex": "deadbeef", "fee": float64(-0.001),
			}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"result": nil}
		}
	})
	if !out.WalletTxHexOK || !out.WalletTxFeeOK {
		t.Fatalf("hex=%v fee=%v", out.WalletTxHexOK, out.WalletTxFeeOK)
	}
	if !out.WalletListTransactionsOK || out.WalletListTransactionsMs < 0 {
		t.Fatalf("latency ok=%v ms=%d", out.WalletListTransactionsOK, out.WalletListTransactionsMs)
	}
}

func TestProbeCoreWalletKeypoolBelowThreshold(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "format": "hd", "keypoolsize": float64(10),
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"result": nil}
		}
	})
	if out.KeypoolTopupOK {
		t.Fatal("expected keypool below threshold")
	}
	found := false
	for _, w := range out.Warnings {
		if w == "keypool_below_threshold" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings=%v", out.Warnings)
	}
}

func TestProbeCoreWalletSignerCmdConfiguredNoDevices(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "signer_cmd_configured": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"result": nil}
		}
	})
	if !out.SignerCmdConfigured || out.SignerConfigured {
		t.Fatalf("signer cmd=%v device=%v", out.SignerCmdConfigured, out.SignerConfigured)
	}
	found := false
	for _, n := range out.Notes {
		if n == "signer_cmd configured but enumeratesigners returned 0 devices" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected signer note, got %v", out.Notes)
	}
}

func TestProbeCoreWalletHintMentionsRescanMetadata(t *testing.T) {
	out := ProbeCoreWallet(nil)
	if !strings.Contains(out.Hint, "wallet_index_height") || !strings.Contains(out.Hint, "/api/wallet/rescan") {
		t.Fatalf("hint=%q", out.Hint)
	}
}

func TestProbeCoreWalletRescanMetadata(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname":          "main",
				"wallet_index_height": float64(100),
				"chain_active_height": float64(200),
				"needs_rescan":        true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if out.NeedsRescan == nil || !*out.NeedsRescan {
		t.Fatalf("needs_rescan=%v", out.NeedsRescan)
	}
	if out.WalletIndexHeight == nil || *out.WalletIndexHeight != 100 {
		t.Fatalf("wallet_index_height=%v want 100", out.WalletIndexHeight)
	}
	if out.ChainActiveHeight == nil || *out.ChainActiveHeight != 200 {
		t.Fatalf("chain_active_height=%v want 200", out.ChainActiveHeight)
	}
	foundNote := false
	for _, n := range out.Notes {
		if strings.Contains(n, "wallet_index_lags_chain") {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreWalletScanIndexOK(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname":                   "main",
				"wallet_index_height":          float64(200),
				"chain_active_height":          float64(200),
				"dogego_wallet_scan_index_ok":  true,
				"dogego_wallet_history_fast_path": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if out.WalletScanIndexOK == nil || !*out.WalletScanIndexOK {
		t.Fatalf("wallet_scan_index_ok=%v", out.WalletScanIndexOK)
	}
	found := false
	for _, n := range out.Notes {
		if strings.Contains(n, "wallet_scan_index_ok") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreWalletHistoryFastPathPartial(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"wallet_index_height":             float64(100),
				"chain_active_height":             float64(400),
				"needs_rescan":                    true,
				"dogego_wallet_history_fast_path": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if out.WalletHistoryFastPath == nil || !*out.WalletHistoryFastPath {
		t.Fatalf("wallet_history_fast_path=%v", out.WalletHistoryFastPath)
	}
	found := false
	for _, n := range out.Notes {
		if strings.Contains(n, "wallet_history_fast_path") {
			found = true
		}
		if strings.Contains(n, "walk all UTXOs") {
			t.Fatalf("misleading note for fast path: %q", n)
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreWalletScanBuildingIndex(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"scanning":                                    map[string]interface{}{"duration": 0},
				"dogego_wallet_listtransactions_utxo_walk":    true,
				"dogego_wallet_listtransactions_scan_pending": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if !out.WalletScanning {
		t.Fatalf("wallet_scanning=%v want true", out.WalletScanning)
	}
	if out.WalletListTransactionsScanPending == nil || !*out.WalletListTransactionsScanPending {
		t.Fatalf("wallet_listtransactions_scan_pending=%v", out.WalletListTransactionsScanPending)
	}
	found := false
	for _, n := range out.Notes {
		if strings.Contains(n, "wallet_scan_building_index") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreWalletHistoryDeferredSkipsListtransactions(t *testing.T) {
	listCalled := false
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getblockchaininfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"initialblockdownload": false,
				"dogego_connect_lag":   float64(128),
			}}
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main",
				"format":     "hd",
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "listtransactions":
			listCalled = true
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if out.WalletHistoryDeferReason != "connect_lag" {
		t.Fatalf("defer_reason=%q want connect_lag", out.WalletHistoryDeferReason)
	}
	if !out.WalletHistoryDeferred {
		t.Fatal("wallet_history_deferred want true")
	}
	if listCalled {
		t.Fatal("listtransactions should be skipped when history deferred")
	}
	if out.WalletListTransactionsMs != 0 {
		t.Fatalf("wallet_listtransactions_ms=%d want 0", out.WalletListTransactionsMs)
	}
	foundDefer := false
	foundSkip := false
	for _, n := range out.Notes {
		if strings.Contains(n, "wallet_history_deferred_connect_lag") {
			foundDefer = true
		}
		if strings.Contains(n, "listtransactions_skipped") {
			foundSkip = true
		}
	}
	if !foundDefer || !foundSkip {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreWalletListtransactionsUtxoWalk(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname":                              "main",
				"spendable_utxo_count":                    float64(200),
				"dogego_wallet_listtransactions_utxo_walk": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel", "getaddressesbylabel", "listlabels", "enumeratesigners":
			return map[string]interface{}{"result": nil}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"error": map[string]interface{}{"message": "unexpected " + method}}
		}
	})
	if out.WalletListTransactionsUtxoWalk == nil || !*out.WalletListTransactionsUtxoWalk {
		t.Fatalf("wallet_listtransactions_utxo_walk=%v", out.WalletListTransactionsUtxoWalk)
	}
	found := false
	for _, n := range out.Notes {
		if strings.Contains(n, "wallet_listtransactions_utxo_walk") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreWalletWalletDatProbe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := os.WriteFile(path, []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{"walletname": "t"}}
		case "getbalance":
			return map[string]interface{}{"result": 0.0}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{coreWalletProbeLabel}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		case "dogego_probewalletdat":
			return map[string]interface{}{
				"result": map[string]interface{}{
					"is_bdb": true, "key_count": 1, "pool_count": 1, "pool_pubkeys": 1,
					"pool_keys_matched": 1, "can_import": true,
					"hint": "Core keypool entries detected",
				},
			}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	if out.WalletDatPath != path || out.WalletDatProbe == nil {
		t.Fatalf("wallet.dat probe missing: %+v", out)
	}
	if out.PoolReplayScanCap != wallet.PoolReplayScanCap {
		t.Fatalf("pool_replay_scan_cap=%d want %d", out.PoolReplayScanCap, wallet.PoolReplayScanCap)
	}
	if !out.OK {
		t.Fatalf("expected ok: issues=%v warnings=%v", out.Issues, out.Warnings)
	}
}

func TestProbeCoreWalletAddressBookPendingNote(t *testing.T) {
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{}}
		case "getbalance":
			return map[string]interface{}{"result": float64(0)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DNewAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DNewAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{coreWalletProbeLabel}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	if !out.OK {
		t.Fatalf("expected ok: issues=%v warnings=%v", out.Issues, out.Warnings)
	}
	found := false
	for _, n := range out.Notes {
		if n == "address_book_pending_new_address" {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes=%v warnings=%v", out.Notes, out.Warnings)
	}
	for _, w := range out.Warnings {
		if w == "dogego_listwalletaddresses_empty" {
			t.Fatal("expected empty book as note not warning when new address exists")
		}
	}
}

func TestProbeCoreWalletWalletDatPoolUnmatchedWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := os.WriteFile(path, []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOGEGO_WALLET_DAT", path)
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{"walletname": "t"}}
		case "getbalance":
			return map[string]interface{}{"result": 0.0}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "setlabel":
			return map[string]interface{}{"result": nil}
		case "getaddressesbylabel":
			return map[string]interface{}{"result": map[string]interface{}{
				"DAddr": map[string]interface{}{"purpose": "receive"},
			}}
		case "listlabels":
			return map[string]interface{}{"result": []interface{}{coreWalletProbeLabel}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		case "dogego_probewalletdat":
			return map[string]interface{}{
				"result": map[string]interface{}{
					"is_bdb": true, "key_count": 1, "pool_count": 2, "pool_pubkeys": 2,
					"pool_keys_matched": 1, "pool_keys_unmatched": 1, "can_import": true,
					"pool_indices_replayed": false,
					"pool_unmatched_hint":   "1 Core pool pubkey(s) have no spend key in wallet.dat",
					"hint":                  "Core keypool entries detected",
				},
			}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			t.Fatalf("method %q", method)
			return nil
		}
	})
	if out.PoolKeysUnmatched == nil || *out.PoolKeysUnmatched != 1 {
		t.Fatalf("pool_keys_unmatched=%v", out.PoolKeysUnmatched)
	}
	found := false
	for _, n := range out.Notes {
		if n == "walletdat_probe_pool_keys_unmatched" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("notes=%v", out.Notes)
	}
	if out.PoolIndicesReplayed == nil || *out.PoolIndicesReplayed {
		t.Fatalf("pool_indices_replayed=%v", out.PoolIndicesReplayed)
	}
	if !strings.Contains(out.PoolUnmatchedHint, "no spend key") {
		t.Fatalf("pool_unmatched_hint=%q", out.PoolUnmatchedHint)
	}
}
