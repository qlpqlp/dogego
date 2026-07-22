# DogeGo relay CGNAT (DGR)

Integrated QUIC reachability relay for DogeGo full nodes behind carrier-grade NAT (CGNAT), Starlink, mobile hotspots, and other networks that block inbound TCP on the P2P port.

## Discovery (autonomous)

1. **Operators** advertise `NODE_DOGEGO_RELAY_CGNAT` on every P2P `version` / `addr` handshake. Dogecoin Core and older DogeGo builds **ignore** unknown service bits and are unaffected.
2. **Clients** learn those peers into `dgr_learned_relays.json` and merge them into **Public relay addresses** (`relay_seeds` in `dogecoinconf.json`).
3. Discovery order for QUIC targets: configured `relay_seeds` → learned operators → DNS TXT → live P2P peers with the service bit.
4. When a listening node still has **no inbound** peers after about **10 minutes** (but has outbound), DogeGo **autonomously starts** the outbound DGR client if it was not already running.
5. Among several public relays, clients **crypto-shuffle** targets and periodically rotate sessions so neither the client nor operators can predict which relay is used.

## Service bit

Public relays advertise **`NODE_DOGEGO_RELAY_CGNAT`** on P2P `version` and `addr` messages:

| Name | Value | Hex (services field) |
|------|-------|----------------------|
| `NODE_DOGEGO_RELAY_CGNAT` | `1 << 29` | `0x20000000` |

DogeGo sets this bit when `dogego_relay_cgnat.inbound_relay=true` and the subsystem is enabled.

## Roles

| Role | Config | Behavior |
|------|--------|----------|
| **Public relay (rdogego)** | `enabled` + `inbound_relay` | Listens QUIC UDP, accepts `REGISTER` from CGNAT clients, keeps tunnel sessions alive, forwards `INV` tx frames (phase 1). |
| **CGNAT client** | `enabled` + `outbound_relay` (or `p2p_connectivity=cgnat` with `enabled`) | Dials relays from static seeds, DNS TXT seed, and P2P peers with `NODE_DOGEGO_RELAY_CGNAT`. Registers tunnel; metrics show `using_relay`. |
| **Normal node** | default (DGR off) | Unchanged P2P; can still connect to peers that run relays. |

## Configuration (`dogecoinconf.json`)

```json
{
  "p2p_connectivity": "cgnat",
  "dogego_relay_cgnat": {
    "enabled": true,
    "outbound_relay": true,
    "relay_seeds": ["relay.example.com:24433"],
    "relay_dnsseed": "_dogego-relay.example.com",
    "relay_port": 24433,
    "max_relay_conns": 3,
    "auth_token": ""
  }
}
```

### Public rdogego operator example

```json
{
  "p2p_connectivity": "both",
  "dogego_relay_cgnat": {
    "enabled": true,
    "inbound_relay": true,
    "listen": ":24433",
    "max_clients": 256,
    "auth_token": "change-me-shared-secret",
    "allow_clients": ["0.0.0.0/0"]
  }
}
```

### Fields

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `false` | Master switch (restart required). |
| `inbound_relay` | `false` | Run public QUIC relay listener. |
| `outbound_relay` | auto for `cgnat`/`both` when enabled | Dial and register with relays. |
| `listen` | `:24433` | UDP QUIC bind address (inbound). |
| `relay_port` | `24433` | QUIC port assumed on P2P peers with relay service bit. |
| `relay_seeds` | `[]` | Static `host:port` QUIC targets. |
| `relay_dnsseed` | `""` | One or more DNS hostnames (newline-separated in the web UI). Each hostname's **TXT records** list `host:port` relay lines (not Dogecoin P2P dnsseeds). |
| `auth_token` | `""` | Optional shared secret on `REGISTER` (empty = open). |
| `max_clients` | `256` | Inbound session cap (1..4096). |
| `max_relay_conns` | `3` | Outbound relay sessions (1..16). |
| `allow_clients` | `[]` | Inbound IP/CIDR allowlist (empty = allow all). |
| `relay_tls_pins` | `[]` | SHA-256 hex fingerprints of relay TLS leaf certs (outbound clients). |
| `max_session_frames_per_sec` | `60` | Inbound DGR frames per registered client session. |
| `max_p2p_proxy_per_sec` | `20` | Inbound `P2P_FRAME` proxy requests per client session. |
| `max_register_per_min` | `10` | Inbound `REGISTER` attempts per client IP per minute. |

Settings are also editable under **Settings → P2P → DogeGo relay (CGNAT)** in the web dashboard. Client mode includes **relay TLS cert pins**; operator advanced fields include per-session rate limits.

**Web UI:** Overview → Network shows a live DGR admin card (role, bound UDP, public advertise address, health). Settings → P2P shows the same metrics from `GET /api/dgr` for the running node. Selecting **CGNAT** P2P mode auto-checks outbound QUIC relay on save (unless you manually customized DGR fields).

### Setup wizard auto-config

When **P2P connectivity** is `both`, `classic`, or `cgnat` and you have not manually set `dogego_relay_cgnat`, the first-run wizard:

1. Runs a CGNAT probe (UPnP/NAT-PMP + RFC6598 shared-space check).
2. **Always enables** DGR (`enabled: true`).
3. **Public IP (not CGNAT):** `inbound_relay: true` - your node can act as rdogego.
4. **CGNAT likely:** `inbound_relay: false`, `outbound_relay: true` - uses public relays instead of hosting one.

Preflight step shows the planned DGR role before you save.

## Protocol (DGR1)

QUIC over TLS 1.3 (self-signed cert; optional **`relay_tls_pins`** on clients). One bidirectional stream per session.

Frame format: magic `DGR1`, type `u8`, length `u32` BE, payload.

| Type | Name | Direction | Purpose |
|------|------|-----------|---------|
| 1 | `REGISTER` | client → relay | Network name, auth token, local P2P TCP port |
| 2 | `REGISTER_OK` | relay → client | Session id |
| 3 | `PING` / 4 | `PONG` | Keepalive |
| 5 | `P2P_FRAME` | both | Opaque P2P wire payload (phase 2+) |
| 6 | `PEER_HINT` | relay → client | Suggested peer host:ports |
| 7 | `INV_TX` | client → relay | Legacy tx inv publish (use `P2P_PUBLISH` / `inv`) |
| 8 | `P2P_PUBLISH` | client → relay | Client publishes `inv` / `tx` / `block` for operator to relay on Dogecoin P2P |
| 9 | `P2P_PUSH` | relay → client | Operator fans inbound `inv` / `tx` / `block` / `headers` to registered CGNAT clients |
| 10 | `P2P_TUNNEL` | relay → client | Unsolicited P2P wire frame from a pooled peer TCP connection (phase 2 persistent tunnel) |

## Ports (quick reference)

| Traffic | Protocol | Default port | Who opens router? |
|---------|----------|--------------|-------------------|
| Dogecoin P2P | TCP | 22556 (testnet) / network default | Optional inbound forward for classic listening |
| DGR relay | **UDP** | **24433** | **Operator only** - forward UDP 24433 to the relay machine |
| CGNAT client | UDP outbound | dials **24433** on the operator | **No** inbound forward needed on the client |

Clients and operators use the **same QUIC port (24433)**; only the operator listens, the client connects out.

When **`outbound_relay`** is enabled, DogeGo also:

1. **Auto-addnode** each relay seed hostname on the **Dogecoin P2P port** (e.g. `qlplock.ddns.net:24433` → persistent `addnode qlplock.ddns.net:44556` on testnet).
2. **Prefer DGR P2P tunnel** (`P2P_FRAME`) before direct TCP for outbound peer dials, even when `p2p_connectivity` is `both` or `classic` (not only `cgnat`).

## Discovery order

1. `relay_seeds` from config  
2. DNS TXT on `relay_dnsseed`  
3. Connected P2P peers with `NODE_DOGEGO_RELAY_CGNAT` → same IP, `relay_port`  

## Metrics and observability

### HTTP API

- **`GET /api/dgr`** - full metrics snapshot (also embedded in `/api/p2p` as `dogego_relay_cgnat`).
- **`GET /api/p2p`** - includes `using_relay`, `active_relay` when outbound relay is registered.

### Key counters

| Metric | Meaning |
|--------|---------|
| `listen_bound` | Actual UDP bind address after the relay listener starts |
| `advertise_addr` | Public `host:port` for inbound rdogego (from UPnP external + relay port) |
| `relay_port` | Configured QUIC discovery port (default 24433) |
| `listener_ok` | Inbound QUIC listener is accepting connections |
| `health` | `ok` / `warming` / `starting` / `degraded` / `off` |
| `health_message` | Operator-readable status line |
| `using_relay` | This node has an active outbound relay registration. |
| `active_relay` | QUIC address of the primary outbound relay. |
| `registered_clients` | Inbound relay: connected CGNAT clients. |
| `register_ok` / `register_fail` | Registration outcomes. |
| `dial_attempts` / `dial_ok` / `dial_fail` | Outbound relay dial stats. |
| `frames_in` / `frames_out` | DGR frames moved. |
| `inv_tx_in` / `inv_tx_out` | Tx inv relay frames. |
| `p2p_frames_in` / `p2p_frames_out` | Phase-2 P2P proxy frames. |
| `p2p_publish_in` / `p2p_publish_out` | Phase-4 client publish frames (operator / client). |
| `p2p_push_in` / `p2p_push_out` | Phase-4 operator fan-in push frames. |
| `p2p_tunnel_in` / `p2p_tunnel_out` | Phase-2 persistent tunnel unsolicited peer frames. |
| `p2p_proxy_ok` / `p2p_proxy_fail` | P2P proxy outcomes on inbound relay. |
| `peer_hints_in` / `peer_hints_out` | Suggested P2P `host:port` hints. |
| `tls_pin_ok` / `tls_pin_fail` | Outbound TLS pin verification outcomes (when `relay_tls_pins` set). |
| `rate_limited` | Inbound frames or REGISTER attempts dropped by rate limits. |
| `server_cert_sha256` | Inbound relay TLS leaf fingerprint (copy into client `relay_tls_pins`). |
| `active_relay_cert_sha256` | Outbound client: fingerprint of the active relay cert. |
| `bytes_in` / `bytes_out` | Approximate DGR byte totals. |
| `clients[]` | Per-client rows (inbound operator view). |
| `outbound[]` | Per-relay session rows (CGNAT client view). |
| `discovery_targets` | Last merged seed list. |

Overview → **Network** shows a compact DGR strip when enabled or in use.

## Deployment checklist (rdogego)

1. VPS or home server with **public UDP** to `listen` port (firewall + router forward).  
2. `p2p_connectivity`: `both` or `classic` (normal inbound P2P + relay).  
3. Enable `inbound_relay`, set `auth_token` for production.  
4. Advertise DNS TXT and/or add relay to community seed lists.  
5. Confirm service bit on `getnetworkinfo` / dashboard: `local_services_hex` includes `0x20000000`.  
6. Watch `/api/dgr` for `registered_clients` and frame counters.  

## CGNAT client checklist

1. `p2p_connectivity`: `cgnat` or `both`.  
2. Enable DGR with `outbound_relay` (auto when mode is cgnat).  
3. Configure at least one of `relay_seeds`, `relay_dnsseed`, or wait for P2P peer discovery.  
4. After restart, `/api/dgr` should show `using_relay: true` when a relay accepts `REGISTER`.  
5. P2P multi-peer relay continues on outbound TCP; DGR complements reachability for future tunneled P2P.  

## Security notes

- Use `auth_token` on public relays.  
- Restrict `allow_clients` when possible.  
- TLS uses ephemeral self-signed certs; set **`relay_tls_pins`** (SHA-256 hex of leaf cert DER) on CGNAT clients to pin operator relays. Copy **`server_cert_sha256`** from **`GET /api/dgr`** on the relay node.  
- Inbound relays rate-limit frames per session (`max_session_frames_per_sec`, default 60), P2P proxy requests (`max_p2p_proxy_per_sec`, default 20), and REGISTER attempts per client IP (`max_register_per_min`, default 10).  
- DGR does not replace wallet or RPC authentication.  

## Roadmap

- **Phase 1:** REGISTER, keepalive, tx `INV` relay, metrics, UI, config.  
- **Phase 2:** `P2P_FRAME` proxy with **persistent TCP tunnel pool** per client session; unsolicited peer frames pushed via **`P2P_TUNNEL`**; `PEER_HINT` auto-`addnode`; **`DialP2POutbound`** for primary sync, header probe, primary redial, and relay peers. **CGNAT mode** tries the QUIC tunnel before TCP. **`dogego_dgr_tunnel`** on `getpeerinfo`.  
- **Phase 3:** **`relay_tls_pins`** cert pinning; inbound session rate limits; relay reputation in addrman (DNS QUIC targets sorted by dial score). Metrics: **`tls_pin_ok`**, **`rate_limited`**, **`server_cert_sha256`**, **`active_relay_cert_sha256`**, publish/push/tunnel counters.  
- **Phase 4:** **`P2P_PUBLISH`** client→operator→network for `inv`/`tx`/`block` (wallet txs, mined blocks); **`P2P_PUSH`** operator→client fan-in for `inv`/`tx`/`block`/`headers` on primary and relay peers; hostname resolution for DGR tunnel dials.

## Phase 4: full CGNAT node (bidirectional relay)

When a CGNAT client registers with an operator (`using_relay: true`):

| Direction | What happens |
|-----------|----------------|
| **Client → operator → network** | Wallet `send`, mempool relay, and mined blocks are published via `P2P_PUBLISH`. The operator broadcasts on normal Dogecoin P2P (primary + relay peers). |
| **Network → operator → client** | When the operator receives `inv`, `tx`, `block`, or `headers` on P2P (primary or relay peers), it fans out via `P2P_PUSH` to all registered CGNAT clients. Clients process pushes like inbound P2P (mempool, block fetch, header sync). |
| **Sync** | Outbound `P2P_FRAME` tunnel with **persistent peer TCP pool** + **`P2P_TUNNEL`** push for unsolicited peer traffic + pushed `inv`/`block`/`headers` keep the client current without inbound TCP. |

**Operator requirements:** synced full node, UDP **24433** + TCP **P2P port** forwarded, `inbound_relay: true`.

**Client requirements:** `p2p_connectivity: cgnat`, `outbound_relay: true`, `relay_seeds` pointing at the operator QUIC address.

## Related

- `p2p_connectivity` modes: `classic`, `cgnat`, `both`  
- Dogecoin-QUIC sidecar (tx-only DRL) - reference design; DGR is integrated in DogeGo.  
