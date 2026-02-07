// Package dsstore implements reading and writing of macOS .DS_Store files.
//
// .DS_Store files are binary files created by macOS Finder to store folder
// presentation metadata: icon positions, background images, window geometry,
// view settings, etc. The format uses a buddy allocator for block management
// and a B-tree for sorted record storage.
//
// This package focuses on write support for generating .DS_Store files
// suitable for embedding in DMG disk images.
package dsstore

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

// FourCharCode is a 4-byte identifier used throughout the .DS_Store format
// for structure types (e.g., "Iloc", "icvp") and data types (e.g., "blob", "long").
type FourCharCode [4]byte

// FCC creates a FourCharCode from a 4-character string.
func FCC(s string) FourCharCode {
	var fcc FourCharCode
	copy(fcc[:], s)
	return fcc
}

// String returns the FourCharCode as a string.
func (f FourCharCode) String() string {
	return string(f[:])
}

// Equal returns true if two FourCharCodes are identical.
func (f FourCharCode) Equal(other FourCharCode) bool {
	return f == other
}

// Data type identifiers. These appear in records to indicate how the
// value payload should be interpreted.
var (
	DataLong = FCC("long") // 32-bit unsigned integer
	DataShor = FCC("shor") // 16-bit unsigned integer (stored in 4 bytes, first 2 zero)
	DataBool = FCC("bool") // Boolean (1 byte: 0x00 or 0x01)
	DataBlob = FCC("blob") // Length-prefixed byte array
	DataType = FCC("type") // FourCharCode value
	DataUstr = FCC("ustr") // Length-prefixed UTF-16BE string
	DataComp = FCC("comp") // 64-bit unsigned integer
	DataDutc = FCC("dutc") // Mac timestamp (1/65536-sec since 1904-01-01)
)

// writeU8 writes a single byte.
func writeU8(buf *bytes.Buffer, v byte) {
	buf.WriteByte(v)
}

// writeU16 writes a big-endian uint16.
func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

// writeU32 writes a big-endian uint32.
func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

// writeU64 writes a big-endian uint64.
func writeU64(buf *bytes.Buffer, v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	buf.Write(b[:])
}

// writeFCC writes a FourCharCode (4 bytes).
func writeFCC(buf *bytes.Buffer, fcc FourCharCode) {
	buf.Write(fcc[:])
}

// writeUTF16BEString writes a length-prefixed UTF-16 big-endian string.
// The length prefix is the number of UTF-16 code units (not bytes).
func writeUTF16BEString(buf *bytes.Buffer, s string) {
	encoded := utf16.Encode([]rune(s))
	writeU32(buf, uint32(len(encoded)))
	for _, u := range encoded {
		writeU16(buf, u)
	}
}

// writeRecordValue writes a typed record value: [data_type (4B)] [payload].
func writeRecordValue(buf *bytes.Buffer, dataType FourCharCode, value interface{}) {
	writeFCC(buf, dataType)

	switch dataType {
	case DataLong:
		writeU32(buf, value.(uint32))

	case DataShor:
		// Stored as 4 bytes: first 2 zero, last 2 = value
		writeU16(buf, 0)
		writeU16(buf, value.(uint16))

	case DataBool:
		if value.(bool) {
			writeU8(buf, 1)
		} else {
			writeU8(buf, 0)
		}

	case DataBlob:
		data := value.([]byte)
		writeU32(buf, uint32(len(data)))
		buf.Write(data)

	case DataType:
		writeFCC(buf, value.(FourCharCode))

	case DataUstr:
		writeUTF16BEString(buf, value.(string))

	case DataComp:
		writeU64(buf, value.(uint64))

	case DataDutc:
		writeU64(buf, value.(uint64))
	}
}

// recordValueSize returns the serialized size of a typed record value
// (including the 4-byte data type prefix).
func recordValueSize(dataType FourCharCode, value interface{}) int {
	size := 4 // data type code

	switch dataType {
	case DataLong:
		size += 4
	case DataShor:
		size += 4
	case DataBool:
		size += 1
	case DataBlob:
		size += 4 + len(value.([]byte))
	case DataType:
		size += 4
	case DataUstr:
		encoded := utf16.Encode([]rune(value.(string)))
		size += 4 + 2*len(encoded)
	case DataComp:
		size += 8
	case DataDutc:
		size += 8
	}

	return size
}

// nextPow2 returns the smallest power of 2 >= v. Returns 1 for v=0.
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
	return v + 1
}

// log2 returns floor(log2(v)). v must be a power of 2.
func log2(v uint32) uint32 {
	var n uint32
	for v > 1 {
		v >>= 1
		n++
	}
	return n
}

// align rounds offset up to the next multiple of alignment.
func align(offset, alignment uint32) uint32 {
	if alignment == 0 {
		return offset
	}
	return (offset + alignment - 1) & ^(alignment - 1)
}
