package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"dogego/chain"
	"dogego/pow"
)

func main() {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	h, err := pow.Header80FromParams(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bits := p.GenesisBits

	// Show current committed nonce fails scrypt PoW.
	binary.LittleEndian.PutUint32(h[76:80], p.GenesisNonce)
	if err := pow.CheckScryptPoW(h[:], bits); err != nil {
		fmt.Println("current nonce", p.GenesisNonce, "scrypt PoW: FAIL (", err, ")")
	} else {
		fmt.Println("current nonce already valid")
		return
	}

	for nonce := uint32(0); nonce < 500000000; nonce++ {
		binary.LittleEndian.PutUint32(h[76:80], nonce)
		if pow.CheckScryptPoW(h[:], bits) == nil {
			fmt.Printf("FOUND nonce=%d block_hash=%s\n", nonce, pow.BlockHashHex(h[:]))
			return
		}
		if nonce%2000000 == 0 {
			fmt.Fprintf(os.Stderr, "tried %d...\n", nonce)
		}
	}
	fmt.Fprintln(os.Stderr, "not found")
	os.Exit(1)
}
