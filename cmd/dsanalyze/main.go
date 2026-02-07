package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// Decode and compare .DS_Store files
func main() {
	for _, path := range os.Args[1:] {
		fmt.Printf("=== %s ===\n", path)
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
		analyze(data)
		fmt.Println()
	}
}

func analyze(data []byte) {
	if len(data) < 36 {
		fmt.Println("File too short")
		return
	}

	// Header
	alignMagic := binary.BigEndian.Uint32(data[0:4])
	fmt.Printf("Alignment magic: 0x%08X\n", alignMagic)

	magic := string(data[4:8])
	fmt.Printf("Magic: %q\n", magic)
	if magic != "Bud1" {
		fmt.Println("Not a valid .DS_Store file")
		return
	}

	bkOffset := binary.BigEndian.Uint32(data[8:12])
	bkSize := binary.BigEndian.Uint32(data[12:16])
	bkOffsetCopy := binary.BigEndian.Uint32(data[16:20])
	fmt.Printf("Bookkeeping: offset=0x%X (%d), size=0x%X (%d), offset_copy=0x%X\n",
		bkOffset, bkOffset, bkSize, bkSize, bkOffsetCopy)

	// Header extra bytes
	fmt.Printf("Header byte 0x14: 0x%08X\n", binary.BigEndian.Uint32(data[20:24]))

	// Bookkeeping block starts at byte 4 + bkOffset
	bkStart := 4 + int(bkOffset)
	if bkStart+4 > len(data) {
		fmt.Println("Bookkeeping block out of range")
		return
	}

	blockCount := binary.BigEndian.Uint32(data[bkStart : bkStart+4])
	fmt.Printf("\nBlock count: %d\n", blockCount)

	// Read block addresses (padded to next power of 2)
	paddedCount := nextPow2(blockCount)
	if paddedCount < 4 {
		paddedCount = 4 // minimum 4 entries?
	}
	fmt.Printf("Block address list padded to: %d entries\n", paddedCount)

	blockAddrs := make([]uint32, paddedCount)
	for i := uint32(0); i < paddedCount; i++ {
		off := bkStart + 4 + int(i)*4
		if off+4 <= len(data) {
			blockAddrs[i] = binary.BigEndian.Uint32(data[off : off+4])
		}
	}

	for i, addr := range blockAddrs {
		offset := addr &^ 0x1F
		log2Size := addr & 0x1F
		size := uint32(1) << log2Size
		absOffset := 4 + int(offset)
		valid := ""
		if uint32(i) >= blockCount {
			valid = " (beyond count)"
		}
		fmt.Printf("  Block %d: addr=0x%08X → offset=0x%X, size=%d (2^%d), abs_offset=0x%X%s\n",
			i, addr, offset, size, log2Size, absOffset, valid)
	}

	// TOC section: after block addresses
	tocStart := bkStart + 4 + int(paddedCount)*4
	if tocStart+4 > len(data) {
		fmt.Println("TOC section out of range")
		return
	}
	tocCount := binary.BigEndian.Uint32(data[tocStart : tocStart+4])
	fmt.Printf("\nTOC count: %d\n", tocCount)

	pos := tocStart + 4
	for i := uint32(0); i < tocCount; i++ {
		if pos >= len(data) {
			break
		}
		nameLen := data[pos]
		pos++
		name := string(data[pos : pos+int(nameLen)])
		pos += int(nameLen)
		blockID := binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
		fmt.Printf("  TOC[%d]: %q → block %d\n", i, name, blockID)
	}

	// Free lists (32 entries)
	fmt.Printf("\nFree lists start at bookkeeping offset %d (abs 0x%X):\n", pos-bkStart, pos)
	for i := 0; i < 32; i++ {
		if pos+4 > len(data) {
			break
		}
		count := binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4
		if count > 0 {
			fmt.Printf("  Free list [%d] (size 2^%d=%d): %d entries\n", i, i, 1<<i, count)
			for j := uint32(0); j < count; j++ {
				if pos+4 > len(data) {
					break
				}
				v := binary.BigEndian.Uint32(data[pos : pos+4])
				pos += 4
				fmt.Printf("    entry %d: 0x%08X\n", j, v)
			}
		} else {
			pos += 0 // no entries
		}
	}

	// Now decode B-tree from DSDB block
	fmt.Println("\n--- B-Tree ---")
	// Find DSDB block
	dsdbBlock := uint32(0)
	// Re-parse TOC to find DSDB
	tPos := tocStart + 4
	for i := uint32(0); i < tocCount; i++ {
		if tPos >= len(data) {
			break
		}
		nameLen := data[tPos]
		tPos++
		name := string(data[tPos : tPos+int(nameLen)])
		tPos += int(nameLen)
		blockID := binary.BigEndian.Uint32(data[tPos : tPos+4])
		tPos += 4
		if name == "DSDB" {
			dsdbBlock = blockID
		}
	}

	if dsdbBlock >= uint32(len(blockAddrs)) {
		fmt.Printf("DSDB block %d out of range\n", dsdbBlock)
		return
	}

	masterAddr := blockAddrs[dsdbBlock]
	masterOffset := int(masterAddr&^0x1F) + 4
	fmt.Printf("DSDB master block at abs offset 0x%X (block %d, addr 0x%08X)\n",
		masterOffset, dsdbBlock, masterAddr)

	if masterOffset+20 > len(data) {
		fmt.Println("Master block data out of range")
		return
	}

	rootNodeID := binary.BigEndian.Uint32(data[masterOffset : masterOffset+4])
	treeHeight := binary.BigEndian.Uint32(data[masterOffset+4 : masterOffset+8])
	recordCount := binary.BigEndian.Uint32(data[masterOffset+8 : masterOffset+12])
	nodeCount := binary.BigEndian.Uint32(data[masterOffset+12 : masterOffset+16])
	pageSize := binary.BigEndian.Uint32(data[masterOffset+16 : masterOffset+20])

	fmt.Printf("Master block: rootNodeID=%d, height=%d, records=%d, nodes=%d, pageSize=%d\n",
		rootNodeID, treeHeight, recordCount, nodeCount, pageSize)

	// Decode root node
	if rootNodeID >= uint32(len(blockAddrs)) {
		fmt.Printf("Root node block %d out of range\n", rootNodeID)
		return
	}

	rootAddr := blockAddrs[rootNodeID]
	rootOffset := int(rootAddr&^0x1F) + 4
	rootSize := uint32(1) << (rootAddr & 0x1F)
	fmt.Printf("Root node at abs offset 0x%X (block %d, size %d)\n",
		rootOffset, rootNodeID, rootSize)

	if rootOffset+8 > len(data) {
		fmt.Println("Root node out of range")
		return
	}

	nodeMode := binary.BigEndian.Uint32(data[rootOffset : rootOffset+4])
	nodeRecCount := binary.BigEndian.Uint32(data[rootOffset+4 : rootOffset+8])
	fmt.Printf("Node: mode=%d (0=leaf), records=%d\n", nodeMode, nodeRecCount)

	// Decode records
	rPos := rootOffset + 8
	for i := uint32(0); i < nodeRecCount; i++ {
		if rPos+4 > len(data) {
			break
		}
		// If internal node, skip child pointer
		if nodeMode > 0 {
			childID := binary.BigEndian.Uint32(data[rPos : rPos+4])
			fmt.Printf("  Child pointer: block %d\n", childID)
			rPos += 4
		}

		// Read filename (UTF-16BE string)
		if rPos+4 > len(data) {
			break
		}
		fnameLen := binary.BigEndian.Uint32(data[rPos : rPos+4])
		rPos += 4
		fname := ""
		for j := uint32(0); j < fnameLen; j++ {
			if rPos+2 > len(data) {
				break
			}
			ch := binary.BigEndian.Uint16(data[rPos : rPos+2])
			rPos += 2
			fname += string(rune(ch))
		}

		// Structure type (4 bytes)
		if rPos+4 > len(data) {
			break
		}
		code := string(data[rPos : rPos+4])
		rPos += 4

		// Data type (4 bytes)
		if rPos+4 > len(data) {
			break
		}
		dataType := string(data[rPos : rPos+4])
		rPos += 4

		// Value
		valueStr := ""
		switch dataType {
		case "long":
			if rPos+4 <= len(data) {
				v := binary.BigEndian.Uint32(data[rPos : rPos+4])
				valueStr = fmt.Sprintf("%d", v)
				rPos += 4
			}
		case "shor":
			if rPos+4 <= len(data) {
				// shor is stored as 4 bytes (padded)
				v := binary.BigEndian.Uint32(data[rPos : rPos+4])
				valueStr = fmt.Sprintf("%d", v)
				rPos += 4
			}
		case "bool":
			if rPos+1 <= len(data) {
				valueStr = fmt.Sprintf("%v", data[rPos] != 0)
				rPos += 1
			}
		case "blob":
			if rPos+4 <= len(data) {
				blobLen := binary.BigEndian.Uint32(data[rPos : rPos+4])
				rPos += 4
				if rPos+int(blobLen) <= len(data) {
					blob := data[rPos : rPos+int(blobLen)]
					if len(blob) > 0 && strings.HasPrefix(string(blob), "bplist") {
						valueStr = fmt.Sprintf("bplist (%d bytes)", blobLen)
					} else if len(blob) >= 4 {
						// Show first few bytes
						preview := fmt.Sprintf("%X", blob[:min(int(blobLen), 32)])
						valueStr = fmt.Sprintf("(%d bytes) %s", blobLen, preview)
					}
					rPos += int(blobLen)
				}
			}
		case "type":
			if rPos+4 <= len(data) {
				valueStr = string(data[rPos : rPos+4])
				rPos += 4
			}
		case "ustr":
			if rPos+4 <= len(data) {
				sLen := binary.BigEndian.Uint32(data[rPos : rPos+4])
				rPos += 4
				s := ""
				for j := uint32(0); j < sLen; j++ {
					if rPos+2 <= len(data) {
						ch := binary.BigEndian.Uint16(data[rPos : rPos+2])
						rPos += 2
						s += string(rune(ch))
					}
				}
				valueStr = fmt.Sprintf("%q", s)
			}
		case "comp":
			if rPos+8 <= len(data) {
				v := binary.BigEndian.Uint64(data[rPos : rPos+8])
				valueStr = fmt.Sprintf("%d", v)
				rPos += 8
			}
		default:
			valueStr = fmt.Sprintf("(unknown type %q)", dataType)
		}

		fmt.Printf("  Record[%d]: file=%q code=%s type=%s value=%s\n",
			i, fname, code, dataType, valueStr)
	}
}

func nextPow2(v uint32) uint32 {
	if v == 0 {
		return 1
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
