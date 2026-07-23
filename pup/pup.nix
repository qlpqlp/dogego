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
      # Pin a commit SHA (not a moving tag). pup.nix lives in this same repo; if
      # rev tracked refs/tags/v0.1.0 and you retagged after editing pup.nix, the
      # tree hash would change and break fetchgit. DogeBox loads pup.nix from the
      # pup source (usually main); this rev only pins the Go sources to build.
      rev = "9d88c34dd3f8f64bc2c5c6afb58062b0da2adb5c";
      hash = "sha256-r1OzX4f9whHdDEruH0/n+yW7kydgGl5S2cAWwaK2xuE=";
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

    echo "DogeGo pup: webui=''${BIND}:''${WEBUI_PORT} datadir=$DATADIR (configure in the DogeGo web UI)"
    exec ${dogego_bin}/bin/dogego node \
      -datadir "$DATADIR" \
      -webui "''${BIND}:''${WEBUI_PORT}" \
      -nobrowser \
      -notls
  '';
in
{
  inherit dogego;
}
