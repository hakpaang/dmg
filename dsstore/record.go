package dsstore

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Structure type code constants. These identify what property a record sets.
var (
	CodeBKGD = FCC("BKGD") // Background type
	CodeICVO = FCC("ICVO") // Icon view options (boolean)
	CodeIcvp = FCC("icvp") // Icon view properties (plist, 10.6+)
	CodeIcvo = FCC("icvo") // Icon view options (pre-10.6)
	CodeIloc = FCC("Iloc") // Icon location
	CodeLsvp = FCC("lsvp") // List view properties (plist)
	CodeBwsp = FCC("bwsp") // Browser window settings (plist)
	CodeFwi0 = FCC("fwi0") // Finder window info
	CodeFwsw = FCC("fwsw") // Finder window sidebar width
	CodeFwvh = FCC("fwvh") // Finder window vertical height
	CodePict = FCC("pict") // Background picture (alias)
	CodeIcvl = FCC("icvl") // Icon view layout (type code, 10.6+)
	CodeVSrn = FCC("vSrn") // Version
	CodeVstl = FCC("vstl") // View style
)

// View style values for the vstl record.
var (
	ViewIcon      = FCC("icnv") // Icon view
	ViewList      = FCC("Nlsv") // List view
	ViewColumn    = FCC("clmv") // Column view
	ViewCoverflow = FCC("Flwv") // Coverflow view
)

// Record represents a single entry in a .DS_Store file.
// Each record associates a filename with a property (Code) and its typed value.
type Record struct {
	Filename string
	Code     FourCharCode // Structure type (e.g., "Iloc", "icvp")
	DataType FourCharCode // Data type (e.g., "blob", "long")
	Value    interface{}  // The actual data
}

// Less returns true if this record sorts before other.
// Records sort by case-insensitive filename, then by structure type code.
func (r Record) Less(other Record) bool {
	a := strings.ToLower(r.Filename)
	b := strings.ToLower(other.Filename)
	if a != b {
		return a < b
	}
	return r.Code.String() < other.Code.String()
}

// ByteLength returns the total serialized size of this record in bytes.
func (r Record) ByteLength() int {
	utf16Len := len(utf16.Encode([]rune(r.Filename)))
	return 4 + (2 * utf16Len) + 4 + recordValueSize(r.DataType, r.Value)
}

// WriteTo serializes the record to a buffer.
func (r Record) WriteTo(buf *bytes.Buffer) {
	writeUTF16BEString(buf, r.Filename)
	writeFCC(buf, r.Code)
	writeRecordValue(buf, r.DataType, r.Value)
}

// --- Record constructors ---

// NewIlocRecord creates an icon location record.
// x and y are the center coordinates of the icon.
func NewIlocRecord(filename string, x, y uint32) Record {
	blob := make([]byte, 16)
	binary.BigEndian.PutUint32(blob[0:4], x)
	binary.BigEndian.PutUint32(blob[4:8], y)
	for i := 8; i < 14; i++ {
		blob[i] = 0xFF
	}
	// bytes 14-15 are 0x0000 (already zero from make)
	return Record{
		Filename: filename,
		Code:     CodeIloc,
		DataType: DataBlob,
		Value:    blob,
	}
}

// NewVSrnRecord creates a version record (always long value 1, filename ".").
func NewVSrnRecord() Record {
	return Record{
		Filename: ".",
		Code:     CodeVSrn,
		DataType: DataLong,
		Value:    uint32(1),
	}
}

// NewVstlRecord creates a view style record (filename ".").
func NewVstlRecord(style FourCharCode) Record {
	return Record{
		Filename: ".",
		Code:     CodeVstl,
		DataType: DataType,
		Value:    style,
	}
}

// NewBKGDDefaultRecord creates a default background record.
func NewBKGDDefaultRecord() Record {
	blob := make([]byte, 12)
	copy(blob[0:4], "DefB")
	return Record{Filename: ".", Code: CodeBKGD, DataType: DataBlob, Value: blob}
}

// NewBKGDColorRecord creates a solid color background record.
func NewBKGDColorRecord(r, g, b uint16) Record {
	blob := make([]byte, 12)
	copy(blob[0:4], "ClrB")
	binary.BigEndian.PutUint16(blob[4:6], r)
	binary.BigEndian.PutUint16(blob[6:8], g)
	binary.BigEndian.PutUint16(blob[8:10], b)
	return Record{Filename: ".", Code: CodeBKGD, DataType: DataBlob, Value: blob}
}

// NewBKGDPictureRecord creates a picture background record.
func NewBKGDPictureRecord() Record {
	blob := make([]byte, 12)
	copy(blob[0:4], "PctB")
	return Record{Filename: ".", Code: CodeBKGD, DataType: DataBlob, Value: blob}
}

// NewPictRecord creates a background picture alias record (filename ".").
func NewPictRecord(aliasData []byte) Record {
	return Record{Filename: ".", Code: CodePict, DataType: DataBlob, Value: aliasData}
}

// NewFwi0Record creates a Finder window info record.
// top, left, bottom, right define the window rect. viewType is the view mode.
func NewFwi0Record(top, left, bottom, right uint16, viewType FourCharCode) Record {
	blob := make([]byte, 16)
	binary.BigEndian.PutUint16(blob[0:2], top)
	binary.BigEndian.PutUint16(blob[2:4], left)
	binary.BigEndian.PutUint16(blob[4:6], bottom)
	binary.BigEndian.PutUint16(blob[6:8], right)
	copy(blob[8:12], viewType[:])
	// bytes 12-15 are zero
	return Record{Filename: ".", Code: CodeFwi0, DataType: DataBlob, Value: blob}
}

// NewFwswRecord creates a Finder window sidebar width record (filename ".").
func NewFwswRecord(width uint32) Record {
	return Record{Filename: ".", Code: CodeFwsw, DataType: DataLong, Value: width}
}

// NewFwvhRecord creates a Finder window vertical height record (filename ".").
func NewFwvhRecord(height uint16) Record {
	return Record{Filename: ".", Code: CodeFwvh, DataType: DataShor, Value: height}
}
