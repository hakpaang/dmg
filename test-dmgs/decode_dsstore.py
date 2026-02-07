#!/Users/leaanthony/Downloads/go-dmg/.venv/bin/python3
"""Decode and compare .DS_Store files using the ds_store Python library."""

import sys
import plistlib
import struct
from ds_store import DSStore

def decode_alias(data):
    """Decode an Alias v2 record."""
    if len(data) < 150:
        return f"[too short: {len(data)} bytes]"

    result = {}
    result['userType'] = struct.unpack('>I', data[0:4])[0]
    result['aliasSize'] = struct.unpack('>H', data[4:6])[0]
    result['version'] = struct.unpack('>H', data[6:8])[0]
    result['kind'] = struct.unpack('>H', data[8:10])[0]

    # Volume name (Pascal string at offset 10, 28 bytes)
    vol_name_len = data[10]
    result['volumeName'] = data[11:11+vol_name_len].decode('ascii', errors='replace')

    result['volCreationDate'] = struct.unpack('>I', data[38:42])[0]
    result['volSignature'] = hex(struct.unpack('>H', data[42:44])[0])
    result['driveType'] = struct.unpack('>H', data[44:46])[0]
    result['parentCNID'] = struct.unpack('>I', data[46:50])[0]

    # Filename (Pascal string at offset 50, 64 bytes)
    fname_len = data[50]
    result['fileName'] = data[51:51+fname_len].decode('ascii', errors='replace')

    result['fileCNID'] = struct.unpack('>I', data[114:118])[0]
    result['fileCreationDate'] = struct.unpack('>I', data[118:122])[0]
    result['fileType'] = hex(struct.unpack('>I', data[122:126])[0])
    result['fileCreator'] = hex(struct.unpack('>I', data[126:130])[0])
    result['nlvlFrom'] = struct.unpack('>H', data[130:132])[0]
    result['nlvlTo'] = struct.unpack('>H', data[132:134])[0]
    result['volAttributes'] = struct.unpack('>I', data[134:138])[0]
    result['volFSID'] = struct.unpack('>H', data[138:140])[0]

    # Tagged extra data starts at offset 150
    tags = []
    pos = 150
    while pos + 4 <= len(data):
        tag_type = struct.unpack('>H', data[pos:pos+2])[0]
        tag_len = struct.unpack('>H', data[pos+2:pos+4])[0]
        if tag_type == 0xFFFF:
            tags.append(('END', 0, b''))
            break
        tag_data = data[pos+4:pos+4+tag_len]
        tags.append((hex(tag_type), tag_len, tag_data))
        pos += 4 + tag_len
        if tag_len % 2 != 0:
            pos += 1  # padding

    result['tags'] = tags
    return result

def print_dsstore(path):
    """Print all records in a .DS_Store file."""
    print(f"\n{'='*60}")
    print(f"FILE: {path}")
    print(f"{'='*60}")

    with DSStore.open(path, 'r') as store:
        for entry in store:
            filename = entry.filename
            code = entry.code
            etype = entry.type
            value = entry.value

            if isinstance(value, bytes) and len(value) > 50:
                # Try to decode as plist
                try:
                    plist_data = plistlib.loads(value)
                    print(f"\n  {filename} / {code} ({etype}, {len(value)} bytes) = PLIST:")
                    for k, v in sorted(plist_data.items()):
                        if isinstance(v, bytes) and len(v) > 20:
                            print(f"    {k}: [{len(v)} bytes]")
                            if code == 'icvp' and k == 'backgroundImageAlias':
                                alias = decode_alias(v)
                                print(f"      === ALIAS DECODED ===")
                                for ak, av in alias.items():
                                    if ak == 'tags':
                                        print(f"      tags:")
                                        for tag_type, tag_len, tag_data in av:
                                            if tag_type == 'END':
                                                print(f"        END")
                                            else:
                                                # Try to show as string
                                                try:
                                                    s = tag_data.decode('utf-8', errors='replace')
                                                    print(f"        {tag_type} ({tag_len}b): {repr(s)}")
                                                except:
                                                    print(f"        {tag_type} ({tag_len}b): {tag_data.hex()}")
                                    else:
                                        print(f"      {ak}: {av}")
                        else:
                            print(f"    {k}: {v}")
                except:
                    print(f"\n  {filename} / {code} ({etype}, {len(value)} bytes) = {value.hex()[:80]}...")
            elif isinstance(value, bytes):
                if code == 'Iloc' and len(value) == 16:
                    x = struct.unpack('>I', value[0:4])[0]
                    y = struct.unpack('>I', value[4:8])[0]
                    print(f"\n  {filename} / {code} ({etype}) = ({x}, {y})")
                else:
                    print(f"\n  {filename} / {code} ({etype}, {len(value)} bytes) = {value.hex()}")
            else:
                print(f"\n  {filename} / {code} ({etype}) = {value}")

if __name__ == '__main__':
    for path in sys.argv[1:]:
        print_dsstore(path)
