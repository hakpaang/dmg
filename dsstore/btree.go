package dsstore

import (
	"bytes"
)

const pageSize = 0x1000 // 4096 bytes per B-tree node

// btreeResult holds the serialized B-tree: a master block and node blocks.
type btreeResult struct {
	masterBlock []byte   // 20-byte DSDB master block
	nodeBlocks  [][]byte // serialized node blocks (indices are block IDs)
}

// buildBTree constructs a B-tree from a sorted slice of records.
// Records MUST be sorted via Record.Less before calling this function.
//
// For a typical DMG with ~10 records, everything fits in a single leaf node.
// Multi-leaf trees are supported for larger stores.
func buildBTree(records []Record) btreeResult {
	if len(records) == 0 {
		leaf := serializeLeafNode(nil)
		master := serializeMasterBlock(0, 0, 0, 1)
		return btreeResult{masterBlock: master, nodeBlocks: [][]byte{leaf}}
	}

	// Step 1: Pack records into leaf nodes
	leaves := buildLeafNodes(records)

	// Step 2: Single leaf — simplest case
	if len(leaves) == 1 {
		master := serializeMasterBlock(0, 0, uint32(len(records)), 1)
		return btreeResult{masterBlock: master, nodeBlocks: [][]byte{leaves[0].data}}
	}

	// Step 3: Multiple leaves — build internal node levels
	// All node blocks are collected here. Indices serve as block IDs.
	var allBlocks [][]byte
	for _, l := range leaves {
		allBlocks = append(allBlocks, l.data)
	}

	// Current level's block IDs and separator records
	type nodeRef struct {
		blockID   uint32
		firstRec  Record // first record in this node (used as separator)
	}

	currentLevel := make([]nodeRef, len(leaves))
	for i, l := range leaves {
		currentLevel[i] = nodeRef{
			blockID:  uint32(i),
			firstRec: l.firstRecord,
		}
	}

	height := uint32(0)
	for len(currentLevel) > 1 {
		height++
		var nextLevel []nodeRef

		// Build internal nodes for this level
		// Each internal node can hold: 8 bytes header + entries
		// Entry = 4 bytes child pointer + separator record
		var children []uint32
		var separators []Record
		nodeHeaderSize := 8 // mode(4) + count(4)
		currentSize := nodeHeaderSize

		for i := 0; i < len(currentLevel); i++ {
			if i == 0 {
				// First child has no separator before it
				children = append(children, currentLevel[i].blockID)
				currentSize += 4 // child pointer
				continue
			}

			sep := currentLevel[i].firstRec
			entrySize := 4 + sep.ByteLength() // child pointer + separator

			if currentSize+entrySize > pageSize && len(children) > 1 {
				// Flush current internal node
				rightmost := children[len(children)-1]
				internalData := serializeInternalNode(
					children[:len(children)-1],
					separators,
					rightmost,
				)
				blockID := uint32(len(allBlocks))
				allBlocks = append(allBlocks, internalData)

				// This internal node's "first record" for the next level
				// is the first separator (or the first record of its first child)
				var firstRec Record
				if len(separators) > 0 {
					firstRec = separators[0]
				}
				nextLevel = append(nextLevel, nodeRef{blockID: blockID, firstRec: firstRec})

				// Start new internal node with current child
				children = []uint32{currentLevel[i].blockID}
				separators = nil
				currentSize = nodeHeaderSize + 4
				continue
			}

			children = append(children, currentLevel[i].blockID)
			separators = append(separators, sep)
			currentSize += entrySize
		}

		// Flush final internal node
		if len(children) > 0 {
			rightmost := children[len(children)-1]
			internalData := serializeInternalNode(
				children[:len(children)-1],
				separators,
				rightmost,
			)
			blockID := uint32(len(allBlocks))
			allBlocks = append(allBlocks, internalData)

			var firstRec Record
			if len(separators) > 0 {
				firstRec = separators[0]
			}
			nextLevel = append(nextLevel, nodeRef{blockID: blockID, firstRec: firstRec})
		}

		currentLevel = nextLevel
	}

	rootBlockID := currentLevel[0].blockID
	master := serializeMasterBlock(rootBlockID, height, uint32(len(records)), uint32(len(allBlocks)))
	return btreeResult{masterBlock: master, nodeBlocks: allBlocks}
}

// leafNode holds serialized leaf data and its first record (for separator promotion).
type leafNode struct {
	data        []byte
	firstRecord Record
}

// buildLeafNodes packs sorted records into leaf nodes respecting the page size.
func buildLeafNodes(records []Record) []leafNode {
	var leaves []leafNode
	var currentRecords []Record
	currentSize := 8 // 8 bytes overhead: mode(4) + count(4)

	for _, rec := range records {
		recSize := rec.ByteLength()
		if currentSize+recSize > pageSize && len(currentRecords) > 0 {
			leaves = append(leaves, leafNode{
				data:        serializeLeafNode(currentRecords),
				firstRecord: currentRecords[0],
			})
			currentRecords = nil
			currentSize = 8
		}
		currentRecords = append(currentRecords, rec)
		currentSize += recSize
	}

	if len(currentRecords) > 0 {
		leaves = append(leaves, leafNode{
			data:        serializeLeafNode(currentRecords),
			firstRecord: currentRecords[0],
		})
	}

	return leaves
}

// serializeLeafNode creates a leaf node: [mode=0 (u32)] [count (u32)] [records...].
// The node is padded to at least pageSize bytes so the buddy allocator assigns a
// block large enough for Finder to read using the pageSize declared in the DSDB
// master block. Without this padding, Finder may ignore icvp and other records.
func serializeLeafNode(records []Record) []byte {
	buf := new(bytes.Buffer)
	writeU32(buf, 0)                    // mode = 0 (leaf)
	writeU32(buf, uint32(len(records))) // count
	for _, rec := range records {
		rec.WriteTo(buf)
	}
	// Pad to pageSize so buddy allocator allocates a full page
	for buf.Len() < pageSize {
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

// serializeInternalNode creates an internal node.
// Format: [mode=rightmostChild (u32)] [count (u32)] [child0 (u32)] [sep0] [child1 (u32)] [sep1] ...
// The children slice has len(separators) entries (one per separator).
// rightmostChild is the final child pointer (stored in the mode field).
func serializeInternalNode(children []uint32, separators []Record, rightmostChild uint32) []byte {
	buf := new(bytes.Buffer)
	writeU32(buf, rightmostChild)           // mode = rightmost child block ID
	writeU32(buf, uint32(len(separators)))  // count = number of separator records
	for i, sep := range separators {
		writeU32(buf, children[i]) // child pointer before this separator
		sep.WriteTo(buf)           // separator record
	}
	return buf.Bytes()
}

// serializeMasterBlock creates the 20-byte DSDB master block.
func serializeMasterBlock(rootBlockID, height, recordCount, nodeCount uint32) []byte {
	buf := new(bytes.Buffer)
	writeU32(buf, rootBlockID)
	writeU32(buf, height)
	writeU32(buf, recordCount)
	writeU32(buf, nodeCount)
	writeU32(buf, pageSize)
	return buf.Bytes()
}
