#!/usr/bin/env bash
# Renders all game art with Blender and publishes it to assets/pack.
#   pip install bpy==5.0.1 pillow     (Blender as a Python module)
#   tools/blender/build.sh
set -euo pipefail
cd "$(dirname "$0")/../.."
OUT=${OUT:-build/art}
python3 tools/blender/props.py "$OUT"
python3 tools/blender/pixelate.py "$OUT"
python3 tools/blender/icons.py "$OUT"
python3 tools/blender/pack.py "$OUT" assets/pack
