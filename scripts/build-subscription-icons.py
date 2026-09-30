#!/usr/bin/env python3
"""Render checked-in SVG sources; requires CairoSVG 2.8.2.

Only maintainers regenerating PNGs need CairoSVG. Normal builds use the assets.
Use --version v1 to reproduce the original category icon set.
"""
import argparse
import json
from pathlib import Path
from xml.etree import ElementTree as ET

import cairosvg

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--version", choices=("v1", "v2"), default="v2")
args = parser.parse_args()
root = Path(__file__).resolve().parents[1] / "apps/web/public/subscription-icons" / args.version
manifest = json.loads((root / "manifest.json").read_text())
for name, spec in manifest["groups"].items():
    if args.version == "v1":
        glyph = ET.fromstring((root / "source" / (spec["glyph"] + ".svg")).read_text())
        body = "".join(ET.tostring(child, encoding="unicode") for child in glyph)
        svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
<rect x="4" y="4" width="120" height="120" rx="32" fill="{spec['color']}"/>
<g transform="translate(28 28) scale(3)" fill="none" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">{body}</g>
</svg>'''
    else:
        glyph = ET.fromstring((root / "source" / spec["source"]).read_text().replace("currentColor", spec["color"]))
        # Keep upstream shapes, aspect ratio, paths and multicolor gradients.
        glyph.set("x", "26")
        glyph.set("y", "26")
        glyph.set("width", "76")
        glyph.set("height", "76")
        glyph.set("preserveAspectRatio", "xMidYMid meet")
        if spec["style"] == "brand":
            glyph.set("fill", spec["color"])
        else:
            glyph.set("stroke", spec["color"])
            glyph.set("stroke-width", "1.8")
        svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
<rect x="3" y="3" width="122" height="122" rx="30" fill="{spec['background']}" stroke="#e2e8f0" stroke-width="1.5"/>
{ET.tostring(glyph, encoding='unicode')}
</svg>'''
    (root / (name + ".svg")).write_text(svg + "\n")
    cairosvg.svg2png(bytestring=svg.encode(), write_to=str(root / (name + ".png")))
print(f"Rendered {len(manifest['groups'])} {args.version} icons at 128 x 128.")
