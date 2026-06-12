#!/usr/bin/env python3
"""Generate all platform icons from the master 1024x1024 appicon.png."""

import struct
import io
import os
import subprocess
import shutil
from PIL import Image

SRC = "build/appicon.png"
OUT_DIR = "build"

ICON_SIZES = {
    "appicon.png": 1024,
}

# macOS iconset sizes (for iconutil)
MAC_ICONSET = [
    (16, 16, "icon_16x16.png"),
    (32, 32, "icon_16x16@2x.png"),
    (32, 32, "icon_32x32.png"),
    (64, 64, "icon_32x32@2x.png"),
    (128, 128, "icon_128x128.png"),
    (256, 256, "icon_128x128@2x.png"),
    (256, 256, "icon_256x256.png"),
    (512, 512, "icon_256x256@2x.png"),
    (512, 512, "icon_512x512.png"),
    (1024, 1024, "icon_512x512@2x.png"),
]

# iOS icon sizes (from build/ios/icon.png)
IOS_SIZES = [
    (40, 40, "icon-40.png"),
    (60, 60, "icon-60.png"),
    (58, 58, "icon-58.png"),
    (87, 87, "icon-87.png"),
    (80, 80, "icon-80.png"),
    (120, 120, "icon-120.png"),
    (180, 180, "icon-180.png"),
    (20, 20, "icon-20.png"),
    (1024, 1024, "icon.png"),
]

# Windows ICO sizes
ICO_SIZES = [256, 128, 64, 48, 32, 16]

# Android icon sizes
ANDROID_SIZES = {
    "mipmap-mdpi": 48,
    "mipmap-hdpi": 72,
    "mipmap-xhdpi": 96,
    "mipmap-xxhdpi": 144,
    "mipmap-xxxhdpi": 192,
}


def resize(img, size):
    """Resize image to (size, size) using high-quality Lanczos."""
    return img.resize((size, size), Image.LANCZOS)


def write_ico(images, output_path):
    """Write a multi-resolution ICO file from a list of PIL Images."""
    # ICO header
    header = struct.pack("<HHH", 0, 1, len(images))  # reserved, type=1(ico), count

    # Compute offsets
    data_offset = 6 + 16 * len(images)
    entries = []
    data_chunks = []

    for img in images:
        # Convert to BGRA (Windows format) and ensure 32-bit
        img = img.convert("RGBA")
        w, h = img.size
        
        # BMP info header (40 bytes)
        bmp_header = struct.pack(
            "<IiiHHIIiiII",
            40,           # biSize
            w,            # biWidth
            h * 2,        # biHeight (double for ICO)
            1,            # biPlanes
            32,           # biBitCount
            0,            # biCompression
            0,            # biSizeImage
            0,            # biXPelsPerMeter
            0,            # biYPelsPerMeter
            0,            # biClrUsed
            0,            # biClrImportant
        )

        # Pixel data (BGRA, bottom-up, XOR mask)
        pixels = bytearray()
        # ICO expects bottom-up rows
        for y in range(h - 1, -1, -1):
            for x in range(w):
                r, g, b, a = img.getpixel((x, y))
                pixels.extend([b, g, r, a])

        # AND mask (1 = transparent, 0 = opaque) - all zeros since we use alpha channel
        and_mask_size = ((w + 31) // 32) * 4 * h
        and_mask = bytearray(and_mask_size)

        image_data = bmp_header + bytes(pixels) + bytes(and_mask)

        # ICO directory entry
        # If w >= 256, use 0 (256+ uses 0 as size indicator)
        w_entry = w if w < 256 else 0
        h_entry = h if h < 256 else 0
        entry = struct.pack(
            "<BBBBHHII",
            w_entry,      # width
            h_entry,      # height
            0,            # color palette count
            0,            # reserved
            1,            # color planes
            32,           # bits per pixel
            len(image_data),  # size
            data_offset,  # offset in file
        )
        entries.append(entry)
        data_chunks.append(image_data)
        data_offset += len(image_data)

    with open(output_path, "wb") as f:
        f.write(header)
        for e in entries:
            f.write(e)
        for d in data_chunks:
            f.write(d)

    print(f"  ✓ {output_path} ({len(images)} resolutions)")


def generate_macos_icons(img):
    """Generate macOS .icns via iconset + iconutil."""
    iconset_dir = os.path.join(OUT_DIR, "Zashiki.iconset")
    if os.path.exists(iconset_dir):
        shutil.rmtree(iconset_dir)
    os.makedirs(iconset_dir, exist_ok=True)

    for w, h, name in MAC_ICONSET:
        resized = resize(img, w)
        path = os.path.join(iconset_dir, name)
        resized.save(path)
        print(f"  → {path} ({w}×{h})")

    # Convert to icns using iconutil
    icns_path = os.path.join(OUT_DIR, "darwin", "icons.icns")
    os.makedirs(os.path.dirname(icns_path), exist_ok=True)
    
    result = subprocess.run(
        ["iconutil", "-c", "icns", iconset_dir, "-o", icns_path],
        capture_output=True, text=True
    )
    if result.returncode == 0:
        print(f"  ✓ {icns_path}")
    else:
        print(f"  ✗ iconutil error: {result.stderr}")

    # Clean up iconset
    shutil.rmtree(iconset_dir)


def generate_windows_ico(img):
    """Generate Windows .ico with multiple resolutions."""
    images = []
    for size in ICO_SIZES:
        w = size if size <= 256 else 0
        images.append(resize(img, size))
    
    ico_path = os.path.join(OUT_DIR, "windows", "icon.ico")
    os.makedirs(os.path.dirname(ico_path), exist_ok=True)
    write_ico(images, ico_path)


def generate_ios_icons(img):
    """Generate iOS icons."""
    ios_dir = os.path.join(OUT_DIR, "ios")
    os.makedirs(ios_dir, exist_ok=True)
    
    for w, h, name in IOS_SIZES:
        resized = resize(img, w)
        path = os.path.join(ios_dir, name)
        resized.save(path)
        print(f"  → {path} ({w}×{h})")


def generate_android_icons(img):
    """Generate Android mipmap icons for all densities."""
    base = os.path.join(OUT_DIR, "android", "app", "src", "main", "res")
    
    for density, size in ANDROID_SIZES.items():
        resized = resize(img, size)
        
        # Regular icon
        path = os.path.join(base, density, "ic_launcher.png")
        os.makedirs(os.path.dirname(path), exist_ok=True)
        resized.save(path)
        
        # Round icon (same image, Android will mask it)
        path_round = os.path.join(base, density, "ic_launcher_round.png")
        resized.save(path_round)
        
        print(f"  → {density}/ic_launcher.png ({size}×{size})")


def generate_web_favicons(img):
    """Generate web favicon (ico + SVG)."""
    # Use the SVG directly as favicon reference
    # Also generate a small ico for the web
    small = resize(img, 32)
    write_ico([small], os.path.join("frontend", "public", "favicon.ico"))
    
    # Also copy SVG as favicon alternative
    svg_src = "build/appicon.icon/Assets/zashiki_icon.svg"
    svg_dst = os.path.join("frontend", "public", "favicon.svg")
    shutil.copy2(svg_src, svg_dst)
    print(f"  ✓ frontend/public/favicon.ico")
    print(f"  ✓ frontend/public/favicon.svg")


def main():
    print("🎨 Generating Zashiki app icons from master PNG...")
    
    if not os.path.exists(SRC):
        # Also try sips conversion
        print("Master PNG not found, creating from SVG...")
        subprocess.run([
            "sips", "-s", "format", "png",
            "build/appicon.icon/Assets/zashiki_icon.svg",
            "--out", SRC
        ], check=True)
        print(f"  ✓ {SRC}")

    img = Image.open(SRC).convert("RGBA")
    print(f"  Master: {img.size[0]}×{img.size[1]}\n")

    print("📱 macOS icons...")
    generate_macos_icons(img)
    print()

    print("🪟 Windows icons...")
    generate_windows_ico(img)
    print()

    print("🍎 iOS icons...")
    generate_ios_icons(img)
    print()

    print("🤖 Android icons...")
    generate_android_icons(img)
    print()

    print("🌐 Web favicons...")
    generate_web_favicons(img)
    print()

    print("✅ All icons generated!")


if __name__ == "__main__":
    main()
