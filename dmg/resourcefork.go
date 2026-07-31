package dmg

import "encoding/binary"

// buildResourceFork constructs a Classic Mac OS resource fork containing
// a single 'icns' resource with the magic ID -16455 (0xBFB9). This is the
// format Finder expects when looking for a custom icon on a file.
//
// Resource fork layout:
//
//	[Header:    256 bytes] (16 bytes significant, 240 reserved)
//	[Data Area: variable ] (4-byte length prefix + ICNS data)
//	[Map:       50 bytes ] (header copy + type list + reference list)
func buildResourceFork(icnsData []byte) []byte {
	dataLen := len(icnsData)
	dataAreaLen := 4 + dataLen // 4-byte length prefix + data

	const headerSize = 256
	const mapSize = 50 // fixed for single-resource fork

	dataOffset := headerSize
	mapOffset := dataOffset + dataAreaLen

	total := mapOffset + mapSize
	buf := make([]byte, total)

	// === Resource Header (first 16 bytes of 256) ===
	binary.BigEndian.PutUint32(buf[0:4], uint32(dataOffset))    // offset to data area
	binary.BigEndian.PutUint32(buf[4:8], uint32(mapOffset))     // offset to map
	binary.BigEndian.PutUint32(buf[8:12], uint32(dataAreaLen))  // data area length
	binary.BigEndian.PutUint32(buf[12:16], uint32(mapSize))     // map length
	// bytes 16-255: reserved (already zero)

	// === Resource Data Area (at offset 256) ===
	pos := dataOffset
	binary.BigEndian.PutUint32(buf[pos:pos+4], uint32(dataLen)) // resource data length
	copy(buf[pos+4:], icnsData)                                  // ICNS bytes

	// === Resource Map (at mapOffset) ===
	pos = mapOffset
	// The resource map begins with an exact copy of the resource header.
	// Finder rejects the resource fork when these bytes are left zeroed.
	copy(buf[pos:pos+16], buf[:16])
	// bytes 16-19: next resource map handle (zero)
	// bytes 20-21: file reference number (zero)
	// bytes 22-23: resource file attributes (zero)
	binary.BigEndian.PutUint16(buf[pos+24:pos+26], 28) // offset from map to type list
	binary.BigEndian.PutUint16(buf[pos+26:pos+28], 50) // offset from map to name list (past end = no names)

	// === Type List (at map + 28) ===
	tlPos := pos + 28
	binary.BigEndian.PutUint16(buf[tlPos:tlPos+2], 0) // number of types minus 1 (1 type)

	// Type entry: 'icns'
	copy(buf[tlPos+2:tlPos+6], "icns")                          // resource type
	binary.BigEndian.PutUint16(buf[tlPos+6:tlPos+8], 0)         // count minus 1 (1 resource)
	binary.BigEndian.PutUint16(buf[tlPos+8:tlPos+10], 10)       // offset from type list to ref list

	// === Reference List Entry (at type list + 10) ===
	rlPos := tlPos + 10
	binary.BigEndian.PutUint16(buf[rlPos:rlPos+2], 0xBFB9) // resource ID -16455
	binary.BigEndian.PutUint16(buf[rlPos+2:rlPos+4], 0xFFFF) // no name
	buf[rlPos+4] = 0                                           // attributes
	// bytes 5-7: 3-byte offset from data area start to this resource (0)
	buf[rlPos+5] = 0
	buf[rlPos+6] = 0
	buf[rlPos+7] = 0
	// bytes 8-11: reserved handle (zero)

	return buf
}
