"""One-shot: regenerate mainnet_seeds.go and testnet_seeds.go from ../src/chainparamsseeds.h."""
import ipaddress
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[2]
H = (ROOT / "src" / "chainparamsseeds.h").read_text(encoding="utf-8")


def parse_block(name: str) -> list[str]:
    m = re.search(
        r"static SeedSpec6 " + re.escape(name) + r"\[\] = \{([\s\S]*?)\};",
        H,
    )
    if not m:
        raise SystemExit("missing " + name)
    out: list[str] = []
    for line in m.group(1).splitlines():
        line = line.strip().rstrip(",")
        # Core format per line: {{0x..,..}, PORT},
        mm = re.match(r"^\{\{(.+)\},\s*(\d+)\}\s*$", line)
        if not mm:
            continue
        hexpart, port_s = mm.group(1), mm.group(2)
        port = int(port_s)
        parts = [int(x.strip(), 16) for x in hexpart.split(",")]
        b = bytes(parts)
        assert len(b) == 16
        if b[:12] == bytes(10) + bytes([255, 255]):
            ip4 = ".".join(str(x) for x in b[12:16])
            out.append(f'"{ip4}:{port}"')
        else:
            a = ipaddress.IPv6Address(b)
            out.append(f'"[{a.compressed}]:{port}"')
    return out


def write_go(path: pathlib.Path, var: str, c_array: str, comment: str) -> None:
    addrs = parse_block(c_array)
    body = ",\n\t".join(addrs)
    path.write_text(
        f"package chain\n\n// {comment}\nvar {var} = []string{{\n\t{body},\n}}\n",
        encoding="utf-8",
    )
    print(path.name, len(addrs), "entries")


if __name__ == "__main__":
    here = pathlib.Path(__file__).resolve().parent
    write_go(
        here / "mainnet_seeds.go",
        "MainnetFixedSeedAddrs",
        "pnSeed6_main",
        "MainnetFixedSeedAddrs are host:port entries from src/chainparamsseeds.h pnSeed6_main "
        "(Dogecoin Core mainnet). Used with DNS seeds; same data as Core autogen.",
    )
    write_go(
        here / "testnet_seeds.go",
        "TestnetFixedSeedAddrs",
        "pnSeed6_test",
        "TestnetFixedSeedAddrs are host:port entries from src/chainparamsseeds.h pnSeed6_test "
        "(Dogecoin Core rebooted testnet). Core has no DNS seeds for this network yet.",
    )
