# Credits & acknowledgements

DogeGo is built by [Paulo Vidal](https://github.com/qlpqlp) ([@inevitable360](https://x.com/inevitable360)) with support from the Dogecoin Foundation and the wider community. This page names people whose work DogeGo depends on or integrates — starting with post-quantum Raccoon-G.

## Dogecoin Foundation — post-quantum (Raccoon-G-44)

The in-tree **Raccoon-G-44** C port vendored under `pqcrypto/raccoon_g/native/` (and integrated into Core green) was developed for the Dogecoin Foundation by:

| | |
|--|--|
| **Ed Tubbs** | Sr Software Engineer, Dogecoin Foundation |
| GitHub | [github.com/edtubbs](https://github.com/edtubbs) |
| X | [@EdTubbs](https://x.com/EdTubbs) |
| Upstream tree | [libdogecoin `src/raccoon_g`](https://github.com/dogecoinfoundation/libdogecoin/tree/0.1.5-dev/src/raccoon_g) |
| Core green PR | [dogecoinfoundation/dogecoin#8](https://github.com/dogecoinfoundation/dogecoin/pull/8) |

DogeGo does not reimplement Raccoon in Go for wire compatibility; it compiles Ed’s Foundation in-tree port via CGO. See [RACCOON_G_BUILD.md](RACCOON_G_BUILD.md).

## How to be listed

If you contribute code, review, KATs, docs, DogeBox packaging, or PQ/crypto work that DogeGo relies on, open a PR adding a short row here (name, role, links). Keep entries factual and opt-in.

## Related

- Package docs: `pqcrypto/raccoon_g/doc.go`, `pqcrypto/raccoon.go`
- Build notes: [RACCOON_G_BUILD.md](RACCOON_G_BUILD.md), `pqcrypto/raccoon_g/BUILD.md`
