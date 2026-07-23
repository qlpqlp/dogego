# DogeGo PUP for DogeBox (same pattern as silly-pups gigawallet / pq-wallet).
# Service attr name MUST match manifest container.services[0].name ("dogego").
#
# Configure the node in the DogeGo web dashboard after start (Settings / setup).
# This entrypoint only binds the UI for DogeBox and keeps data under /storage.
# -notls: DogeBox does not terminate TLS; skip local CA install / HTTPS wizard.
#
# After the first DogeBox build, replace src.hash and vendorHash with the
# values printed in the nix log (got: sha256-...), then recompute
# manifest.json container.build.nixFileSha256 (LF-normalized SHA-256 of this file).
{ pkgs ? import <nixpkgs> {} }:

let
  dogego_bin = pkgs.buildGoModule {
    pname = "dogego";
    version = "0.1.0";

    # Matches silly-pups Go pups (gigawallet, pq-wallet, memetracker, …).
    go = pkgs.go_1_24;

    src = pkgs.fetchgit {
      url = "https://github.com/qlpqlp/dogego.git";
      # Native -notls required for DogeBox (do not pin pre-02932c9).
      rev = "2eb7e69da8712ee40563d7541455681e35ffd2c7";
      hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
    };

    modRoot = "DogeGo";
    # Bootstrap hash: first nix build fails and prints the correct value.
    vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

    env.CGO_ENABLED = "0";
    doCheck = false;

    subPackages = [ "cmd/dogego" ];

    ldflags = [
      "-s"
      "-w"
    ];
  };

  dogego = pkgs.writeShellScriptBin "run.sh" ''
    set -euo pipefail

    DATADIR="/storage/dogego"
    WEBUI_PORT="2013"
    BIND="''${DBX_PUP_IP:-0.0.0.0}"

    mkdir -p "$DATADIR"

    echo "DogeGo pup: webui=''${BIND}:''${WEBUI_PORT} datadir=$DATADIR (plain HTTP -notls)"
    exec ${pkgs.coreutils}/bin/env \
      DOGEGO_NO_TLS=1 \
      DOGEGO_NOTLS=1 \
      ${dogego_bin}/bin/dogego node \
      -datadir "$DATADIR" \
      -webui "''${BIND}:''${WEBUI_PORT}" \
      -nobrowser \
      -notls
  '';
in
{
  inherit dogego;
}
