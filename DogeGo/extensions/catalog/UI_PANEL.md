# Secure extension WebUI (`ui_panel`)

Extensions **do not** ship HTML, JavaScript, or CSS into the dashboard. That keeps the node UI sandboxed.

Instead, an enabled extension with the `ui_panel` permission returns a **JSON `ui` object** from its status RPC (usually `info`). DogeGo's host renders that object with allowlisted controls only; all strings are escaped.

## How operators see it

1. **Extensions** → open the extension card  
2. Extension must be **Enabled**  
3. **Extension workspace** card appears with a left **Menu** (Home / Tools / Settings / custom sections)

If an extension only returns a flat `summary` / `tools` list, the host **auto-promotes** it into the same Menu shell.

**Mobile / layout**

- **Menu** can be collapsed (hide/show)
- **Results** console collapses (especially on small screens)
- Tool cards are collapsible; mark `advanced: true` to start closed
- Tables support search + “show more” infinite scroll

## How authors add UI

Declare:

```json
"permissions": ["rpc_register", "ui_panel"],
"ui": { "status_method": "info" }
```

Return from `info` (or `ui_status`):

```json
{
  "ui": {
    "panel_title": "My extension",
    "subtitle": "One line status",
    "layout": "workspace",
    "status_chips": [{ "id": "ok", "label": "State", "value": "Ready", "tone": "ok", "icon": "check" }],
    "nav": [
      { "id": "home", "label": "Home", "icon": "home" },
      { "id": "tools", "label": "Tools", "icon": "construction" },
      { "id": "settings", "label": "Settings", "icon": "tune" }
    ],
    "sections": {
      "home": {
        "title": "Overview",
        "lead": "Keep Home light.",
        "widgets": [
          { "type": "stats", "items": [{ "label": "Peers", "value": "2", "icon": "hub" }] },
          { "type": "progress", "label": "Sync", "percent": 80, "value": "80%" },
          {
            "type": "metric_chart", "title": "Activity", "chart": "line",
            "labels": ["a", "b", "c"],
            "series": [{ "label": "n", "data": [1, 3, 2], "color": "#c2a633" }]
          },
          {
            "type": "table", "title": "Rows", "search": true, "page_size": 20,
            "columns": [{ "key": "id", "label": "ID" }, { "key": "name", "label": "Name" }],
            "rows": [{ "id": "1", "name": "woof" }]
          }
        ],
        "quick_actions": [{ "id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh" }]
      },
      "tools": {
        "title": "Tools",
        "tools": [{
          "id": "ping", "label": "Ping", "method": "ping", "icon": "network_ping"
        }]
      },
      "settings": {
        "title": "Settings",
        "lead": "Put wallet_rpc toggles and preferences here. data/ survives upgrades.",
        "tools": [{
          "id": "setconfig", "label": "Save", "method": "setconfig", "params_as": "object",
          "fields": [
            { "name": "wallet_rpc_enabled", "label": "Enable wallet RPC", "type": "switch", "default": "false" }
          ]
        }]
      }
    }
  }
}
```

Go helper: `extensions.DefaultWorkspaceUI(title, subtitle, tools)`.

### Allowed widget types

| `type` | Purpose |
|--------|---------|
| `stats` | KPI tiles |
| `item_list` | Simple title/meta rows (search + show more) |
| `proof_list` | ZK-style proof cards |
| `table` | Modern table with search + infinite scroll (`page_size`, optional `load_more`) |
| `metric_chart` | Line/bar chart (`labels` + `series[]`) via host Chart.js |
| `callout` | Info / warn banner (`tone`, `title`, `body`) |
| `progress` | Progress bar (`percent`, `label`, `value`) |

### Allowed field types

`text`, `number`, `textarea`, `select` (modern choice boxes — click cards, not a native dropdown), `switch` (preferred), `checkbox` (same modern toggle)

Optional field `hint` shows helper text under the control. Select `options` may include `icon` (Material Icons name).

### Wizards

A section may include `wizards: [{ id, title, method, steps: [{ title, fields }] }]`. The host walks steps and calls `method` with one JSON object of collected fields.

### Config & backups

- Store settings under `extensions/<id>/data/` so **zip upgrades keep them** (installer preserves `data/`).
- Prefer Settings forms with `switch` fields + `setconfig`.
- Optional: `exportbackup` / `importbackup` RPCs (see Doginals) writing JSON under `data/backups/`.

## Security model

| Allowed | Forbidden |
|---------|-----------|
| JSON `ui` via status RPC | Raw HTML / `<script>` / CSS injection |
| Host-escaped labels and values | Setting `innerHTML` from extension blobs |
| Allowlisted widgets + form fields | Arbitrary DOM / iframes |
| `wallet_rpc` allowlist + dashboard unlock | Key export / `walletpassphrase` from the extension |

Panel HTTP: `GET /api/extensions/panel?id=<extension-id>` (requires dashboard auth when configured).

See also [AUTHORING.md](AUTHORING.md) and examples: `doginals`, `zkl2`, `bbpow`, `example-go`.
