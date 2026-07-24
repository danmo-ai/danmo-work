#!/usr/bin/env python3
"""Generate Danmo Work desktop icon."""
from PIL import Image, ImageDraw, ImageFilter
import math, os

SIZE = 1024

# --- Gradient background ---
bg = Image.new("RGBA", (SIZE, SIZE))
bd = ImageDraw.Draw(bg)
for y in range(SIZE):
    t = y / SIZE
    r = int(30 + 30 * t)
    g = int(18 + 15 * t)
    b = int(110 + 70 * t)
    bd.line([(0, y), (SIZE - 1, y)], fill=(r, g, b, 255))

# Diagonal warmth overlay
warm = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
wd = ImageDraw.Draw(warm)
for x in range(0, SIZE, 2):
    s = x / SIZE
    wd.line([(x, 0), (x, SIZE - 1)], fill=(int(50 * s), int(20 * s), int(40 * (1 - s)), 55))
bg = Image.alpha_composite(bg, warm)

# --- Ambient glow ---
glow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
c1 = Image.new("RGBA", (600, 600), (0, 0, 0, 0))
ImageDraw.Draw(c1).ellipse([0, 0, 599, 599], fill=(255, 130, 60, 22))
c1 = c1.filter(ImageFilter.GaussianBlur(80))
glow.paste(c1, (SIZE - 400, -150), c1)
c2 = Image.new("RGBA", (500, 500), (0, 0, 0, 0))
ImageDraw.Draw(c2).ellipse([0, 0, 499, 499], fill=(60, 190, 255, 18))
c2 = c2.filter(ImageFilter.GaussianBlur(70))
glow.paste(c2, (-80, SIZE - 350), c2)
img = Image.alpha_composite(bg, glow)
draw = ImageDraw.Draw(img)

# --- Nodes ---
cx, cy = SIZE // 2, SIZE // 2
main_r = 130
orbit_r = 68
orbit_dist = 245

# Connection lines
lines = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
ld = ImageDraw.Draw(lines)
for angle_deg in [90, 210, 330]:
    a = math.radians(angle_deg)
    ox = cx + int(orbit_dist * math.cos(a))
    oy = cy - int(orbit_dist * math.sin(a))
    ld.line([(cx, cy), (ox, oy)], fill=(255, 255, 255, 35), width=14)
lines = lines.filter(ImageFilter.GaussianBlur(4))
img = Image.alpha_composite(img, lines)
draw = ImageDraw.Draw(img)

def draw_node(base, bx, by, radius, color):
    """Draw a node directly on the base image."""
    d = ImageDraw.Draw(base)
    # Outer glow rings (drawn directly)
    for ring in range(radius + 40, radius, -3):
        t = (ring - radius) / 40
        alpha = int(30 * (1 - t) ** 2)
        d.ellipse(
            [bx - ring, by - ring, bx + ring, by + ring],
            fill=color[:3] + (alpha,),
        )
    # Solid core
    d.ellipse([bx - radius, by - radius, bx + radius, by + radius], fill=color)
    # Inner highlight: small white ellipse at upper-left
    hl_r = int(radius * 0.3)
    hlx = bx - int(radius * 0.25)
    hly = by - int(radius * 0.3)
    d.ellipse([hlx - hl_r, hly - hl_r, hlx + hl_r, hly + hl_r], fill=(255, 255, 255, 55))
    # Soften highlight with a slightly larger, more transparent ring
    d.ellipse([hlx - hl_r - 5, hly - hl_r - 5, hlx + hl_r + 5, hly + hl_r + 5], fill=(255, 255, 255, 15))

# Central node
draw_node(img, cx, cy, main_r, (225, 235, 255, 255))

# Orbiting nodes
node_colors = [
    (255, 155, 85, 255),
    (90, 215, 195, 255),
    (195, 135, 255, 255),
]
for i, angle_deg in enumerate([90, 210, 330]):
    a = math.radians(angle_deg)
    ox = cx + int(orbit_dist * math.cos(a))
    oy = cy - int(orbit_dist * math.sin(a))
    draw_node(img, ox, oy, orbit_r, node_colors[i])

# --- macOS rounded-rect mask ---
mask = Image.new("L", (SIZE, SIZE), 0)
ImageDraw.Draw(mask).rounded_rectangle([0, 0, SIZE - 1, SIZE - 1], radius=int(SIZE * 0.22), fill=255)
img.putalpha(mask)

# --- Save ---
out = os.path.dirname(os.path.abspath(__file__))
path = os.path.join(out, "icon_master.png")
img.save(path, "PNG")
print(f"Saved {path}")
