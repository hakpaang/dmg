#!/Users/leaanthony/Downloads/go-dmg/.venv/bin/python3
"""Compare alias records and file headers between two .DS_Store files."""

import sys
import struct
import plistlib
from ds_store import DSStore

def decode_alias_full(data):
    """Decode alias and return structured data."""
    r = {}
    r['total_bytes'] = len(data)
    r['userType'] = struct.unpack('>I', data[0:4])[0]
    r['aliasSize'] = struct.unpack('>H', data[4:6])[0]
    r['version'] = struct.unpack('>H', data[6:8])[0]
    r['kind'] = struct.unpack('>H', data[8:10])[0]

    vn_len = data[10]
    r['volumeName'] = data[11:11+vn_len].decode('ascii', errors='replace')

    r['volCreDate'] = struct.unpack('>I', data[38:42])[0]
    r['volSig'] = struct.unpack('>H', data[42:44])[0]
    r['driveType'] = struct.unpack('>H', data[44:46])[0]
    r['parentCNID'] = struct.unpack('>I', data[46:50])[0]

    fn_len = data[50]
    r['fileName'] = data[51:51+fn_len].decode('ascii', errors='replace')

    r['fileCNID'] = struct.unpack('>I', data[114:118])[0]
    r['fileCreDate'] = struct.unpack('>I', data[118:122])[0]
    r['nlvlFrom'] = struct.unpack('>H', data[130:132])[0]
    r['nlvlTo'] = struct.unpack('>H', data[132:134])[0]

    # Tags
    tags = {}
    pos = 150
    while pos + 4 <= len(data):
        tag = struct.unpack('>H', data[pos:pos+2])[0]
        tlen = struct.unpack('>H', data[pos+2:pos+4])[0]
        if tag == 0xFFFF:
            break
        tag_data = data[pos+4:pos+4+tlen]
        tags[tag] = tag_data
        pos += 4 + tlen
        if tlen % 2 != 0:
            pos += 1
    r['tags'] = tags
    return r

def format_tag(tag_num, data):
    """Format a tag value for display."""
    if tag_num in (0x0000, 0x0012, 0x0013):
        return repr(data.decode('utf-8', errors='replace'))
    elif tag_num == 0x0001:
        cnids = []
        for i in range(0, len(data), 4):
            cnids.append(struct.unpack('>I', data[i:i+4])[0])
        return str(cnids)
    elif tag_num == 0x0002:
        return repr(data.decode('utf-8', errors='replace'))
    elif tag_num in (0x000E, 0x000F):
        count = struct.unpack('>H', data[0:2])[0]
        chars = []
        for i in range(count):
            chars.append(chr(struct.unpack('>H', data[2+i*2:4+i*2])[0]))
        return repr(''.join(chars))
    elif tag_num in (0x0010, 0x0011):
        val = struct.unpack('>Q', data)[0]
        return f"0x{val:016x} (hfs={val>>16})"
    else:
        return data.hex()

def get_alias_from_dsstore(path):
    """Extract the alias from icvp record."""
    with DSStore.open(path, 'r') as store:
        for entry in store:
            if entry.code == b'icvp' or entry.code == 'icvp':
                value = entry.value
                if isinstance(value, bytes):
                    pdata = plistlib.loads(value)
                elif isinstance(value, dict):
                    pdata = value
                else:
                    continue
                alias = pdata.get('backgroundImageAlias')
                if alias:
                    return alias
    return None

def compare_file_headers(path1, path2):
    """Compare the Bud1 headers of two .DS_Store files."""
    with open(path1, 'rb') as f:
        d1 = f.read()
    with open(path2, 'rb') as f:
        d2 = f.read()

    print(f"\n{'='*60}")
    print("FILE HEADERS")
    print(f"{'='*60}")
    print(f"  {'Field':<30} {'Golden':>15} {'Ours':>15}")
    print(f"  {'-----':<30} {'------':>15} {'----':>15}")
    print(f"  {'File size':<30} {len(d1):>15} {len(d2):>15}")

    magic1 = struct.unpack('>I', d1[0:4])[0]
    magic2 = struct.unpack('>I', d2[0:4])[0]
    print(f"  {'Alignment magic':<30} {hex(magic1):>15} {hex(magic2):>15}")

    print(f"  {'Buddy magic':<30} {d1[4:8].decode():>15} {d2[4:8].decode():>15}")

    bk_off1 = struct.unpack('>I', d1[8:12])[0]
    bk_off2 = struct.unpack('>I', d2[8:12])[0]
    print(f"  {'Bookkeeping offset':<30} {hex(bk_off1):>15} {hex(bk_off2):>15}")

    bk_sz1 = struct.unpack('>I', d1[12:16])[0]
    bk_sz2 = struct.unpack('>I', d2[12:16])[0]
    print(f"  {'Bookkeeping size':<30} {hex(bk_sz1):>15} {hex(bk_sz2):>15}")

    bk_off1c = struct.unpack('>I', d1[16:20])[0]
    bk_off2c = struct.unpack('>I', d2[16:20])[0]
    print(f"  {'Bookkeeping offset copy':<30} {hex(bk_off1c):>15} {hex(bk_off2c):>15}")

    h16_1 = struct.unpack('>I', d1[20:24])[0]
    h16_2 = struct.unpack('>I', d2[20:24])[0]
    off1 = h16_1 & ~0x1f
    sz1 = 1 << (h16_1 & 0x1f) if h16_1 else 0
    off2 = h16_2 & ~0x1f
    sz2 = 1 << (h16_2 & 0x1f) if h16_2 else 0
    print(f"  {'Header[16] raw':<30} {hex(h16_1):>15} {hex(h16_2):>15}")
    print(f"  {'  -> root node offset':<30} {hex(off1):>15} {hex(off2):>15}")
    print(f"  {'  -> root node size':<30} {sz1:>15} {sz2:>15}")

if __name__ == '__main__':
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} golden.dsstore ours.dsstore")
        sys.exit(1)

    path1, path2 = sys.argv[1], sys.argv[2]

    compare_file_headers(path1, path2)

    alias1 = get_alias_from_dsstore(path1)
    alias2 = get_alias_from_dsstore(path2)

    if not alias1:
        print(f"\nNo alias found in {path1}")
    if not alias2:
        print(f"\nNo alias found in {path2}")

    if alias1 and alias2:
        a1 = decode_alias_full(alias1)
        a2 = decode_alias_full(alias2)

        print(f"\n{'='*60}")
        print("ALIAS HEADER COMPARISON")
        print(f"{'='*60}")

        fields = ['total_bytes', 'aliasSize', 'version', 'kind', 'volumeName',
                  'volCreDate', 'volSig', 'driveType', 'parentCNID', 'fileName',
                  'fileCNID', 'fileCreDate', 'nlvlFrom', 'nlvlTo']

        for f in fields:
            v1 = a1.get(f)
            v2 = a2.get(f)
            if isinstance(v1, int) and v1 > 255:
                s1, s2 = hex(v1), hex(v2)
            else:
                s1, s2 = str(v1), str(v2)
            match = "OK" if v1 == v2 else "DIFFER"
            print(f"  {f:<20} {s1:>20} {s2:>20}  {match}")

        print(f"\n{'='*60}")
        print("ALIAS TAGS COMPARISON")
        print(f"{'='*60}")

        all_tags = sorted(set(list(a1['tags'].keys()) + list(a2['tags'].keys())))
        for tag in all_tags:
            d1 = a1['tags'].get(tag)
            d2 = a2['tags'].get(tag)
            tag_name = {0:'dirName', 1:'cnidPath', 2:'carbonPath',
                       0xe:'fileName16', 0xf:'volName16',
                       0x10:'hiResVolDate', 0x11:'hiResFileDate',
                       0x12:'posixPath', 0x13:'mountPoint'}.get(tag, f'tag_{tag}')

            if d1 is None:
                print(f"\n  Tag 0x{tag:04x} ({tag_name}): MISSING vs {format_tag(tag, d2)}")
            elif d2 is None:
                print(f"\n  Tag 0x{tag:04x} ({tag_name}): {format_tag(tag, d1)} vs MISSING")
            elif d1 == d2:
                print(f"\n  Tag 0x{tag:04x} ({tag_name}): MATCH = {format_tag(tag, d1)}")
            else:
                print(f"\n  Tag 0x{tag:04x} ({tag_name}): DIFFER")
                print(f"    Golden: {format_tag(tag, d1)}")
                print(f"    Ours:   {format_tag(tag, d2)}")
                print(f"    Golden hex: {d1.hex()}")
                print(f"    Ours hex:   {d2.hex()}")
