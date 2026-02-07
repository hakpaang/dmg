package dsstore

import (
	"bytes"
	"encoding/binary"
	"path"
	"strings"
	"unicode/utf16"
)

// AliasFileInfo holds filesystem metadata for building accurate alias records.
// Populated from the actual mounted volume during DMG creation.
type AliasFileInfo struct {
	VolCreationDate uint32 // HFS timestamp (seconds since 1904-01-01)
	FileCreationDate uint32
	ParentCNID      uint32 // inode of parent directory
	FileCNID        uint32 // inode of the file
	DriveType       uint16 // 5 for disk image
}

// BuildAlias builds a macOS Alias v2 record that points to a file
// inside a volume. This is used for .DS_Store background image references
// in the icvp.backgroundImageAlias field.
//
// volumeName is the DMG volume name (e.g., "My App").
// relativePath is the path from volume root (e.g., ".background/background.png").
// info provides real filesystem metadata; if nil, placeholder values are used.
func BuildAlias(volumeName, relativePath string, info *AliasFileInfo) []byte {
	buf := &bytes.Buffer{}

	var volCreDate, fileCreDate, parentCNID, fileCNID uint32
	var driveType uint16
	if info != nil {
		volCreDate = info.VolCreationDate
		fileCreDate = info.FileCreationDate
		parentCNID = info.ParentCNID
		fileCNID = info.FileCNID
		driveType = info.DriveType
	}

	// userType (4 bytes, always 0)
	writeU32(buf, 0)

	// Placeholder for aliasSize (filled in at end)
	aliasSizeOffset := buf.Len()
	writeU16(buf, 0)

	// version = 2
	writeU16(buf, 2)

	// kind = 0 (file)
	writeU16(buf, 0)

	// Volume name as Pascal string (1 byte length + 27 bytes padded)
	writePascalString(buf, volumeName, 28)

	// Volume creation date (Mac HFS timestamp)
	writeU32(buf, volCreDate)

	// Volume signature: "H+" = 0x482B
	writeU16(buf, 0x482B)

	// Drive type
	writeU16(buf, driveType)

	// Parent directory CNID
	writeU32(buf, parentCNID)

	// File name as Pascal string (1 byte length + 63 bytes padded)
	fileName := path.Base(relativePath)
	writePascalString(buf, fileName, 64)

	// File CNID
	writeU32(buf, fileCNID)

	// File creation date
	writeU32(buf, fileCreDate)

	// File type FourCC (0)
	writeU32(buf, 0)

	// File creator FourCC (0)
	writeU32(buf, 0)

	// nlvlFrom (0xFFFF = unknown, as Finder uses)
	writeU16(buf, 0xFFFF)

	// nlvlTo (0xFFFF = unknown, as Finder uses)
	writeU16(buf, 0xFFFF)

	// Volume attributes (0)
	writeU32(buf, 0)

	// Volume filesystem ID (0)
	writeU16(buf, 0)

	// Reserved (10 bytes)
	buf.Write(make([]byte, 10))

	// --- Tagged extra data ---
	// Tag order and formats match mac_alias reference implementation.

	dir := path.Dir(relativePath)
	if dir == "." {
		dir = ""
	}

	// Tag 0x0000: Parent directory name (UTF-8)
	// For files at volume root, parent directory is the volume itself.
	if dir != "" {
		writeAliasTag(buf, 0x0000, []byte(dir))
	} else {
		writeAliasTag(buf, 0x0000, []byte(volumeName))
	}

	// Tag 0x0010: High-resolution volume creation date (uint64, HFS+ epoch)
	// Format: 32-bit seconds shifted left 16 bits (lower 16 for sub-second precision)
	writeAliasTagU64(buf, 0x0010, uint64(volCreDate)<<16)

	// Tag 0x0011: High-resolution file creation date (uint64, HFS+ epoch)
	writeAliasTagU64(buf, 0x0011, uint64(fileCreDate)<<16)

	// Tag 0x0002: Carbon/HFS colon-separated path
	carbonPath := volumeName + ":" + strings.ReplaceAll(relativePath, "/", ":")
	writeAliasTag(buf, 0x0002, []byte(carbonPath))

	// Tag 0x000e: File name (UTF-16BE with 2-byte character count prefix)
	writeAliasTagUTF16Counted(buf, 0x000E, fileName)

	// Tag 0x000f: Volume name (UTF-16BE with 2-byte character count prefix)
	writeAliasTagUTF16Counted(buf, 0x000F, volumeName)

	// Tag 0x0012: Relative POSIX path from volume root (UTF-8)
	writeAliasTag(buf, 0x0012, []byte("/"+relativePath))

	// Tag 0x0013: Volume mount point (UTF-8)
	mountPoint := "/Volumes/" + volumeName
	writeAliasTag(buf, 0x0013, []byte(mountPoint))

	// End tag
	writeU16(buf, 0xFFFF)
	writeU16(buf, 0)

	// Fill in aliasSize: total record size (matches mac_alias convention)
	data := buf.Bytes()
	aliasSize := uint16(len(data))
	binary.BigEndian.PutUint16(data[aliasSizeOffset:], aliasSize)

	return data
}

// writePascalString writes a Pascal string padded to exactly padTo bytes.
// The first byte is the string length, followed by the string bytes,
// followed by zero padding.
func writePascalString(buf *bytes.Buffer, s string, padTo int) {
	data := []byte(s)
	maxLen := padTo - 1
	if len(data) > maxLen {
		data = data[:maxLen]
	}
	buf.WriteByte(byte(len(data)))
	buf.Write(data)
	padding := padTo - 1 - len(data)
	if padding > 0 {
		buf.Write(make([]byte, padding))
	}
}

// writeAliasTag writes a tagged data entry in the alias extra data section.
func writeAliasTag(buf *bytes.Buffer, tagType uint16, data []byte) {
	writeU16(buf, tagType)
	writeU16(buf, uint16(len(data)))
	buf.Write(data)
	if len(data)%2 != 0 {
		buf.WriteByte(0) // pad to even length
	}
}

// writeAliasTagUTF16Counted writes a tagged UTF-16BE string entry with a
// 2-byte character count prefix, matching Finder's alias format.
func writeAliasTagUTF16Counted(buf *bytes.Buffer, tagType uint16, s string) {
	encoded := utf16.Encode([]rune(s))
	// 2 bytes for count + 2 bytes per character
	data := make([]byte, 2+len(encoded)*2)
	binary.BigEndian.PutUint16(data[0:], uint16(len(encoded)))
	for i, u := range encoded {
		binary.BigEndian.PutUint16(data[2+i*2:], u)
	}
	writeAliasTag(buf, tagType, data)
}

// writeU32BE writes a big-endian uint32 to a buffer.
func writeU32BE(buf *bytes.Buffer, v uint32) {
	binary.Write(buf, binary.BigEndian, v)
}

// writeAliasTagU64 writes a tagged 8-byte uint64 entry.
func writeAliasTagU64(buf *bytes.Buffer, tagType uint16, v uint64) {
	writeU16(buf, tagType)
	writeU16(buf, 8)
	binary.Write(buf, binary.BigEndian, v)
}

// splitPath splits a path into its components.
func splitPath(p string) []string {
	var parts []string
	for p != "" && p != "." && p != "/" {
		dir, file := path.Split(p)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		p = path.Clean(dir)
	}
	return parts
}
