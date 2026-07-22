// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"math/rand"
	"slices"
	"testing"

	"dogego/wire"
)

func poolAddTx(t *testing.T, p *Pool, tx *wire.Tx) string {
	t.Helper()
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	return txidDisplayHex(tx.TxHash())
}

func mkSpend(prev [32]byte, vout uint32, nOut int, tag byte) *wire.Tx {
	outs := make([]wire.TxOut, nOut)
	for i := range outs {
		outs[i] = wire.TxOut{Value: 1000, PkScript: []byte{0x51, tag, byte(i)}}
	}
	return &wire.Tx{
		Version:  1,
		Vin:      []wire.TxIn{{PrevHash: prev, PrevIdx: vout, Script: []byte{0x51}, Sequence: 0xffffffff}},
		Vout:     outs,
		LockTime: 0,
	}
}

func mkMultiSpend(ins []wire.TxIn, nOut int, tag byte) *wire.Tx {
	outs := make([]wire.TxOut, nOut)
	for i := range outs {
		outs[i] = wire.TxOut{Value: 500, PkScript: []byte{0x51, tag, byte(i)}}
	}
	return &wire.Tx{Version: 1, Vin: ins, Vout: outs, LockTime: 0}
}

func mkRoot(nOut int, salt byte) *wire.Tx {
	outs := make([]wire.TxOut, nOut)
	for i := range outs {
		outs[i] = wire.TxOut{Value: 10_000, PkScript: []byte{0x51, salt, byte(i)}}
	}
	var prev [32]byte
	prev[0] = salt
	return &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    outs,
	}
}

func TestPackageDAG_diamond(t *testing.T) {
	p := New(20)
	root := mkRoot(2, 0x10)
	rid := poolAddTx(t, p, root)
	a := mkSpend(root.TxHash(), 0, 1, 0x20)
	aid := poolAddTx(t, p, a)
	b := mkSpend(root.TxHash(), 1, 1, 0x21)
	bid := poolAddTx(t, p, b)
	c := mkMultiSpend([]wire.TxIn{
		{PrevHash: a.TxHash(), PrevIdx: 0, Script: []byte{0x51}, Sequence: 0xffffffff},
		{PrevHash: b.TxHash(), PrevIdx: 0, Script: []byte{0x51}, Sequence: 0xffffffff},
	}, 1, 0x30)
	cid := poolAddTx(t, p, c)

	ancC, err := p.MempoolAncestorTxIDs(cid)
	if err != nil {
		t.Fatal(err)
	}
	wantAnc := []string{aid, bid, rid}
	slices.Sort(wantAnc)
	if !slices.Equal(ancC, wantAnc) {
		t.Fatalf("C ancestors %v want %v", ancC, wantAnc)
	}
	descR, err := p.MempoolDescendantTxIDs(rid)
	if err != nil {
		t.Fatal(err)
	}
	wantDesc := []string{aid, bid, cid}
	slices.Sort(wantDesc)
	if !slices.Equal(descR, wantDesc) {
		t.Fatalf("root descendants %v want %v", descR, wantDesc)
	}

	fees := map[string]int64{rid: 10, aid: 20, bid: 30, cid: 40}
	sizes, err := p.BuildMempoolSizes()
	if err != nil {
		t.Fatal(err)
	}
	st, err := p.PackageStatsForTxID(cid, fees, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if st.AncestorCount != 4 {
		t.Fatalf("ancestor count %d want 4 (root+A+B+C)", st.AncestorCount)
	}
	if st.AncestorFeesKoinu != 100 {
		t.Fatalf("ancestor fees %d want 100", st.AncestorFeesKoinu)
	}
	if st.DescendantCount != 1 || st.DescendantFeesKoinu != 40 {
		t.Fatalf("leaf package desc count=%d fees=%d", st.DescendantCount, st.DescendantFeesKoinu)
	}
}

func TestPackageDAG_twoParentsOneChild(t *testing.T) {
	p := New(10)
	p1 := mkRoot(1, 0x41)
	p2 := mkRoot(1, 0x42)
	id1 := poolAddTx(t, p, p1)
	id2 := poolAddTx(t, p, p2)
	child := mkMultiSpend([]wire.TxIn{
		{PrevHash: p1.TxHash(), PrevIdx: 0, Script: []byte{0x51}, Sequence: 0xffffffff},
		{PrevHash: p2.TxHash(), PrevIdx: 0, Script: []byte{0x51}, Sequence: 0xffffffff},
	}, 1, 0x43)
	cid := poolAddTx(t, p, child)
	anc, err := p.MempoolAncestorTxIDs(cid)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{id1, id2}
	slices.Sort(want)
	if !slices.Equal(anc, want) {
		t.Fatalf("anc %v want %v", anc, want)
	}
	for _, pid := range []string{id1, id2} {
		d, err := p.MempoolDescendantTxIDs(pid)
		if err != nil {
			t.Fatal(err)
		}
		if len(d) != 1 || d[0] != cid {
			t.Fatalf("parent %s desc %v", pid, d)
		}
	}
}

func TestPackageDAG_fanOut(t *testing.T) {
	p := New(20)
	root := mkRoot(3, 0x50)
	rid := poolAddTx(t, p, root)
	var kids []string
	for i := 0; i < 3; i++ {
		ch := mkSpend(root.TxHash(), uint32(i), 1, byte(0x60+i))
		kids = append(kids, poolAddTx(t, p, ch))
	}
	desc, err := p.MempoolDescendantTxIDs(rid)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(kids)
	if !slices.Equal(desc, kids) {
		t.Fatalf("desc %v want %v", desc, kids)
	}
	for _, kid := range kids {
		anc, err := p.MempoolAncestorTxIDs(kid)
		if err != nil {
			t.Fatal(err)
		}
		if len(anc) != 1 || anc[0] != rid {
			t.Fatalf("kid %s anc %v", kid, anc)
		}
	}
}

func TestPackageDAG_propertyRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 25; trial++ {
		p := New(64)
		root := mkRoot(4, byte(trial+1))
		poolAddTx(t, p, root)
		type node struct {
			tx *wire.Tx
			id string
		}
		nodes := []node{{tx: root, id: txidDisplayHex(root.TxHash())}}
		// Grow a small DAG: each new tx spends 1–2 random prior outputs.
		for n := 0; n < 8; n++ {
			parent := nodes[rng.Intn(len(nodes))]
			vout := uint32(rng.Intn(len(parent.tx.Vout)))
			// Avoid double-spend of same outpoint in this synthetic pool.
			spent := false
			_, parents, children, err := p.SpendEdges()
			if err != nil {
				t.Fatal(err)
			}
			_ = parents
			for _, kid := range children[parent.id] {
				for _, nd := range nodes {
					if nd.id != kid {
						continue
					}
					for _, in := range nd.tx.Vin {
						if in.PrevHash == parent.tx.TxHash() && in.PrevIdx == vout {
							spent = true
						}
					}
				}
			}
			if spent {
				continue
			}
			ch := mkSpend(parent.tx.TxHash(), vout, 1+rng.Intn(2), byte(0x80+n))
			id := poolAddTx(t, p, ch)
			nodes = append(nodes, node{tx: ch, id: id})
		}

		_, parents, children, err := p.SpendEdges()
		if err != nil {
			t.Fatal(err)
		}
		// Independent BFS transitive closure vs MempoolAncestor/DescendantTxIDs.
		for _, nd := range nodes {
			wantAnc := transitive(parents, nd.id, false)
			gotAnc, err := p.MempoolAncestorTxIDs(nd.id)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(gotAnc, wantAnc) {
				t.Fatalf("trial %d id %s anc got %v want %v", trial, nd.id, gotAnc, wantAnc)
			}
			wantDesc := transitive(children, nd.id, false)
			gotDesc, err := p.MempoolDescendantTxIDs(nd.id)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(gotDesc, wantDesc) {
				t.Fatalf("trial %d id %s desc got %v want %v", trial, nd.id, gotDesc, wantDesc)
			}
			fees := map[string]int64{}
			for _, x := range nodes {
				fees[x.id] = 1
			}
			sizes, err := p.BuildMempoolSizes()
			if err != nil {
				t.Fatal(err)
			}
			st, err := p.PackageStatsForTxID(nd.id, fees, sizes)
			if err != nil {
				t.Fatal(err)
			}
			if st.AncestorCount != len(wantAnc)+1 {
				t.Fatalf("anc count %d want %d", st.AncestorCount, len(wantAnc)+1)
			}
			if st.DescendantCount != len(wantDesc)+1 {
				t.Fatalf("desc count %d want %d", st.DescendantCount, len(wantDesc)+1)
			}
			if st.AncestorFeesKoinu != int64(st.AncestorCount) {
				t.Fatalf("anc fees %d", st.AncestorFeesKoinu)
			}
		}
	}
}

func transitive(edges map[string][]string, seed string, includeSeed bool) []string {
	seen := map[string]bool{}
	var out []string
	stack := append([]string(nil), edges[seed]...)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		seen[cur] = true
		out = append(out, cur)
		stack = append(stack, edges[cur]...)
	}
	slices.Sort(out)
	if includeSeed {
		out = append(out, seed)
		slices.Sort(out)
	}
	return out
}
