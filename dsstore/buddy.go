package dsstore

import (
	"bytes"
	"io"
	"sort"
)

// buddyBuilder assembles a complete .DS_Store file from allocated blocks.
// It implements the macOS buddy allocator format with:
//   - Block 0 reserved for the bookkeeping block itself
//   - Proper free list tracking via buddy splitting
//   - 2^31 virtual address space (matching Finder's implementation)
type buddyBuilder struct {
	blocks         []buddyBlock
	toc            map[string]uint32 // name → block ID
	rootNodeBlockID uint32           // block ID of root B-tree node (for header field)
}

type buddyBlock struct {
	data []byte // actual block content
	size uint32 // power-of-2 allocation size (>= len(data))
}

// newBuddyBuilder creates a new builder.
func newBuddyBuilder() *buddyBuilder {
	return &buddyBuilder{
		toc: make(map[string]uint32),
	}
}

// allocate adds a block and returns its block ID.
// Block 0 is reserved for the bookkeeping block, so data blocks start at ID 1.
func (b *buddyBuilder) allocate(data []byte) uint32 {
	size := nextPow2(uint32(len(data)))
	if size < 32 {
		size = 32 // minimum block size
	}
	id := uint32(len(b.blocks)) + 1 // +1: block 0 = bookkeeping
	b.blocks = append(b.blocks, buddyBlock{data: data, size: size})
	return id
}

// setBlockData replaces the data for a previously allocated block.
func (b *buddyBuilder) setBlockData(id uint32, data []byte) {
	b.blocks[id-1].data = data // ID 1 = blocks[0]
}

// setTOC sets a table-of-contents entry mapping name to block ID.
func (b *buddyBuilder) setTOC(name string, blockID uint32) {
	b.toc[name] = blockID
}

// setRootNodeBlockID records the root B-tree node block ID.
// Finder stores this block's packed address in the Bud1 header at offset 16.
func (b *buddyBuilder) setRootNodeBlockID(id uint32) {
	b.rootNodeBlockID = id
}

// buddyAddressSpaceLog2 is the total virtual address space managed by the
// buddy allocator. Finder uses 2^31. Free blocks beyond the file size exist
// in virtual space and would cause file extension if allocated.
const buddyAddressSpaceLog2 = 31

// writeTo assembles the complete .DS_Store file and writes it to w.
//
// File layout:
//
//	[4 bytes]  Alignment magic: 0x00000001
//	[32 bytes] Bud1 header: magic + bookkeeping offset/size
//	[...]      Data blocks (power-of-2 aligned)
//	[...]      Bookkeeping block (block addresses + TOC + free lists)
func (b *buddyBuilder) writeTo(w io.Writer) error {
	totalBlocks := 1 + len(b.blocks) // block 0 (bookkeeping) + data blocks

	// Step 1: Lay out data blocks (IDs 1..N).
	// Offsets are relative to the allocator base (byte 4 of file).
	// The Bud1 header occupies offsets 0-31, so data starts at offset 32.
	layouts := make([]blockLayout, totalBlocks)
	currentOffset := uint32(32) // after Bud1 header

	for i, blk := range b.blocks {
		currentOffset = align(currentOffset, blk.size)
		layouts[i+1] = blockLayout{offset: currentOffset, size: blk.size}
		currentOffset += blk.size
	}

	// Step 2: Place bookkeeping block (block 0).
	bkAllocSize := uint32(2048) // minimum; always sufficient for typical .DS_Store
	bkOffset := align(currentOffset, bkAllocSize)
	layouts[0] = blockLayout{offset: bkOffset, size: bkAllocSize}

	// Step 3: Compute free lists by tracing buddy splits.
	// Include the Bud1 header (offset 0, size 32) as an implicit allocation.
	allAllocs := make([]blockLayout, totalBlocks+1)
	allAllocs[0] = blockLayout{offset: 0, size: 32} // Bud1 header
	for i := 0; i < totalBlocks; i++ {
		allAllocs[i+1] = layouts[i]
	}
	freeLists := computeFreeLists(allAllocs, buddyAddressSpaceLog2)

	// Step 4: Build bookkeeping block content.
	bookkeeping := b.buildBookkeeping(layouts, freeLists)

	// Step 5: Assemble the complete file.
	totalFileSize := 4 + bkOffset + bkAllocSize
	file := make([]byte, totalFileSize)

	// 4-byte alignment magic
	file[0] = 0x00
	file[1] = 0x00
	file[2] = 0x00
	file[3] = 0x01

	// Bud1 header at file offset 4 (allocator offset 0)
	hdr := new(bytes.Buffer)
	hdr.Write([]byte("Bud1"))        // magic
	writeU32(hdr, bkOffset)          // offset to bookkeeping block
	writeU32(hdr, bkAllocSize)       // size of bookkeeping block
	writeU32(hdr, bkOffset)          // copy of offset (for validation)

	// Offset 16: packed address of root B-tree node block.
	// Finder uses this for fast B-tree root lookup.
	if b.rootNodeBlockID > 0 && int(b.rootNodeBlockID) < len(layouts) {
		rootLayout := layouts[b.rootNodeBlockID]
		writeU32(hdr, rootLayout.offset|log2(rootLayout.size))
	} else {
		writeU32(hdr, 0)
	}
	hdr.Write(make([]byte, 12))      // remaining reserved bytes
	copy(file[4:], hdr.Bytes())

	// Write data blocks
	for i, blk := range b.blocks {
		fileOffset := layouts[i+1].offset + 4 // +4 for alignment magic
		copy(file[fileOffset:], blk.data)
	}

	// Write bookkeeping block
	copy(file[bkOffset+4:], bookkeeping)

	_, err := w.Write(file)
	return err
}

// buildBookkeeping builds the bookkeeping data block containing:
//   - Block count + unknown field
//   - Block addresses (padded to multiple of 256)
//   - Table of contents
//   - Free lists (32 buddy size classes)
func (b *buddyBuilder) buildBookkeeping(layouts []blockLayout, freeLists [32][]uint32) []byte {
	buf := new(bytes.Buffer)

	// Block count (includes block 0 = bookkeeping itself)
	blockCount := uint32(len(layouts))
	writeU32(buf, blockCount)

	// Unknown field (always 0)
	writeU32(buf, 0)

	// Block addresses: packed offset|log2(size), padded to 256 entries
	paddedCount := ((int(blockCount) + 255) / 256) * 256
	for i := 0; i < paddedCount; i++ {
		if i < len(layouts) {
			packed := layouts[i].offset | log2(layouts[i].size)
			writeU32(buf, packed)
		} else {
			writeU32(buf, 0) // unused padding
		}
	}

	// Table of contents (sorted for deterministic output)
	type tocEntry struct {
		name    string
		blockID uint32
	}
	var entries []tocEntry
	for name, id := range b.toc {
		entries = append(entries, tocEntry{name, id})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	writeU32(buf, uint32(len(entries)))
	for _, e := range entries {
		writeU8(buf, byte(len(e.name)))
		buf.WriteString(e.name)
		writeU32(buf, e.blockID)
	}

	// Free lists (32 size classes, one per power of 2)
	for i := 0; i < 32; i++ {
		list := freeLists[i]
		writeU32(buf, uint32(len(list)))
		for _, off := range list {
			writeU32(buf, off)
		}
	}

	return buf.Bytes()
}

// computeFreeLists computes the buddy allocator free lists by recursively
// splitting the address space. Each allocated block "consumes" a buddy slot;
// the remaining buddy halves at each split level become free list entries.
func computeFreeLists(allocs []blockLayout, totalSizeLog2 int) [32][]uint32 {
	var freeLists [32][]uint32
	buddyFree(0, totalSizeLog2, allocs, &freeLists)
	return freeLists
}

// buddyFree recursively splits the buddy block at (offset, 2^sizeLog2) and
// adds any completely unallocated sub-blocks to the appropriate free list.
func buddyFree(offset uint32, sizeLog2 int, allocs []blockLayout, free *[32][]uint32) {
	if sizeLog2 < 0 {
		return
	}

	blockSize := uint32(1) << sizeLog2
	blockEnd := offset + blockSize

	// Check for allocations overlapping this range
	hasOverlap := false
	for _, a := range allocs {
		aEnd := a.offset + a.size
		if a.offset < blockEnd && aEnd > offset {
			hasOverlap = true
			// Check for exact match (block fully allocated)
			if a.offset == offset && a.size == blockSize {
				return // fully allocated, nothing to free
			}
		}
	}

	if !hasOverlap {
		// Entirely free — add to free list
		free[sizeLog2] = append(free[sizeLog2], offset)
		return
	}

	// Partially allocated — split into buddy halves and recurse
	halfLog2 := sizeLog2 - 1
	halfSize := uint32(1) << halfLog2
	buddyFree(offset, halfLog2, allocs, free)
	buddyFree(offset+halfSize, halfLog2, allocs, free)
}

type blockLayout struct {
	offset uint32
	size   uint32
}
