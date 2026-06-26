# DogeGo

Open-source Dogecoin full node written in Go, with a built-in web dashboard, BlockStep explorer, and Core-compatible JSON-RPC.

**Website:** [dogego.org](https://dogego.org) (static site in [`docs/`](docs/))

## Repository layout

```
dogego/
├── docs/                 GitHub Pages site (dogego.org)
│   ├── index.html
│   ├── css/
│   ├── js/
│   ├── assets/
│   ├── CNAME             Custom domain
│   ├── robots.txt
│   └── sitemap.xml
├── cmd/dogego/           Node binary (coming with v0.1.0-beta)
├── go.mod                Go module (coming soon)
└── README.md
```

The Go source lands in this repo at the root alongside `docs/`. The marketing site stays in `docs/` so GitHub Pages can serve it without mixing build artifacts into the website tree.

## GitHub Pages (dogego.org)

1. Push to `github.com/qlpqlp/dogego`.
2. **Settings → Pages** → Build from **Deploy from a branch**.
3. Branch: `main`, folder: **`/docs`** (not root).
4. **Custom domain:** `dogego.org` (`docs/CNAME` is already set).
5. DNS at your registrar:
   - `A` records to GitHub Pages IPs, or
   - `CNAME`: `dogego.org` → `qlpqlp.github.io`
6. Enable **Enforce HTTPS** after DNS propagates.

## Preview the website locally

Serve the `docs` folder (recommended; anchor links and smooth scroll work over HTTP):

```bash
npx serve docs
```

Then open `http://localhost:3000`.

## Build DogeGo (when source is public)

Requires [Go 1.21+](https://go.dev/dl/).

```bash
git clone https://github.com/qlpqlp/dogego.git
cd dogego
go build -o dogego ./cmd/dogego
./dogego
```

First run opens the setup wizard; the dashboard is at `http://127.0.0.1:2013`.

## License

TBD with the v0.1.0-beta release.
