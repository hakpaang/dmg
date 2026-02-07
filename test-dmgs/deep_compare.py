#!/Users/leaanthony/Downloads/go-dmg/.venv/bin/python3
"""Deep structural comparison of two .DS_Store files."""
import struct
import sys

def read_file(path):
    with open(path, 'rb') as f:
        return f.read()

def parse_header(data):
    """Parse the Bud1 header."""
    h = {}
    h['align_magic'] = struct.unpack('>I', data[0:4])[0]
    h['buddy_magic'] = data[4:8]
    h['bk_offset'] = struct.unpack('>I', data[8:12])[0]
    h['bk_size'] = struct.unpack('>I', data[12:16])[0]
    h['bk_offset_copy'] = struct.unpack('>I', data[16:20])[0]
    # Bytes 20-23: Header[16] in allocator-relative terms
    h['header16'] = struct.unpack('>I', data[20:24])[0]
    # Remaining header bytes (24-31)
    h['header_rest'] = data[24:36]
    return h

def parse_bookkeeping(data, bk_file_offset):
    """Parse the bookkeeping/allocator metadata section."""
    d = data[bk_file_offset:]
    bk = {}
    bk['block_count'] = struct.unpack('>I', d[0:4])[0]
    bk['unknown'] = struct.unpack('>I', d[4:8])[0]

    # Block address list
    count = bk['block_count']
    bk['block_addresses'] = []
    for i in range(count):
        off = 8 + i * 4
        addr = struct.unpack('>I', d[off:off+4])[0]
        block_offset = addr & ~0x1f
        block_log2 = addr & 0x1f
        block_size = 1 << block_log2 if addr else 0
        bk['block_addresses'].append({
            'raw': addr,
            'offset': block_offset,
            'log2': block_log2,
            'size': block_size,
        })

    # After block addresses: padding to 256*4 boundary, then TOC, then free lists
    # TOC starts after block address array (padded to 256 entries)
    toc_base = 8 + 256 * 4  # 1032
    bk['toc_count'] = struct.unpack('>I', d[toc_base:toc_base+4])[0]
    bk['toc_entries'] = []
    for i in range(bk['toc_count']):
        off = toc_base + 4 + i * 8
        name_len = d[off]
        name = d[off+1:off+1+name_len].decode('ascii', errors='replace')
        block_id = struct.unpack('>I', d[off+4:off+8])[0]
        bk['toc_entries'].append({'name': name, 'block_id': block_id})

    # Free lists follow TOC
    fl_base = toc_base + 4 + bk['toc_count'] * 8
    bk['free_lists'] = []
    for i in range(32):
        off = fl_base + i * 4
        if off + 4 <= len(d):
            count = struct.unpack('>I', d[off:off+4])[0]
            bk['free_lists'].append(count)

    return bk

def parse_dsdb(data, dsdb_offset):
    """Parse the DSDB master block."""
    d = data[dsdb_offset:]
    db = {}
    db['root_block_id'] = struct.unpack('>I', d[0:4])[0]
    db['tree_height'] = struct.unpack('>I', d[4:8])[0]
    db['record_count'] = struct.unpack('>I', d[8:12])[0]
    db['node_count'] = struct.unpack('>I', d[12:16])[0]
    db['page_size'] = struct.unpack('>I', d[16:20])[0]
    return db

def parse_btree_node(data, node_offset, node_size):
    """Parse a B-tree node."""
    d = data[node_offset:node_offset+node_size]
    node = {}
    node['mode'] = struct.unpack('>I', d[0:4])[0]
    node['count'] = struct.unpack('>I', d[4:8])[0]
    node['records'] = []

    pos = 8
    for i in range(node['count']):
        if node['mode'] > 0:
            # Internal node: child pointer before each record
            child = struct.unpack('>I', d[pos:pos+4])[0]
            pos += 4

        rec = {}
        # Filename
        if pos + 4 > len(d):
            break
        name_len = struct.unpack('>I', d[pos:pos+4])[0]
        pos += 4
        name_chars = []
        for j in range(name_len):
            if pos + 2 > len(d):
                break
            ch = struct.unpack('>H', d[pos:pos+2])[0]
            name_chars.append(chr(ch))
            pos += 2
        rec['filename'] = ''.join(name_chars)

        # Code
        if pos + 4 > len(d):
            break
        rec['code'] = d[pos:pos+4].decode('ascii', errors='replace')
        pos += 4

        # Data type
        if pos + 4 > len(d):
            break
        rec['dtype'] = d[pos:pos+4].decode('ascii', errors='replace')
        pos += 4

        # Value
        if rec['dtype'] == 'blob':
            if pos + 4 > len(d):
                break
            blob_len = struct.unpack('>I', d[pos:pos+4])[0]
            pos += 4
            rec['value_size'] = blob_len
            rec['value'] = d[pos:pos+blob_len]
            pos += blob_len
        elif rec['dtype'] == 'long':
            rec['value'] = struct.unpack('>I', d[pos:pos+4])[0]
            pos += 4
        elif rec['dtype'] == 'shor':
            rec['value'] = struct.unpack('>I', d[pos:pos+4])[0]
            pos += 4
        elif rec['dtype'] == 'bool':
            rec['value'] = d[pos]
            pos += 1
        elif rec['dtype'] == 'type':
            rec['value'] = d[pos:pos+4].decode('ascii', errors='replace')
            pos += 4
        else:
            rec['value'] = f'(unknown dtype: {rec["dtype"]})'

        node['records'].append(rec)

    return node

def main():
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} file1 file2")
        sys.exit(1)

    for path in sys.argv[1:]:
        data = read_file(path)
        h = parse_header(data)
        print(f"\n{'='*70}")
        print(f"FILE: {path} ({len(data)} bytes)")
        print(f"{'='*70}")

        print(f"\nHEADER:")
        print(f"  Buddy magic: {h['buddy_magic']}")
        print(f"  Bookkeeping offset: {h['bk_offset']} (file: {h['bk_offset']+4})")
        print(f"  Bookkeeping size: {h['bk_size']}")
        h16 = h['header16']
        print(f"  Header[16]: 0x{h16:08x} -> offset={h16 & ~0x1f}, size={1<<(h16&0x1f) if h16 else 0}")

        bk_file = h['bk_offset'] + 4
        bk = parse_bookkeeping(data, bk_file)

        print(f"\nBOOKKEEPING:")
        print(f"  Block count: {bk['block_count']}")
        for i, ba in enumerate(bk['block_addresses'][:bk['block_count']]):
            print(f"  Block {i}: raw=0x{ba['raw']:08x} offset=0x{ba['offset']:04x} size={ba['size']}")
        print(f"  TOC entries: {bk['toc_count']}")
        for te in bk['toc_entries']:
            print(f"    {te['name']}: block_id={te['block_id']}")

        # Non-zero free lists
        print(f"  Free lists (non-zero):")
        for i, fl in enumerate(bk['free_lists']):
            if fl > 0:
                print(f"    [{i}] (size {1<<i}): {fl} entries")

        # Get DSDB block
        dsdb_block_id = None
        for te in bk['toc_entries']:
            if te['name'] == 'DSDB':
                dsdb_block_id = te['block_id']
        if dsdb_block_id is not None and dsdb_block_id < bk['block_count']:
            ba = bk['block_addresses'][dsdb_block_id]
            dsdb_file_offset = ba['offset'] + 4
            db = parse_dsdb(data, dsdb_file_offset)
            print(f"\nDSDB MASTER:")
            print(f"  Root block ID: {db['root_block_id']}")
            print(f"  Tree height: {db['tree_height']}")
            print(f"  Record count: {db['record_count']}")
            print(f"  Node count: {db['node_count']}")
            print(f"  Page size: {db['page_size']}")

            # Parse root node
            root_id = db['root_block_id']
            if root_id < bk['block_count']:
                rba = bk['block_addresses'][root_id]
                node_file_offset = rba['offset'] + 4
                node = parse_btree_node(data, node_file_offset, rba['size'])
                print(f"\nROOT B-TREE NODE (block {root_id}, offset 0x{rba['offset']:04x}, size {rba['size']}):")
                print(f"  Mode: {node['mode']} ({'leaf' if node['mode']==0 else 'internal'})")
                print(f"  Record count: {node['count']}")
                for rec in node['records']:
                    code = rec['code']
                    fname = rec['filename']
                    if isinstance(rec.get('value'), bytes) and len(rec['value']) > 20:
                        print(f"  [{fname}] {code} ({rec['dtype']}, {rec.get('value_size', '?')} bytes)")
                    elif isinstance(rec.get('value'), bytes):
                        print(f"  [{fname}] {code} ({rec['dtype']}) = {rec['value'].hex()}")
                    else:
                        print(f"  [{fname}] {code} ({rec['dtype']}) = {rec.get('value')}")

if __name__ == '__main__':
    main()
