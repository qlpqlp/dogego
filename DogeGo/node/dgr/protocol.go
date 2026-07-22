// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"encoding/binary"
	"errors"
	"io"
)

const magic = "DGR1"

const (
	MsgRegister   byte = 1
	MsgRegisterOK byte = 2
	MsgPing       byte = 3
	MsgPong       byte = 4
	MsgP2PFrame   byte = 5
	MsgPeerHint   byte = 6
	MsgInvTx      byte = 7
	MsgP2PPublish byte = 8 // client → relay → Dogecoin P2P network
	MsgP2PPush    byte = 9 // relay → client (inbound fan-in)
	MsgP2PTunnel  byte = 10 // relay → client (unsolicited tunneled peer P2P frame)
)

var (
	errShortFrame = errors.New("dgr: short frame")
	errBadMagic   = errors.New("dgr: bad magic")
)

func writeFrame(w io.Writer, typ byte, payload []byte) error {
	hdr := make([]byte, 4+1+4)
	copy(hdr[:4], magic)
	hdr[4] = typ
	binary.BigEndian.PutUint32(hdr[5:9], uint32(len(payload)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func readFrame(r io.Reader) (typ byte, payload []byte, err error) {
	hdr := make([]byte, 9)
	if _, err = io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	if string(hdr[:4]) != magic {
		return 0, nil, errBadMagic
	}
	typ = hdr[4]
	n := binary.BigEndian.Uint32(hdr[5:9])
	if n == 0 {
		return typ, nil, nil
	}
	if n > 16<<20 {
		return 0, nil, errors.New("dgr: frame too large")
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return typ, payload, nil
}
