// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
)

func TestDeriveAddressesPKH(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k, _ := secp256k1.NewPrivateKey()
	wif, _ := chain.EncodeWIF(k.Serialize(), p.PrivKeyWIFVersion, true)
	addr, err := addressFromWIF("testnet", wif)
	if err != nil {
		t.Fatal(err)
	}
	desc := "pkh(" + addr + ")"
	raw, _ := json.Marshal(desc)
	addrs, code, msg := execDeriveAddresses("testnet", []json.RawMessage{raw})
	if code != 0 {
		t.Fatalf("deriveaddresses: %s", msg)
	}
	sl, ok := addrs.([]string)
	if !ok || len(sl) != 1 || sl[0] != addr {
		t.Fatalf("addrs %#v", addrs)
	}
}

func TestExtractDescriptorChecksum(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k, _ := secp256k1.NewPrivateKey()
	wif, _ := chain.EncodeWIF(k.Serialize(), p.PrivKeyWIFVersion, true)
	addr, err := addressFromWIF("testnet", wif)
	if err != nil {
		t.Fatal(err)
	}
	desc := "pkh(" + addr + ")"
	cs := descriptorChecksum(desc)
	raw, _ := json.Marshal(desc + "#" + cs)
	res, code, msg := execExtractDescriptor([]json.RawMessage{raw})
	if code != 0 {
		t.Fatalf("extractdescriptor: %s", msg)
	}
	m, _ := res.(map[string]interface{})
	if m["descriptor"] != desc {
		t.Fatalf("descriptor %v", m["descriptor"])
	}
	if m["checksum"] != cs {
		t.Fatalf("checksum %v", m["checksum"])
	}
}
