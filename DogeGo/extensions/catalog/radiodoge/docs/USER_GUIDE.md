# RadioDoge extension (DogeGo)

Connect a **RadioDoge** Heltec WiFi LoRa 32 **V3** SoftAP device to DogeGo so you can:

1. **Broadcast** signed Dogecoin transactions over the LoRa mesh when this host has **no internet**
2. **Relay** inbound mesh transactions into the local node with `sendrawtransaction` when this host **is** online

This mirrors the HTTP SoftAP integration in [Dogecoin Wallet](https://github.com/qlpqlp/dogecoin-wallet) (`RadioDogeHelper`) and talks to firmware from:

[dogecoinfoundation/radiodoge `heltec-firmware-v3`](https://github.com/dogecoinfoundation/radiodoge/tree/0.0.1-Beta-1/heltec-firmware-v3)

## How it connects

There is **no USB/serial** path. The device opens WiFi SoftAP:

| Item | Value |
|------|--------|
| SSID | `RadioDoge` |
| SoftAP IP | `192.168.4.1` |
| Broadcast | `POST /api/broadcast` `type=transaction&priority=normal&message=<hex>` |
| Status | `GET /api/status` |
| Logs | `GET /api/logs` |

Join the SoftAP from the machine running DogeGo, or keep the device reachable on your LAN at the configured base URL (default `http://192.168.4.1`).

## Install

1. Build the universal zip (or install from catalog once published):

```powershell
cd DogeGo/extensions/catalog/radiodoge
./build-universal.ps1
```

2. In DogeGo **Extensions** → install `dist/radiodoge-universal.zip` → **Enable** `dogego.radiodoge`.

3. Unlock the wallet in the dashboard if you want smart local broadcast / inbound relay (`wallet_rpc`).

## Operator flow

### Offline client (like the Android wallet)

1. Enable RadioDoge in extension Settings (`prefer_radio_offline` on).
2. Connect the host WiFi to SSID `RadioDoge` (or ensure SoftAP is reachable).
3. Tools → **Broadcast via RadioDoge** with a signed raw tx hex, or call:

```text
dogego_ext_dogego_radiodoge_broadcast {"hex":"01000000...","txid":"<optional>"}
```

`broadcast_smart` uses RadioDoge when offline + device up; otherwise `sendrawtransaction`.

### Online gateway

1. Keep DogeGo online with peers.
2. Leave **Auto-relay inbound mesh txs** enabled.
3. Optionally set gateway fields in Settings and run **Push gateway config to device** so the Heltec forwards to your node/RPC.

Inbound hex seen in `/api/logs` is submitted via `sendrawtransaction`.

## RPC

| Method | Purpose |
|--------|---------|
| `info` | Status + UI panel |
| `probe` | SoftAP reachability / Ready |
| `should_use_radio` | Offline gate check |
| `broadcast` | Mesh broadcast |
| `broadcast_smart` | P2P or mesh |
| `send_direct` | Unicast LoRa `/api/transaction` |
| `configure_gateway` | `POST /api/gateway/save` |
| `logs` | Fetch + parse confirmations |
| `getconfig` / `setconfig` | Persist prefs |

## Credits

- RadioDoge firmware: [Dogecoin Foundation radiodoge](https://github.com/dogecoinfoundation/radiodoge)
- Wallet SoftAP client pattern: Paulo Vidal / Dogecoin Wallet RadioDoge support
