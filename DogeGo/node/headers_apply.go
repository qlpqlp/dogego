// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func prepareHeadersForConnect(j *store.HeaderJournal, aux *store.HeaderAuxJournal, decoded []wire.DecodedHeader, bs *BlockStoreCtx) error {
	return prepareHeadersForConnectImpl(j, aux, decoded, bs)
}

// prepareHeadersForConnect truncates the journal when the first header builds on a fork ancestor
// (Core reorg), or returns an error if the batch does not connect to genesis or the active chain.
func prepareHeadersForConnectImpl(j *store.HeaderJournal, aux *store.HeaderAuxJournal, decoded []wire.DecodedHeader, bs *BlockStoreCtx) error {
	if len(decoded) == 0 {
		return nil
	}
	var firstPrev [32]byte
	copy(firstPrev[:], decoded[0].Header80[4:36])
	tipHash, err := j.LastTipHash()
	if err != nil {
		return err
	}
	if firstPrev == tipHash {
		return nil
	}
	var z [32]byte
	if firstPrev == z {
		count, err := j.Count()
		if err != nil {
			return err
		}
		if count > 1 {
			return fmt.Errorf("headers: fork from genesis while local tip height %d", count-1)
		}
		return nil
	}
	forkAt, err := j.HeightByBlockHashLE(firstPrev)
	if err != nil {
		return fmt.Errorf("headers: batch does not extend local tip or known ancestor: %w", err)
	}
	tipH, err := j.TipHeight()
	if err != nil {
		return err
	}
	var inc, cur *big.Int
	prefer := false
	if forkAt < tipH {
		var err1, err2 error
		inc, err1 = incomingChainWork(decoded)
		cur, err2 = journalChainWork(j, forkAt+1, tipH)
		if bs != nil && bs.Policy != nil {
			if ph := bs.Policy.PreciousHash(); ph != "" && HeadersBatchContainsHash(decoded, ph) {
				prefer = true
			}
		}
		if err1 == nil && err2 == nil && inc.Cmp(cur) < 0 && !prefer {
			return fmt.Errorf("headers: fork rejected (insufficient chain work)")
		}
		if bs != nil && bs.forkProbe != nil {
			bs.forkProbe(forkAt, firstPrev)
		}
		if err1 == nil && err2 == nil && shouldDeferMarginalReorg(inc, cur, prefer) {
			applog.Line("headers", fmt.Sprintf("marginal fork at height %d: deferring reorg (incoming +%s chain work)", forkAt,
				new(big.Int).Sub(inc, cur).String()))
			return marginalReorgErr(inc, cur)
		}
		if err1 == nil && bs != nil && bs.chainElection != nil {
			elCtx, cancel := context.WithTimeout(context.Background(), forkElectionSyncTimeout+time.Second)
			errEl := bs.chainElection(elCtx, forkAt, firstPrev, decoded, inc)
			cancel()
			if errEl != nil {
				return errEl
			}
		}
		if err1 == nil && err2 == nil {
			logReorgChainWork(forkAt, tipH, inc, cur, prefer)
		}
	}
	applog.Line("headers", fmt.Sprintf("reorg: truncating header journal from height %d to %d (new fork)", tipH, forkAt))
	if forkAt < tipH {
		recordHeaderReorgAnalytics(j, aux, bs, forkAt, tipH, decoded, inc, cur, prefer)
	}
	return truncateChainToHeightLocked(j, aux, bs, forkAt)
}

// ApplyHeadersMessage decodes a P2P "headers" payload, validates against the journal and chain
// rules, and appends. Used during initial getheaders sync and for post-handshake header announcements
// (Core sends these after both sides enable sendheaders).
func ApplyHeadersMessage(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, pl []byte, nowUnix int64, bs *BlockStoreCtx) (count int, partialBatch bool, err error) {
	err = withHeaderChainWriteErr(func() error {
		var innerErr error
		count, partialBatch, innerErr = applyHeadersMessageLocked(j, aux, p, pl, nowUnix, bs)
		return innerErr
	})
	return count, partialBatch, err
}

func applyHeadersMessageLocked(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, pl []byte, nowUnix int64, bs *BlockStoreCtx) (count int, partialBatch bool, err error) {
	decoded, err := wire.DecodeHeadersPayload(pl)
	if err != nil {
		return 0, false, err
	}
	if len(decoded) == 0 {
		return 0, true, nil
	}
	tipBefore, err := j.TipHeight()
	if err != nil {
		return 0, false, err
	}
	if rewound, err := maybeRewindStaleHeaderTimes(j, aux, p, decoded, bs); rewound {
		return 0, false, err
	} else if err != nil {
		return 0, false, err
	}
	if bs != nil && bs.Policy != nil {
		for _, d := range decoded {
			display := pow.BlockHashHex(d.Header80)
			if bs.Policy.IsInvalid(display) {
				return 0, false, fmt.Errorf("headers: block %s… is marked invalid", display[:12])
			}
		}
	}
	if err := prepareHeadersForConnectImpl(j, aux, decoded, bs); err != nil {
		return 0, false, err
	}
	if err := consensus.ValidateHeaders(j, p, decoded, nowUnix); err != nil {
		if rewound, rerr := maybeRewindOnBadNBits(j, aux, p, bs, err); rewound {
			return 0, false, rerr
		}
		if rewound, rerr := maybeRewindOnAuxpowRuleMismatch(j, aux, p, bs, err); rewound {
			return 0, false, rerr
		}
		if rewound, rerr := maybeRewindOnInvalidAuxPow(j, aux, p, bs, err); rewound {
			return 0, false, rerr
		}
		if rewound, rerr := maybeRewindOnCheckpointMismatch(j, aux, p, bs, err); rewound {
			return 0, false, rerr
		}
		return 0, false, err
	}
	if aux != nil {
		headerCount, err := j.Count()
		if err != nil {
			return 0, false, err
		}
		if err := aux.EnsureRecordCount(headerCount); err != nil {
			return 0, false, fmt.Errorf("header aux align: %w", err)
		}
	}
	hdrBatch := make([]byte, len(decoded)*80)
	for k, d := range decoded {
		if len(d.Header80) != 80 {
			return 0, false, fmt.Errorf("header %d: bad len %d", k, len(d.Header80))
		}
		copy(hdrBatch[k*80:], d.Header80)
	}
	if err := j.AppendWireHeaderBatch(hdrBatch); err != nil {
		return 0, false, err
	}
	if bs != nil && bs.ChainWork != nil {
		if batchWork, err := incomingChainWork(decoded); err == nil {
			bs.ChainWork.Extend(tipBefore, tipBefore+int64(len(decoded)), batchWork)
		}
	}
	if aux != nil {
		auxBlobs := make([][]byte, len(decoded))
		for k, d := range decoded {
			if d.Aux != nil {
				auxBlobs[k], err = wire.SerializeAuxPow(d.Aux)
				if err != nil {
					return 0, false, fmt.Errorf("header %d aux serialize: %w", k, err)
				}
			}
		}
		if err := aux.AppendEntries(auxBlobs); err != nil {
			return 0, false, err
		}
	}
	maybeClearPostAuxEraStallHint(p.Net, tipBefore+int64(len(decoded)))
	return len(decoded), len(decoded) < 2000, nil
}
