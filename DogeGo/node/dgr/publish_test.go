// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import "testing"

func TestEncodeDecodeP2PPayload(t *testing.T) {
	payload := []byte("dogecoin inv payload")
	raw, err := encodeP2PPayload("inv", payload)
	if err != nil {
		t.Fatal(err)
	}
	cmd, body, ok := decodeP2PPayload(raw)
	if !ok || cmd != "inv" || string(body) != string(payload) {
		t.Fatalf("round trip %#v %#v", cmd, body)
	}
}

func TestEncodeP2PPayloadRejectsOversize(t *testing.T) {
	_, err := encodeP2PPayload("block", make([]byte, maxP2PPublishPayload+1))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodeDecodeTunnelData(t *testing.T) {
	wireMsg := []byte{0xfa, 0xca, 0xda, 0xbe, 0x01, 0x02}
	raw, err := encodeTunnelData("127.0.0.1:44556", wireMsg)
	if err != nil {
		t.Fatal(err)
	}
	peer, body, ok := decodeTunnelData(raw)
	if !ok || peer != "127.0.0.1:44556" || string(body) != string(wireMsg) {
		t.Fatalf("round trip peer=%q body=%x", peer, body)
	}
}
