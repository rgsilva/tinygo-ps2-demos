#!/bin/bash
# One-time setup of PCSX2 for the headless harness.
#
#   harness/setup-pcsx2.sh [PCSX2_DIR] [BIOS_FILE]
#
# PCSX2_DIR  where to install (default ~/dev/ps2go/tools/pcsx2, or $PS2GO_PCSX2_DIR)
# BIOS_FILE  a PS2 BIOS dump to copy into the data directory (or $PS2_BIOS).
#            PCSX2 needs one even for homebrew ELFs. Never commit it anywhere.
#
# Needs: Xvfb, python3, and the Mesa GL libraries (installed with apt below).
set -euo pipefail

VERSION=v2.9.12
URL="https://github.com/PCSX2/pcsx2/releases/download/$VERSION/pcsx2-$VERSION-linux-appimage-x64-Qt.AppImage"
DIR=${1:-${PS2GO_PCSX2_DIR:-$HOME/dev/ps2go/tools/pcsx2}}
BIOS=${2:-${PS2_BIOS:-}}
HERE=$(cd "$(dirname "$0")" && pwd)

mkdir -p "$DIR"
cd "$DIR"

if ! [ -x squashfs-root/usr/bin/pcsx2-qt ]; then
    APP=$(basename "$URL")
    [ -f "$APP" ] || { echo "downloading $URL"; curl -fL -o "$APP" "$URL"; }
    chmod +x "$APP"
    rm -rf squashfs-root
    ./"$APP" --appimage-extract > /dev/null
fi
echo "pcsx2: $(squashfs-root/usr/bin/pcsx2-qt -version 2>/dev/null | head -1 || echo installed)"

if command -v apt-get > /dev/null; then
    missing=$(for p in libopengl0 libegl1 libegl-mesa0 xvfb; do dpkg -s $p > /dev/null 2>&1 || echo $p; done)
    if [ -n "$missing" ]; then
        echo "installing:$missing"
        sudo apt-get install -y $missing
    fi
fi

mkdir -p data/PCSX2/inis data/PCSX2/bios
# Let PCSX2 write its complete default ini (a partial one makes it sit in the
# setup wizard), then apply the harness settings on top.
if ! [ -f data/PCSX2/inis/PCSX2.ini ]; then
    xvfb-run -a squashfs-root/usr/bin/pcsx2-qt -datapath "$DIR/data" -testconfig > /dev/null 2>&1 || true
    [ -f data/PCSX2/inis/PCSX2.ini ] || { echo "PCSX2 did not create data/PCSX2/inis/PCSX2.ini"; exit 1; }
fi
python3 - "$HERE/PCSX2.ini" data/PCSX2/inis/PCSX2.ini <<'PY'
import configparser, sys
def load(path):
    c = configparser.RawConfigParser(strict=False, delimiters=("=",), comment_prefixes=(";", "#"))
    c.optionxform = str
    c.read(path)
    return c
ours, theirs = load(sys.argv[1]), load(sys.argv[2])
for section in ours.sections():
    if not theirs.has_section(section):
        theirs.add_section(section)
    for key, value in ours.items(section):
        theirs.set(section, key, value)
with open(sys.argv[2], "w") as f:
    theirs.write(f, space_around_delimiters=True)
PY
if [ -n "$BIOS" ]; then
    cp "$BIOS" data/PCSX2/bios/
fi
if ! ls data/PCSX2/bios/* > /dev/null 2>&1; then
    echo "note: no BIOS in $DIR/data/PCSX2/bios; pass one as the second argument or copy it there"
fi
echo "done: PS2GO_PCSX2_DIR=$DIR"
