# Dogecoin Proposals (DIPs)

**DIPs** are DogeGo’s catalog of Bitcoin Improvement Proposals (**BIPs**) as they apply to Dogecoin and this node, plus a few DogeGo-specific overlay proposals.

Numbering usually mirrors the upstream BIP (DIP-0021 tracks BIP 21). Status strings:

| Status | Meaning |
|--------|---------|
| `implemented` | Present in DogeGo and usable |
| `partial` | Present with known gaps vs Core / BIP text |
| `present-disabled` | Code paths exist but consensus-disabled (same as Core policy) |
| `extension` | Lives in an optional extension, not L1 consensus |
| `documented` | Tracked for operators / roadmap |

Open any `dip-XXXX.md` from the WebUI **Docs → DIPs** section, or via the markdown viewer.

## Index

| DIP | Title | BIP | Status |
|-----|-------|-----|--------|
| [DIP-0009](dip-0009.md) | Version bits | BIP9 | implemented |
| [DIP-0021](dip-0021.md) | Payment URI (`dogecoin:`) | BIP21 | implemented |
| [DIP-0022](dip-0022.md) | getblocktemplate | BIP22 | implemented |
| [DIP-0032](dip-0032.md) | Hierarchical deterministic wallets | BIP32 | partial |
| [DIP-0034](dip-0034.md) | Block height in coinbase | BIP34 | implemented |
| [DIP-0035](dip-0035.md) | Mempool P2P message | BIP35 | implemented |
| [DIP-0037](dip-0037.md) | Bloom filters / relaytxes | BIP37 | implemented |
| [DIP-0038](dip-0038.md) | Passphrase-encrypted keys | BIP38 | implemented |
| [DIP-0039](dip-0039.md) | Mnemonic seed phrases | BIP39 | implemented |
| [DIP-0044](dip-0044.md) | Multi-account HD paths | BIP44 | implemented |
| [DIP-0061](dip-0061.md) | Reject messages | BIP61 | implemented |
| [DIP-0065](dip-0065.md) | CHECKLOCKTIMEVERIFY | BIP65 | implemented |
| [DIP-0066](dip-0066.md) | Strict DER signatures | BIP66 | partial |
| [DIP-0068](dip-0068.md) | Relative lock-time (CSV) | BIP68 | implemented |
| [DIP-0112](dip-0112.md) | CHECKSEQUENCEVERIFY | BIP112 | implemented |
| [DIP-0125](dip-0125.md) | Opt-in Replace-by-Fee | BIP125 | partial |
| [DIP-0133](dip-0133.md) | Fee filter | BIP133 | implemented |
| [DIP-0141](dip-0141.md) | Segregated Witness | BIP141 | present-disabled |
| [DIP-0147](dip-0147.md) | NULLDUMMY soft fork | BIP147 | partial |
| [DIP-0152](dip-0152.md) | Compact block relay | BIP152 | partial |
| [DIP-0157](dip-0157.md) | Client-side block filters (P2P) | BIP157 | implemented |
| [DIP-0158](dip-0158.md) | Compact block filters (GCS) | BIP158 | implemented |
| [DIP-0159](dip-0159.md) | NODE_NETWORK_LIMITED | BIP159 | implemented |
| [DIP-0174](dip-0174.md) | Partially Signed Bitcoin Transaction | BIP174 | partial |
| [DIP-3869](dip-3869.md) | Groth16 affine proof encoding (overlay) |  | extension |

## Contributing a DIP

1. Add `dip-NNNN.md` with `# DIP-NNNN: Title`, plus `**Status:**`, `**BIP:**`, `**Summary:**` lines near the top.
2. Link it from this README table.
3. Rebuild DogeGo so the embedded Docs tab picks it up.
4. Prefer documenting intentional differences in `docs/INTENTIONAL_DIFFERENCES.md` and gaps in `docs/CORE_PARITY_GAPS.md`.
