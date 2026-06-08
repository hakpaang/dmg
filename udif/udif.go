// Package udif writes Apple UDIF disk images (.dmg containers) in pure Go.
//
// A UDIF image wraps a raw, sector-aligned filesystem image (e.g. HFS+) in a
// compressed container that Finder and hdiutil can mount. This package
// implements the UDZO variant (zlib-compressed) and makes no calls to hdiutil
// or any host OS facility, so it works when cross-compiled to any platform.
//
// The implementation is a clean-room one written from the publicly documented
// UDIF format (the "koly" trailer, the "mish" / BLKX block map, and the three
// CRC-32 checksums). It does not derive from any GPL-licensed source.
//
// Layout of the produced file:
//
//	[ data fork: concatenated compressed/raw chunks ]
//	[ XML property list: the resource-fork describing the BLKX block map ]
//	[ koly trailer: 512-byte fixed UDIFResourceFile structure ]
package udif

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	sectorSize   = 512
	chunkSectors = 2048 // 1 MiB decompressed per chunk, matching hdiutil

	kolySignature = 0x6b6f6c79 // 'koly'
	mishSignature = 0x6d697368 // 'mish'

	// BLKX chunk entry types.
	chunkZeroFill   = 0x00000002 // run of zero sectors, no data stored
	chunkRaw        = 0x00000001 // uncompressed data stored verbatim
	chunkZlib       = 0x80000005 // zlib-compressed data
	chunkTerminator = 0xffffffff // end-of-table marker

	checksumCRC32 = 2 // UDIFChecksum.Type for CRC-32

	blocksDescriptor = 0xfffffffe // observed constant in mish header
)

// Build wraps a raw, 512-byte-sector-aligned filesystem image into a
// UDZO-compressed UDIF (.dmg) container and writes it to w.
//
// rawImage must be a whole-disk filesystem image (such as an HFS+ volume)
// whose length is a multiple of 512. partitionName is the cosmetic label
// recorded in the block map (e.g. "disk image"); it does not affect mounting.
func Build(w io.Writer, rawImage []byte, partitionName string) error {
	if len(rawImage)%sectorSize != 0 {
		return fmt.Errorf("udif: raw image length %d is not a multiple of %d", len(rawImage), sectorSize)
	}
	totalSectors := uint64(len(rawImage) / sectorSize)

	// Build the data fork (compressed chunks) and the matching BLKX chunk table.
	// The BLKX checksum is a CRC-32 over the uncompressed bytes of the *stored*
	// chunks only (zlib/raw), with zero-fill regions excluded — this matches
	// how hdiutil computes and re-verifies it.
	var dataFork bytes.Buffer
	var chunks []chunkEntry
	blkxHash := crc32.NewIEEE()
	for start := 0; start < len(rawImage); start += chunkSectors * sectorSize {
		end := start + chunkSectors*sectorSize
		if end > len(rawImage) {
			end = len(rawImage)
		}
		block := rawImage[start:end]
		sectorNum := uint64(start / sectorSize)
		sectorCount := uint64(len(block) / sectorSize)
		compOff := uint64(dataFork.Len())

		if isZero(block) {
			chunks = append(chunks, chunkEntry{
				entryType:   chunkZeroFill,
				sectorNum:   sectorNum,
				sectorCount: sectorCount,
				compOff:     compOff,
				compLen:     0,
			})
			continue
		}

		blkxHash.Write(block) // stored chunk: contributes to the BLKX checksum
		compressed := zlibCompress(block)
		if len(compressed) < len(block) {
			dataFork.Write(compressed)
			chunks = append(chunks, chunkEntry{
				entryType:   chunkZlib,
				sectorNum:   sectorNum,
				sectorCount: sectorCount,
				compOff:     compOff,
				compLen:     uint64(len(compressed)),
			})
		} else {
			// Compression did not help; store the block verbatim.
			dataFork.Write(block)
			chunks = append(chunks, chunkEntry{
				entryType:   chunkRaw,
				sectorNum:   sectorNum,
				sectorCount: sectorCount,
				compOff:     compOff,
				compLen:     uint64(len(block)),
			})
		}
	}
	// Terminator entry.
	chunks = append(chunks, chunkEntry{
		entryType:   chunkTerminator,
		sectorNum:   totalSectors,
		sectorCount: 0,
		compOff:     uint64(dataFork.Len()),
		compLen:     0,
	})

	forkBytes := dataFork.Bytes()

	// Checksums.
	blkxChecksum := blkxHash.Sum32()
	dataForkChecksum := crc32.ChecksumIEEE(forkBytes)
	// Master checksum is CRC-32 over the concatenation of each BLKX's CRC-32,
	// big-endian. We emit a single BLKX, so it covers just this one value.
	masterChecksum := crc32.ChecksumIEEE(be32(blkxChecksum))

	mish := buildMish(totalSectors, blkxChecksum, chunks)
	plist := buildPlist(partitionName, mish)

	koly := buildKoly(kolyParams{
		dataForkLen:      uint64(len(forkBytes)),
		xmlOffset:        uint64(len(forkBytes)),
		xmlLength:        uint64(len(plist)),
		sectorCount:      totalSectors,
		dataForkChecksum: dataForkChecksum,
		masterChecksum:   masterChecksum,
	})

	if _, err := w.Write(forkBytes); err != nil {
		return err
	}
	if _, err := w.Write(plist); err != nil {
		return err
	}
	if _, err := w.Write(koly); err != nil {
		return err
	}
	return nil
}

type chunkEntry struct {
	entryType   uint32
	sectorNum   uint64
	sectorCount uint64
	compOff     uint64
	compLen     uint64
}

// buildMish serializes the BLKX ("mish") block map for a single whole-disk
// partition covering all sectors.
func buildMish(totalSectors uint64, blkxChecksum uint32, chunks []chunkEntry) []byte {
	var b bytes.Buffer
	put32(&b, mishSignature)
	put32(&b, 1) // InfoVersion
	put64(&b, 0) // FirstSectorNumber
	put64(&b, totalSectors)
	put64(&b, 0)                     // DataStart
	put32(&b, chunkSectors+8)        // DecompressBufferRequested
	put32(&b, blocksDescriptor)      // BlocksDescriptor
	b.Write(make([]byte, 24))        // 6 reserved uint32
	b.Write(udifChecksum(blkxChecksum)) // 136-byte checksum of uncompressed data
	put32(&b, uint32(len(chunks)))   // BlockCount
	for _, c := range chunks {
		put32(&b, c.entryType)
		put32(&b, 0) // Comment
		put64(&b, c.sectorNum)
		put64(&b, c.sectorCount)
		put64(&b, c.compOff)
		put64(&b, c.compLen)
	}
	return b.Bytes()
}

type kolyParams struct {
	dataForkLen      uint64
	xmlOffset        uint64
	xmlLength        uint64
	sectorCount      uint64
	dataForkChecksum uint32
	masterChecksum   uint32
}

// buildKoly serializes the 512-byte koly trailer (UDIFResourceFile).
func buildKoly(p kolyParams) []byte {
	var b bytes.Buffer
	put32(&b, kolySignature)
	put32(&b, 4)   // Version
	put32(&b, 512) // HeaderSize
	put32(&b, 1)   // Flags
	put64(&b, 0)   // RunningDataForkOffset
	put64(&b, 0)   // DataForkOffset
	put64(&b, p.dataForkLen)
	put64(&b, 0) // RsrcForkOffset
	put64(&b, 0) // RsrcForkLength
	put32(&b, 1) // SegmentNumber
	put32(&b, 1) // SegmentCount
	b.Write(make([]byte, 16))               // SegmentID (zeros; deterministic)
	b.Write(udifChecksum(p.dataForkChecksum)) // DataForkChecksum
	put64(&b, p.xmlOffset)
	put64(&b, p.xmlLength)
	b.Write(make([]byte, 120))            // Reserved1
	b.Write(udifChecksum(p.masterChecksum)) // MasterChecksum
	put32(&b, 2) // ImageVariant: 2 = whole-disk "device" image
	put64(&b, p.sectorCount)
	put32(&b, 0) // reserved
	put32(&b, 0) // reserved
	put32(&b, 0) // reserved
	out := b.Bytes()
	if len(out) != 512 {
		// Programming error: the koly trailer must be exactly 512 bytes.
		panic(fmt.Sprintf("udif: koly trailer is %d bytes, want 512", len(out)))
	}
	return out
}

// udifChecksum builds the 136-byte UDIFChecksum structure for a CRC-32 value.
func udifChecksum(crc uint32) []byte {
	b := make([]byte, 136)
	binary.BigEndian.PutUint32(b[0:4], checksumCRC32) // Type
	binary.BigEndian.PutUint32(b[4:8], 32)            // BitCount
	binary.BigEndian.PutUint32(b[8:12], crc)          // first 4 bytes of Data[128]
	return b
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>resource-fork</key>
	<dict>
		<key>blkx</key>
		<array>
			<dict>
				<key>Attributes</key>
				<string>0x0050</string>
				<key>CFName</key>
				<string>%[1]s</string>
				<key>Data</key>
				<data>%[2]s</data>
				<key>ID</key>
				<string>0</string>
				<key>Name</key>
				<string>%[1]s</string>
			</dict>
		</array>
	</dict>
</dict>
</plist>
`

func buildPlist(partitionName string, mish []byte) []byte {
	enc := base64.StdEncoding.EncodeToString(mish)
	return []byte(fmt.Sprintf(plistTemplate, partitionName, enc))
}

func zlibCompress(data []byte) []byte {
	var buf bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	zw.Write(data)
	zw.Close()
	return buf.Bytes()
}

func isZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func be32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func put32(b *bytes.Buffer, v uint32) { binary.Write(b, binary.BigEndian, v) }
func put64(b *bytes.Buffer, v uint64) { binary.Write(b, binary.BigEndian, v) }
