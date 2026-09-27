"""Publishes the rendered art as a content-addressed pack for the server.

Copies build/art/pack/* to assets/pack/ with the content hash in every file
name (so they can be cached forever) and writes assets/pack/manifest.json,
the only file clients re-check. sprite-service serves this directory at
/api/v1/sprites/pack/.

    python3 tools/blender/pack.py build/art assets/pack
"""
import hashlib
import json
import shutil
import sys
from pathlib import Path


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()[:12]


def main():
    src, dst = Path(sys.argv[1]), Path(sys.argv[2])
    art = src / "pack"
    if dst.exists():
        shutil.rmtree(dst)
    (dst / "icons").mkdir(parents=True)
    manifest = {"format": 1, "icons": {}, "props": {}}
    h = digest(art / "props.png")
    shutil.copy(art / "props.png", dst / f"props.{h}.png")
    layout = json.loads((art / "props.json").read_text())
    manifest["props"] = {"image": f"props.{h}.png", "tile": layout["tile"], "objects": layout["objects"]}
    if (art / "gui.png").exists():
        h = digest(art / "gui.png")
        shutil.copy(art / "gui.png", dst / f"gui.{h}.png")
        gui = json.loads((art / "gui.json").read_text())
        manifest["gui"] = {"image": f"gui.{h}.png", "scale": gui["scale"], "parts": gui["parts"]}
    for f in sorted((art / "icons").glob("*.png")):
        h = digest(f)
        shutil.copy(f, dst / "icons" / f"{f.stem}.{h}.png")
        manifest["icons"][f.stem] = f"icons/{f.stem}.{h}.png"
    manifest["version"] = hashlib.sha256(json.dumps(manifest, sort_keys=True).encode()).hexdigest()[:12]
    (dst / "manifest.json").write_text(json.dumps(manifest, indent=1, sort_keys=True))
    print("pack", manifest["version"], len(manifest["icons"]), "icons")


if __name__ == "__main__":
    main()
