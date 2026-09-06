#!/usr/bin/env python3
"""Write a bootable PlayStation 2 CD image: an ISO 9660 level 1 filesystem
with SYSTEM.CNF (BOOT2 pointing at the ELF) and the ELF, plus any extra
files, all in the root directory with 8.3 uppercase names. The ELF is laid
out first on the disc. Pure Python, no dependencies.

    mkiso.py [-v VOLUME] [-n NAME.ELF] [-x FILE ...] OUT.iso PROGRAM.elf
    mkiso.py --list IMAGE.iso        # print the root directory (self-check)

The first 16 sectors, where retail discs carry Sony's license data, are
left zero: a modchip (or an emulator) does not need them.
"""
import argparse
import os
import re
import struct
import sys
import time

SECTOR = 2048
SYSTEM_ID = b"PLAYSTATION"


def both16(n):
    return struct.pack("<H", n) + struct.pack(">H", n)


def both32(n):
    return struct.pack("<I", n) + struct.pack(">I", n)


def pad(data, size, fill=b" "):
    return data[:size] + fill * (size - len(data))


def sectors(n):
    return (n + SECTOR - 1) // SECTOR


def iso_name(name):
    """8.3 uppercase identifier for a host file name."""
    base, dot, ext = os.path.basename(name).upper().rpartition(".")
    if not dot:
        base, ext = ext, ""
    base = re.sub(r"[^A-Z0-9_]", "_", base)[:8]
    ext = re.sub(r"[^A-Z0-9_]", "_", ext)[:3]
    return base + "." + ext if ext else base


def dir_record(name, extent, size, when, is_dir=False):
    ident = name if is_dir else name + b";1"
    rec = bytearray()
    rec += b"\x00"  # length, patched below
    rec += b"\x00"  # extended attribute record length
    rec += both32(extent)
    rec += both32(size)
    rec += bytes([when.tm_year - 1900, when.tm_mon, when.tm_mday, when.tm_hour, when.tm_min, when.tm_sec, 0])
    rec += bytes([2 if is_dir else 0])  # flags
    rec += b"\x00\x00"  # unit size, interleave gap
    rec += both16(1)  # volume sequence number
    rec += bytes([len(ident)]) + ident
    if len(rec) % 2:
        rec += b"\x00"
    rec[0] = len(rec)
    return bytes(rec)


def path_table(root_extent, big):
    e = ">" if big else "<"
    return bytes([1, 0]) + struct.pack(e + "I", root_extent) + struct.pack(e + "H", 1) + b"\x00\x00"


def build(out, elf, extras, volume, elf_name):
    when = time.gmtime()
    elf_name = elf_name.upper().encode()
    if not re.fullmatch(rb"[A-Z0-9_]{1,8}\.[A-Z0-9_]{1,3}", elf_name):
        sys.exit(f"{elf_name.decode()}: not an 8.3 name")
    cnf = b"BOOT2 = cdrom0:\\" + elf_name + b";1\r\nVER = 1.00\r\nVMODE = NTSC\r\n"

    # (identifier, data) in disc order: the ELF first, then SYSTEM.CNF, then
    # the extras. The directory lists them sorted by identifier, as ISO 9660
    # requires; the data order is free.
    files = [(elf_name, open(elf, "rb").read()), (b"SYSTEM.CNF", cnf)]
    for path in extras:
        files.append((iso_name(path).encode(), open(path, "rb").read()))
    names = [f[0] for f in files]
    if len(set(names)) != len(names):
        sys.exit("duplicate 8.3 names: " + ", ".join(n.decode() for n in names))

    # Layout: 0-15 system area, 16 PVD, 17 terminator, 18 path table (L),
    # 19 path table (M), 20 root directory, 21.. file data.
    pvd_sector, term_sector, ptl_sector, ptm_sector, root_sector = 16, 17, 18, 19, 20
    extent = root_sector + 1
    placed = []
    for name, data in files:
        placed.append((name, extent, len(data)))
        extent += max(sectors(len(data)), 1)
    total = extent

    root = dir_record(b"\x00", root_sector, SECTOR, when, True) + dir_record(b"\x01", root_sector, SECTOR, when, True)
    for name, ext, size in sorted(placed):
        root += dir_record(name, ext, size, when)
    if len(root) > SECTOR:
        sys.exit("too many files for a one-sector root directory")
    root = pad(root, SECTOR, b"\x00")

    root_rec = dir_record(b"\x00", root_sector, SECTOR, when, True)
    date = pad(b"", 16, b"0") + b"\x00"
    pvd = bytearray()
    pvd += b"\x01CD001\x01\x00"
    pvd += pad(SYSTEM_ID, 32)
    pvd += pad(volume.upper().encode()[:32], 32)
    pvd += b"\x00" * 8
    pvd += both32(total)
    pvd += b"\x00" * 32
    pvd += both16(1)  # volume set size
    pvd += both16(1)  # volume sequence number
    pvd += both16(SECTOR)
    pvd += both32(len(path_table(root_sector, False)))
    pvd += struct.pack("<I", ptl_sector) + struct.pack("<I", 0)  # L path table, optional
    pvd += struct.pack(">I", ptm_sector) + struct.pack(">I", 0)  # M path table, optional
    pvd += root_rec
    pvd += pad(b"", 128)  # volume set id
    pvd += pad(b"", 128)  # publisher
    pvd += pad(b"", 128)  # data preparer
    pvd += pad(b"PS2GO", 128)  # application
    pvd += pad(b"", 37) * 3  # copyright, abstract, bibliographic file ids
    pvd += date * 4  # creation, modification, expiration, effective
    pvd += b"\x01\x00"  # file structure version, reserved
    pvd = pad(bytes(pvd), SECTOR, b"\x00")
    assert len(root_rec) == 34

    with open(out, "wb") as f:
        f.write(b"\x00" * (pvd_sector * SECTOR))
        f.write(pvd)
        f.write(pad(b"\xffCD001\x01", SECTOR, b"\x00"))
        f.write(pad(path_table(root_sector, False), SECTOR, b"\x00"))
        f.write(pad(path_table(root_sector, True), SECTOR, b"\x00"))
        f.write(root)
        for (name, data), (_, ext, _) in zip(files, placed):
            assert f.tell() == ext * SECTOR
            f.write(pad(data, max(sectors(len(data)), 1) * SECTOR, b"\x00"))
        assert f.tell() == total * SECTOR
    return total


def listing(path):
    """The root directory of an ISO 9660 image, as (name, extent, size)."""
    with open(path, "rb") as f:
        f.seek(16 * SECTOR)
        pvd = f.read(SECTOR)
        if pvd[:6] != b"\x01CD001":
            sys.exit(f"{path}: no primary volume descriptor")
        system_id = pvd[8:40].strip().decode()
        volume = pvd[40:72].strip().decode()
        total, = struct.unpack_from("<I", pvd, 80)
        root_extent, = struct.unpack_from("<I", pvd, 156 + 2)
        root_size, = struct.unpack_from("<I", pvd, 156 + 10)
        f.seek(root_extent * SECTOR)
        root = f.read(root_size)
    entries, pos = [], 0
    while pos < len(root) and root[pos]:
        length = root[pos]
        extent, = struct.unpack_from("<I", root, pos + 2)
        size, = struct.unpack_from("<I", root, pos + 10)
        flags = root[pos + 25]
        n = root[pos + 32]
        name = root[pos + 33:pos + 33 + n]
        if name not in (b"\x00", b"\x01"):
            entries.append((name.decode().split(";")[0], extent, size, flags))
        pos += length
    return system_id, volume, total, entries


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("out", help="output image, or the image to list with --list")
    ap.add_argument("elf", nargs="?", help="the program (ELF)")
    ap.add_argument("-v", "--volume", help="volume identifier (default: the ELF's name)")
    ap.add_argument("-n", "--name", help="8.3 name of the ELF on the disc (default: from the file name)")
    ap.add_argument("-x", "--extra", action="append", default=[], help="extra file to add (8.3 name from its file name)")
    ap.add_argument("--list", action="store_true", help="print the root directory of an image")
    args = ap.parse_args()
    if args.list:
        system_id, volume, total, entries = listing(args.out)
        print(f"{args.out}: system {system_id!r} volume {volume!r} {total} sectors ({total * SECTOR // 1024} KB)")
        for name, extent, size, flags in entries:
            print(f"  {name:<14} sector {extent:<6} {size} bytes{' (dir)' if flags & 2 else ''}")
        return
    if not args.elf:
        ap.error("the ELF is required")
    name = args.name or iso_name(args.elf)
    volume = args.volume or os.path.splitext(os.path.basename(args.elf))[0]
    total = build(args.out, args.elf, args.extra, volume, name)
    print(f"{args.out}: {total} sectors ({total * SECTOR // 1024} KB), boots {name}")


if __name__ == "__main__":
    main()
