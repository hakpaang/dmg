// Package hfsplus writes raw HFS+/HFSX volume images in pure Go.
//
// The output is a sector-aligned filesystem image suitable for wrapping in a
// UDIF container (see the sibling udif package) to produce a mountable .dmg.
// It makes no host OS calls, so it works when cross-compiled to any platform —
// the goal being to build macOS disk images on Linux/Windows CI.
//
// The writer is build-once and append-only: the entire directory tree is known
// up front, so there is no deletion, no node splitting, no journal, and no free
// space reuse. B-trees are bulk-loaded from pre-sorted keys. This is a clean-room
// implementation from Apple's Technote TN1150 (HFS Plus Volume Format); it does
// not derive from any GPL-licensed source.
//
// To avoid implementing the TN1150 Unicode case-folding table, volumes are
// written as case-sensitive HFSX (signature "HX", binary key comparison). This
// is fully mountable by macOS and adequate for read-only distribution images.
package hfsplus

import (
	"fmt"
	"time"
)

const (
	blockSize = 4096 // allocation block size in bytes
	nodeSize  = 4096 // B-tree node size in bytes
	sectorSz  = 512

	sigHFSX    = 0x4858 // 'HX'
	versionHFSX = 5

	// Reserved catalog node IDs (CNIDs).
	cnidRootParent = 1 // parent of the root folder
	cnidRootFolder = 2 // the root folder itself
	cnidFirstUser  = 16

	// B-tree node kinds (stored as a signed byte).
	kindLeaf   = -1
	kindIndex  = 0
	kindHeader = 1
	kindMap    = 2

	// Catalog record types.
	recFolder       = 1
	recFile         = 2
	recFolderThread = 3
	recFileThread   = 4

	keyCompareBinary = 0xBC // HFSX case-sensitive (binary) comparison

	// B-tree header attributes.
	btBigKeysMask          = 0x00000002
	btVariableIndexKeysMask = 0x00000004

	maxCatalogKeyLen = 516

	// Volume attribute bits.
	volUnmounted = 0x00000100
)

// hfsEpochDelta is the seconds between the HFS epoch (1904-01-01) and the Unix
// epoch (1970-01-01).
const hfsEpochDelta = 2082844800

func hfsTime(t time.Time) uint32 {
	return uint32(t.Unix() + hfsEpochDelta)
}

// extent is a contiguous run of allocation blocks.
type extent struct {
	startBlock uint32
	blockCount uint32
}

// fork describes one file fork (its size and on-disk extents). For this
// build-once writer every fork fits in a single extent.
type fork struct {
	logicalSize uint64
	totalBlocks uint32
	extents     []extent
}

// BuildEmpty writes a minimal, empty HFSX volume of the given size (in bytes,
// which must be a positive multiple of the 4096-byte block size) and returns
// the raw image. The volume contains only its root folder.
//
// This is the foundational case; file/folder support builds on the same layout.
func BuildEmpty(volumeName string, sizeBytes int, createdAt time.Time) ([]byte, error) {
	if sizeBytes <= 0 || sizeBytes%blockSize != 0 {
		return nil, fmt.Errorf("hfsplus: size %d must be a positive multiple of %d", sizeBytes, blockSize)
	}
	totalBlocks := uint32(sizeBytes / blockSize)
	if totalBlocks < 8 {
		return nil, fmt.Errorf("hfsplus: volume too small (%d blocks)", totalBlocks)
	}

	img := make([]byte, sizeBytes)

	// --- Block layout (contiguous, simple) ---
	//   block 0           : reserved (boot blocks + volume header at offset 1024)
	//   block 1           : allocation bitmap
	//   blocks 2..3       : catalog B-tree (header node + root leaf node)
	//   block 4           : extents overflow B-tree (header node only, empty)
	//   last block        : holds the alternate volume header (offset size-1024)
	const allocStart = 1
	const catalogStart = 2
	const catalogBlocks = 2
	const extentsStart = 4
	const extentsBlocks = 1
	lastBlock := totalBlocks - 1

	catalogFork := fork{
		logicalSize: uint64(catalogBlocks) * blockSize,
		totalBlocks: catalogBlocks,
		extents:     []extent{{catalogStart, catalogBlocks}},
	}
	extentsFork := fork{
		logicalSize: uint64(extentsBlocks) * blockSize,
		totalBlocks: extentsBlocks,
		extents:     []extent{{extentsStart, extentsBlocks}},
	}
	allocFork := fork{
		logicalSize: blockSize,
		totalBlocks: 1,
		extents:     []extent{{allocStart, 1}},
	}

	// --- Allocation bitmap ---
	usedBlocks := []uint32{0, allocStart, catalogStart, catalogStart + 1, extentsStart, lastBlock}
	bitmap := make([]byte, blockSize)
	for _, b := range usedBlocks {
		bitmap[b/8] |= 0x80 >> (b % 8)
	}
	copy(img[allocStart*blockSize:], bitmap)
	freeBlocks := totalBlocks - uint32(len(usedBlocks))

	// --- Catalog B-tree ---
	catalog := buildCatalogEmpty(volumeName, createdAt)
	copy(img[catalogStart*blockSize:], catalog)

	// --- Extents overflow B-tree (empty) ---
	extents := buildEmptyBTree(extentsBlocks, maxExtentKeyLen, keyCompareBinary)
	copy(img[extentsStart*blockSize:], extents)

	// --- Volume header (offset 1024) and alternate (offset size-1024) ---
	vh := buildVolumeHeader(volumeHeaderParams{
		volumeName:  volumeName,
		createdAt:   createdAt,
		totalBlocks: totalBlocks,
		freeBlocks:  freeBlocks,
		nextAlloc:   extentsStart + extentsBlocks,
		nextCatalog: cnidFirstUser,
		fileCount:   0,
		folderCount: 0,
		allocFork:   allocFork,
		extentsFork: extentsFork,
		catalogFork: catalogFork,
	})
	copy(img[1024:], vh)
	copy(img[sizeBytes-1024:], vh) // alternate volume header

	return img, nil
}

const maxExtentKeyLen = 10 // kHFSPlusExtentKeyMaximumLength
