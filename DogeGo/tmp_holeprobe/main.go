package main

import (
	"fmt"
	"os"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func main() {
	chainDir := `C:\dogedata\mainnet`
	rbDir := `C:\dogedata\mainnet\rawblocks`
	net := chain.MainnetDogecoin
	rawGen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		panic(err)
	}
	j, err := store.OpenHeaderChain(chainDir, rawGen[:80])
	if err != nil {
		panic(err)
	}
	// OpenRawBlockStore expects chain dir (it appends /rawblocks).
	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		panic(err)
	}
	n, _ := raw.Count()
	tip, _ := j.TipHeight()
	fmt.Printf("raw_count=%d tip=%d opts=%+v\n", n, tip, raw.StorageOpts())
	for _, h := range []int64{0, 1, 100, 8300, 8301, 8302, 8303, 10000, 50000, 100000, 200000, 231270, 231272} {
		hdr, err := j.ReadHeaderAt(h)
		if err != nil {
			fmt.Printf("h=%d header_err=%v\n", h, err)
			continue
		}
		hash := pow.BlockHashLE(hdr)
		has := store.HasStoredBodyAtHeight(j, raw, h, net)
		minB := store.MinRawBlockBytes(net, h)
		adeq := raw.HasStoredBody(hash, minB)
		bin := fmt.Sprintf("%s\\%x.bin", rbDir, hash)
		binSize := int64(-1)
		if st, err := os.Stat(bin); err == nil {
			binSize = st.Size()
		}
		fmt.Printf("h=%-6d hasAt=%-5v adeq=%-5v min=%-3d bin=%-6d hashLE8=%x\n", h, has, adeq, minB, binSize, hash[:8])
	}
	hole := int64(-1)
	for h := int64(0); h < 250000; h++ {
		if !store.HasStoredBodyAtHeight(j, raw, h, net) {
			hole = h
			break
		}
	}
	fmt.Printf("first_hole=%d\n", hole)
}
