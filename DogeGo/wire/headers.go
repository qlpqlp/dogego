// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const auxpowVersionBit = 1 << 8

// DecodedHeader is one entry in a "headers" message: 80-byte header plus optional auxpow.
type DecodedHeader struct {
	Header80 []byte
	Aux      *AuxPow // non-nil iff version has auxpow bit
	// AuxWire is pre-serialized CAuxPow when Aux is nil (serving from headers_aux.bin).
	AuxWire []byte
}

// EncodeGetHeaders builds a getheaders payload (CBlockLocator + hashStop).
func EncodeGetHeaders(protocolVersion int32, locator [][32]byte, hashStop [32]byte) ([]byte, error) {
	if len(locator) > 101 {
		return nil, fmt.Errorf("locator too large %d", len(locator))
	}
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, protocolVersion); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(locator))); err != nil {
		return nil, err
	}
	for _, h := range locator {
		if _, err := b.Write(h[:]); err != nil {
			return nil, err
		}
	}
	if _, err := b.Write(hashStop[:]); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// GetHeadersMsg is a decoded getheaders P2P payload.
type GetHeadersMsg struct {
	Version  int32
	Locator  [][32]byte
	HashStop [32]byte
}

// DecodeGetHeaders parses getheaders (protocol version + block locator + hashStop).
func DecodeGetHeaders(payload []byte) (GetHeadersMsg, error) {
	var out GetHeadersMsg
	r := bytes.NewReader(payload)
	if err := binary.Read(r, binary.LittleEndian, &out.Version); err != nil {
		return out, err
	}
	n, err := ReadCompactSize(r)
	if err != nil {
		return out, err
	}
	if n > 101 {
		return out, fmt.Errorf("locator too large %d", n)
	}
	out.Locator = make([][32]byte, 0, n)
	for i := uint64(0); i < n; i++ {
		var h [32]byte
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return out, err
		}
		out.Locator = append(out.Locator, h)
	}
	if _, err := io.ReadFull(r, out.HashStop[:]); err != nil {
		return out, err
	}
	if r.Len() != 0 {
		return out, fmt.Errorf("getheaders trailing %d bytes", r.Len())
	}
	return out, nil
}

// EncodeHeadersPayload builds a headers message body (vector<CBlock> with empty vtx each).
func EncodeHeadersPayload(headers []DecodedHeader) ([]byte, error) {
	if len(headers) > 2000 {
		return nil, fmt.Errorf("too many headers %d", len(headers))
	}
	var b bytes.Buffer
	if err := WriteCompactSize(&b, uint64(len(headers))); err != nil {
		return nil, err
	}
	for i, dh := range headers {
		if len(dh.Header80) != 80 {
			return nil, fmt.Errorf("header %d: bad len %d", i, len(dh.Header80))
		}
		if _, err := b.Write(dh.Header80); err != nil {
			return nil, err
		}
		ver := binary.LittleEndian.Uint32(dh.Header80[0:4])
		if ver&auxpowVersionBit != 0 {
			switch {
			case dh.Aux != nil:
				ab, err := SerializeAuxPow(dh.Aux)
				if err != nil {
					return nil, fmt.Errorf("header %d auxpow: %w", i, err)
				}
				if _, err := b.Write(ab); err != nil {
					return nil, err
				}
			case len(dh.AuxWire) > 0:
				if _, err := b.Write(dh.AuxWire); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("header %d: auxpow version without aux data", i)
			}
		}
		if err := WriteCompactSize(&b, 0); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

// DecodeHeadersPayload parses a headers message body: vector<CBlock> with empty vtx each.
func DecodeHeadersPayload(payload []byte) ([]DecodedHeader, error) {
	r := bytes.NewReader(payload)
	nBlocks, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nBlocks > 2000 {
		return nil, fmt.Errorf("too many headers %d", nBlocks)
	}
	out := make([]DecodedHeader, 0, nBlocks)
	for i := uint64(0); i < nBlocks; i++ {
		var h80 [80]byte
		if _, err := io.ReadFull(r, h80[:]); err != nil {
			return nil, err
		}
		ver := binary.LittleEndian.Uint32(h80[0:4])
		var aux *AuxPow
		if ver&auxpowVersionBit != 0 {
			aux, err = ReadAuxPow(r)
			if err != nil {
				return nil, fmt.Errorf("header %d auxpow: %w", i, err)
			}
		}
		nTx, err := ReadCompactSize(r)
		if err != nil {
			return nil, err
		}
		if nTx != 0 {
			return nil, fmt.Errorf("header %d: expected 0 tx in headers message, got %d", i, nTx)
		}
		out = append(out, DecodedHeader{
			Header80: append([]byte(nil), h80[:]...),
			Aux:      aux,
		})
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("trailing junk %d bytes", r.Len())
	}
	return out, nil
}
