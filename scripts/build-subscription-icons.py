#!/usr/bin/env python3
"""Render the checked-in Lucide sources; requires CairoSVG 2.8.2.

Only maintainers regenerating PNGs need CairoSVG. Normal builds use the assets.
"""
import json
from pathlib import Path
from xml.etree import ElementTree as ET

import cairosvg

root = Path(__file__).resolve().parents[1] / "apps/web/public/subscription-icons/v1"
manifest = json.loads((root / "manifest.json").read_text())
for name, spec in manifest["groups"].items():
    glyph = ET.fromstring((root / "source" / (spec["glyph"] + ".svg")).read_text())
    body = "".join(ET.tostring(child, encoding="unicode") for child in glyph)
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
<rect x="4" y="4" width="120" height="120" rx="32" fill="{spec['color']}"/>
<g transform="translate(28 28) scale(3)" fill="none" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">{body}</g>
</svg>'''
    (root / (name + ".svg")).write_text(svg + "\n")
    cairosvg.svg2png(bytestring=svg.encode(), write_to=str(root / (name + ".png")))
print(f"Rendered {len(manifest['groups'])} icons at 128 x 128.")
