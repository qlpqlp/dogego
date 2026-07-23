# DogeGo PUP for DogeBox.
# Builds the Go node from this GitHub repo (modRoot = DogeGo/).
# Service attr name MUST match manifest container.services[0].name ("dogego").
#
# Configure the node in the DogeGo web dashboard after start (Settings / setup).
# This entrypoint only binds the UI for DogeBox and keeps data under /storage.
#
# After the first DogeBox build, replace src.hash and vendorHash with the
# values printed in the nix log (got: sha256-...), then recompute
# manifest.json container.build.nixFileSha256 (LF-normalized SHA-256 of this file).
{ pkgs ? import {} }:

let
  dogegoSrc = pkgs.fetchgit {
    url = "https://github.com/qlpqlp/dogego.git";
    # Pin to a release tag when available, e.g. refs/tags/v0.1.0
    rev = "refs/heads/main";
    # Bootstrap hash: first nix build fails and prints the correct value.
    hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
  };

  dogego_bin = pkgs.buildGoModule {
    pname = "dogego";
    version = "0.1.0";

    # DogeBox nixpkgs no longer ships go_1_22; 1.23+ builds DogeGo (go.mod 1.22.0).
    go = pkgs.go_1_23;

    src = dogegoSrc;
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
      -nobrowser
  '';
in
{
  inherit dogego;
}
