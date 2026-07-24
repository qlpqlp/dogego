# DogeGo PUP for DogeBox / silly-pups.
# Service attr name MUST match manifest container.services[0].name ("dogego").
#
# Starts the setup wizard (no -datadir / no seeded dogecoinconf.json).
# cwd is /storage/dogego so wizard default ./dogedata → /storage/dogego/dogedata.
#
# Plain HTTP: native DogeGo -notls + DOGEGO_NO_TLS (commit 02932c9+). Do NOT pin
# pre-notls revs (e.g. 9d88c34) — wizard defaults force webui_tls_local + CA install,
# and omitempty JSON makes the old setup UI treat "TLS off" as "TLS on" when saving.
#
# DogeBox proxies from a private IP: set DOGEGO_TRUST_PRIVATE_CLIENTS=1 so setup
# wallet-backup and other loopback-gated APIs accept the proxy (else 403).
# Force webui=$DBX_PUP_IP:2013 in conf + CLI (wizard defaults to localhost and
# would break the proxy right after first setup until container restart).
# DOGEGO_HEADLESS=1 / -tray=false avoid desktop tray/DBus noise in the pup.
#
# Avoid pkgs.buildGoModule: DogeBox nixpkgs sets env.CGO_ENABLED, and a
# top-level CGO_ENABLED = "0" (legacy/injected) makes evaluation fail with
# overlapping env vs derivation attributes.
#
# Includes native -notls / DOGEGO_NO_TLS. Keep src.hash / goModules outputHash in sync
# with the pinned rev, then recompute manifest.json nixFileSha256 (LF SHA-256 of this file).
{ pkgs ? import <nixpkgs> {} }:

let
  src = pkgs.fetchgit {
    url = "https://github.com/qlpqlp/dogego.git";
    # Includes -notls, DOGEGO_TRUST_PRIVATE_CLIENTS, setup uacomment-preview.
    rev = "f3fd3a56a628699a6fa0bf6f5ca68b7d826a67bf";
    hash = "sha256-ikmuCWwYfxCtGWZK8KHqq4uVow6niequQJXOZAnc44w=";
  };

  goModules = pkgs.stdenv.mkDerivation {
    name = "dogego-go-modules";
    inherit src;
    nativeBuildInputs = [
      pkgs.go_1_24
      pkgs.git
      pkgs.cacert
    ];
    impureEnvVars = pkgs.lib.fetchers.proxyImpureEnvVars ++ [
      "GIT_PROXY_COMMAND"
      "SOCKS_SERVER"
      "GOPROXY"
    ];
    configurePhase = ''
      runHook preConfigure
      export GOCACHE=$TMPDIR/go-cache
      export GOPATH=$TMPDIR/go
      cd DogeGo
      runHook postConfigure
    '';
    buildPhase = ''
      runHook preBuild
      export GIT_SSL_CAINFO=$NIX_SSL_CERT_FILE
      go mod vendor
      mkdir -p vendor
      runHook postBuild
    '';
    installPhase = ''
      runHook preInstall
      cp -r --reflink=auto vendor $out
      runHook postInstall
    '';
    dontFixup = true;
    outputHashMode = "recursive";
    outputHash = "sha256-xwHNyDyPMEXSY7A71/t/mGdgtoXxibiHghu8OvfVOYI=";
  };

  dogego_bin = pkgs.stdenv.mkDerivation {
    pname = "dogego";
    version = "0.1.0";
    inherit src;

    nativeBuildInputs = [ pkgs.go_1_24 pkgs.python3 ];

    dontConfigure = true;

    # Keep dashboard on the pup IP after wizard finish (upstream pin still
    # replaces merged config with localhost:2013 from setup.html defaults).
    # Drop when pup.nix pins a rev that includes the webui-align fix.
    postPatch = ''
      ${pkgs.python3}/bin/python3 - <<'PY'
from pathlib import Path

main = Path("DogeGo/cmd/dogego/main.go")
mt = main.read_text(encoding="utf-8")
old_main = """\t\tmerged = config.FromFile(saved)
\t\tif wantNoTLS {
\t\t\tconfig.ApplyNoTLSMerged(&merged)
\t\t}
\t\tskipBrowserThisStart = true
\t}"""
new_main = """\t\tmerged = config.FromFile(saved)
\t\tif wantNoTLS {
\t\t\tconfig.ApplyNoTLSMerged(&merged)
\t\t\tconfig.DisableLocalTLS(&saved)
\t\t}
\t\t// Wizard JSON defaults to localhost:2013. Re-apply CLI listen flags so DogeBox
\t\t// (and any -webui beyond loopback) keeps the dashboard reachable after setup.
\t\tconfDirty := wantNoTLS
\t\tif visited["webui"] {
\t\t\tmerged.WebUI = strings.TrimSpace(*webui)
\t\t\tsaved.WebUI = merged.WebUI
\t\t\tconfDirty = true
\t\t}
\t\tif visited["nobrowser"] {
\t\t\tmerged.NoBrowser = *nobrowser
\t\t\tsaved.NoBrowser = merged.NoBrowser
\t\t\tconfDirty = true
\t\t}
\t\tif visited["tray"] {
\t\t\tmerged.Tray = *tray
\t\t\tsaved.Tray = config.TrayPtr(merged.Tray)
\t\t\tconfDirty = true
\t\t}
\t\tif confDirty {
\t\t\tif err := config.Save(savePath, saved); err != nil {
\t\t\t\tfmt.Fprintf(os.Stderr, "DogeGo: warning: could not refresh conf after setup: %v\\n", err)
\t\t\t}
\t\t}
\t\tskipBrowserThisStart = true
\t}"""
if old_main not in mt:
    raise SystemExit("main.go post-wizard patch target not found")
main.write_text(mt.replace(old_main, new_main, 1), encoding="utf-8")

setup = Path("DogeGo/ui/setup.go")
st = setup.read_text(encoding="utf-8")
old_seed = "\tseed = config.SetupWizardSeed(seed)\n\thtml, err := fs.ReadFile(static, \"static/setup.html\")"
new_seed = """\tseed = config.SetupWizardSeed(seed)
\t// DogeBox / non-loopback -webui: keep the listen address in the saved conf.
\tif ha, _, err := net.SplitHostPort(strings.TrimSpace(listenAddr)); err == nil {
\t\tswitch strings.ToLower(strings.Trim(ha, "[]")) {
\t\tcase "localhost", "127.0.0.1", "::1", "":
\t\tdefault:
\t\t\tseed.WebUI = listenAddr
\t\t}
\t}
\thtml, err := fs.ReadFile(static, "static/setup.html")"""
if old_seed not in st:
    raise SystemExit("setup.go seed patch target not found")
st = st.replace(old_seed, new_seed, 1)
if "\t\"net\"\n" not in st and "\n\t\"net\"\n" not in st:
    st = st.replace("\t\"io/fs\"\n\t\"net/http\"\n", "\t\"io/fs\"\n\t\"net\"\n\t\"net/http\"\n", 1)
old_f = """\t\tf := req.File
\t\tif seed.NoTLS {
\t\t\tconfig.DisableLocalTLS(&f)
\t\t}
\t\tstartNode := true"""
new_f = """\t\tf := req.File
\t\tif seed.NoTLS {
\t\t\tconfig.DisableLocalTLS(&f)
\t\t}
\t\tif !req.DualInstance {
\t\t\twu := strings.TrimSpace(f.WebUI)
\t\t\thost := wu
\t\t\tif h, _, err := net.SplitHostPort(wu); err == nil {
\t\t\t\thost = h
\t\t\t}
\t\t\tswitch strings.ToLower(strings.Trim(host, "[]")) {
\t\t\tcase "", "localhost", "127.0.0.1", "::1":
\t\t\t\tif ha, _, err := net.SplitHostPort(strings.TrimSpace(listenAddr)); err == nil {
\t\t\t\t\tswitch strings.ToLower(strings.Trim(ha, "[]")) {
\t\t\t\t\tcase "localhost", "127.0.0.1", "::1", "":
\t\t\t\t\tdefault:
\t\t\t\t\t\tf.WebUI = listenAddr
\t\t\t\t\t}
\t\t\t\t}
\t\t\t}
\t\t}
\t\tstartNode := true"""
if old_f not in st:
    raise SystemExit("setup.go save patch target not found")
setup.write_text(st.replace(old_f, new_f, 1), encoding="utf-8")
print("DogeGo pup: patched post-wizard webui bind for DogeBox")
PY
    '';

    buildPhase = ''
      runHook preBuild
      export GOCACHE=$TMPDIR/go-cache
      export GOPATH=$TMPDIR/go
      export GO111MODULE=on
      export GOTOOLCHAIN=local
      export CGO_ENABLED=0
      export GOPROXY=off
      export GOSUMDB=off
      cd DogeGo
      rm -rf vendor
      cp -r --reflink=auto ${goModules} vendor
      go build -mod=vendor -trimpath -ldflags="-s -w" -o dogego ./cmd/dogego
      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      mkdir -p $out/bin
      cp dogego $out/bin/dogego
      runHook postInstall
    '';
  };

  dogego = pkgs.writeShellScriptBin "run.sh" ''
    set -euo pipefail

    WORKDIR="/storage/dogego"
    WEBUI_PORT="2013"
    BIND="''${DBX_PUP_IP:-0.0.0.0}"
    WEBUI="''${BIND}:''${WEBUI_PORT}"

    mkdir -p "$WORKDIR/dogedata" \
      "$WORKDIR/.config/DogeGo" \
      "$WORKDIR/.local/share" \
      "$WORKDIR/.cache"
    chmod -R u+rwX "$WORKDIR" || true

    # Drop stale conf from older pup builds that enabled local HTTPS / CA install.
    if [ -f "$WORKDIR/dogecoinconf.json" ]; then
      if [ ! -d "$WORKDIR/dogedata/mainnet" ] \
         && [ ! -d "$WORKDIR/dogedata/testnet" ] \
         && [ ! -d "$WORKDIR/mainnet" ] \
         && [ ! -d "$WORKDIR/testnet" ]; then
        rm -f "$WORKDIR/dogecoinconf.json"
        rm -f "$WORKDIR/.config/DogeGo/dogecoinconf.json"
      fi
    fi
    # Keep conf aligned with the pup IP: wizard defaults write localhost:2013, which
    # breaks the DogeBox reverse proxy after setup until the container restarts.
    CONF="$WORKDIR/dogecoinconf.json"
    if [ ! -f "$CONF" ] && [ -f "$WORKDIR/.config/DogeGo/dogecoinconf.json" ]; then
      CONF="$WORKDIR/.config/DogeGo/dogecoinconf.json"
    fi
    if [ -f "$CONF" ] && command -v ${pkgs.python3}/bin/python3 >/dev/null 2>&1; then
      ${pkgs.python3}/bin/python3 - "$CONF" "$WEBUI" <<'PY' || true
import json, sys
path, webui = sys.argv[1], sys.argv[2]
try:
    with open(path, encoding="utf-8") as f:
        conf = json.load(f)
except Exception:
    raise SystemExit(0)
changed = False
for k, v in (
    ("webui", webui),
    ("webui_tls_local", False),
    ("rpc_tls_local", False),
    ("local_tls_trust_ca", False),
    ("no_tls", True),
    ("tray", False),
    ("nobrowser", True),
):
    if conf.get(k) != v:
        conf[k] = v
        changed = True
for k in ("webui_tls_cert", "webui_tls_key", "rpc_tls_cert", "rpc_tls_key"):
    if conf.pop(k, None) is not None:
        changed = True
if changed:
    with open(path, "w", encoding="utf-8") as f:
        json.dump(conf, f, indent=2)
        f.write("\n")
    print("DogeGo pup: aligned conf", path, "webui=", webui)
PY
    fi

    cd "$WORKDIR"

    echo "DogeGo pup: HTTP webui=$WEBUI workdir=$WORKDIR (native -notls, trust private clients, headless)"
    exec ${pkgs.coreutils}/bin/env \
      HOME="$WORKDIR" \
      XDG_CONFIG_HOME="$WORKDIR/.config" \
      XDG_DATA_HOME="$WORKDIR/.local/share" \
      XDG_CACHE_HOME="$WORKDIR/.cache" \
      DOGEGO_NO_TLS=1 \
      DOGEGO_NOTLS=1 \
      DOGEGO_TRUST_PRIVATE_CLIENTS=1 \
      DOGEGO_HEADLESS=1 \
      ${dogego_bin}/bin/dogego node \
        -webui "$WEBUI" \
        -nobrowser \
        -tray=false \
        -notls
  '';

  monitor = pkgs.stdenv.mkDerivation {
    pname = "dogego-monitor";
    version = "0.1.0";
    src = ./monitor;
    nativeBuildInputs = [ pkgs.go_1_24 ];
    dontConfigure = true;
    buildPhase = ''
      export GOCACHE=$TMPDIR/go-cache
      export GOPATH=$TMPDIR/go
      export GO111MODULE=off
      export CGO_ENABLED=0
      go build -trimpath -ldflags="-s -w" -o monitor monitor.go
    '';
    installPhase = ''
      mkdir -p $out/bin
      cp monitor $out/bin/monitor
    '';
  };
in
{
  inherit dogego monitor;
}
