"""Synchronise embedded and installer versions with the release tag."""

import json
from pathlib import Path
import re
import sys


def replace(path: Path, pattern: str, replacement: str, count: int = 1) -> None:
    source = path.read_text(encoding="utf-8")
    result, actual = re.subn(pattern, replacement, source, count=count, flags=re.MULTILINE)
    if actual != count:
        raise ValueError(f"Unexpected version field in {path.name}")
    path.write_text(result, encoding="utf-8")


def prepare(root: Path, tag: str) -> None:
    number = r"(?:0|[1-9]\d*)"
    identifier = rf"(?:{number}|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
    match = re.fullmatch(rf"v?({number}\.{number}\.{number}(?:-{identifier}(?:\.{identifier})*)?)", tag)
    if not match:
        raise ValueError("Release tag must be a semantic version, e.g. v1.2.3")
    version = match[1]
    numeric = version.split("-", 1)[0]
    build = root / "build"
    replace(build / "config.yml", r'^  version: "[^"]+"', f'  version: "{version}"')
    replace(build / "darwin/Info.plist", r'(<key>CFBundle(?:ShortVersionString|Version)</key>\s*<string>)[^<]+', rf'\g<1>{numeric}', 2)
    info_path = build / "windows/info.json"
    info = json.loads(info_path.read_text(encoding="utf-8"))
    info["fixed"]["file_version"] = numeric
    info["info"]["0000"]["ProductVersion"] = version
    info_path.write_text(json.dumps(info, indent="\t", ensure_ascii=False) + "\n", encoding="utf-8")
    for name in ("project.nsi", "wails_tools.nsh"):
        replace(build / "windows/nsis" / name, r'(!define INFO_PRODUCTVERSION )"[^"]+"', rf'\g<1>"{numeric}"')
    replace(build / "linux/nfpm/nfpm.yaml", r'^version: "[^"]+"', f'version: "{version}"')


if __name__ == "__main__":
    prepare(Path(__file__).resolve().parent.parent, sys.argv[1])
