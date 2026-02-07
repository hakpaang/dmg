package dsstore

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestCodecNextPow2(t *testing.T) {
	tests := []struct {
		in   uint32
		want uint32
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{31, 32},
		{32, 32},
		{33, 64},
		{4096, 4096},
		{4097, 8192},
	}
	for _, tt := range tests {
		got := nextPow2(tt.in)
		if got != tt.want {
			t.Errorf("nextPow2(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestCodecLog2(t *testing.T) {
	tests := []struct {
		in   uint32
		want uint32
	}{
		{1, 0},
		{2, 1},
		{4, 2},
		{8, 3},
		{32, 5},
		{4096, 12},
	}
	for _, tt := range tests {
		got := log2(tt.in)
		if got != tt.want {
			t.Errorf("log2(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestWriteUTF16BEString(t *testing.T) {
	buf := new(bytes.Buffer)
	writeUTF16BEString(buf, "test")

	data := buf.Bytes()
	// 4 bytes length (4 chars) + 4 * 2 bytes = 12 bytes total
	if len(data) != 12 {
		t.Fatalf("expected 12 bytes, got %d", len(data))
	}

	charCount := binary.BigEndian.Uint32(data[0:4])
	if charCount != 4 {
		t.Errorf("expected char count 4, got %d", charCount)
	}

	// 't' = 0x0074, 'e' = 0x0065, 's' = 0x0073, 't' = 0x0074
	expected := []byte{0x00, 0x74, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74}
	if !bytes.Equal(data[4:], expected) {
		t.Errorf("UTF-16BE data mismatch: got %x, want %x", data[4:], expected)
	}
}

func TestRecordIloc(t *testing.T) {
	rec := NewIlocRecord("MyApp.app", 140, 180)

	if rec.Filename != "MyApp.app" {
		t.Errorf("filename = %q, want MyApp.app", rec.Filename)
	}
	if rec.Code != CodeIloc {
		t.Errorf("code = %s, want Iloc", rec.Code)
	}
	if rec.DataType != DataBlob {
		t.Errorf("data type = %s, want blob", rec.DataType)
	}

	blob := rec.Value.([]byte)
	if len(blob) != 16 {
		t.Fatalf("blob length = %d, want 16", len(blob))
	}

	x := binary.BigEndian.Uint32(blob[0:4])
	y := binary.BigEndian.Uint32(blob[4:8])
	if x != 140 || y != 180 {
		t.Errorf("position = (%d, %d), want (140, 180)", x, y)
	}

	// Check padding
	for i := 8; i < 14; i++ {
		if blob[i] != 0xFF {
			t.Errorf("byte %d = 0x%02x, want 0xFF", i, blob[i])
		}
	}
	for i := 14; i < 16; i++ {
		if blob[i] != 0x00 {
			t.Errorf("byte %d = 0x%02x, want 0x00", i, blob[i])
		}
	}
}

func TestRecordSorting(t *testing.T) {
	r1 := NewIlocRecord("Applications", 400, 180)
	r2 := NewIlocRecord("MyApp.app", 140, 180)
	r3 := NewVSrnRecord() // filename "."
	r4 := NewVstlRecord(ViewIcon) // filename "."

	// "." < "Applications" < "MyApp.app"
	if !r3.Less(r1) {
		t.Error("expected '.' < 'Applications'")
	}
	if !r1.Less(r2) {
		t.Error("expected 'Applications' < 'MyApp.app'")
	}
	// Same filename ".": sort by code
	if !r3.Less(r4) {
		t.Errorf("expected vSrn < vstl, got %s vs %s", r3.Code, r4.Code)
	}
}

func TestRecordSerialization(t *testing.T) {
	rec := NewVSrnRecord()
	buf := new(bytes.Buffer)
	rec.WriteTo(buf)

	data := buf.Bytes()
	expectedLen := rec.ByteLength()
	if len(data) != expectedLen {
		t.Errorf("serialized length = %d, ByteLength() = %d", len(data), expectedLen)
	}

	// Should start with filename "." in UTF-16BE: length=1, char=0x002E
	nameLen := binary.BigEndian.Uint32(data[0:4])
	if nameLen != 1 {
		t.Errorf("filename length = %d, want 1", nameLen)
	}
	nameChar := binary.BigEndian.Uint16(data[4:6])
	if nameChar != 0x002E {
		t.Errorf("filename char = 0x%04x, want 0x002E ('.')", nameChar)
	}
}

func TestBTreeSingleLeaf(t *testing.T) {
	records := []Record{
		NewIlocRecord("Applications", 400, 180),
		NewIlocRecord("MyApp.app", 140, 180),
		NewVSrnRecord(),
		NewVstlRecord(ViewIcon),
	}

	// Sort
	sortRecords(records)

	result := buildBTree(records)

	// Master block should be 20 bytes
	if len(result.masterBlock) != 20 {
		t.Fatalf("master block = %d bytes, want 20", len(result.masterBlock))
	}

	// With 4 records, should fit in single leaf
	if len(result.nodeBlocks) != 1 {
		t.Fatalf("node blocks = %d, want 1", len(result.nodeBlocks))
	}

	// Verify master block fields
	rootID := binary.BigEndian.Uint32(result.masterBlock[0:4])
	height := binary.BigEndian.Uint32(result.masterBlock[4:8])
	recCount := binary.BigEndian.Uint32(result.masterBlock[8:12])
	nodeCount := binary.BigEndian.Uint32(result.masterBlock[12:16])
	pSize := binary.BigEndian.Uint32(result.masterBlock[16:20])

	if rootID != 0 {
		t.Errorf("rootID = %d, want 0", rootID)
	}
	if height != 0 {
		t.Errorf("height = %d, want 0", height)
	}
	if recCount != 4 {
		t.Errorf("record count = %d, want 4", recCount)
	}
	if nodeCount != 1 {
		t.Errorf("node count = %d, want 1", nodeCount)
	}
	if pSize != 4096 {
		t.Errorf("page size = %d, want 4096", pSize)
	}

	// Verify leaf node starts with mode=0, count=4
	leaf := result.nodeBlocks[0]
	mode := binary.BigEndian.Uint32(leaf[0:4])
	count := binary.BigEndian.Uint32(leaf[4:8])
	if mode != 0 {
		t.Errorf("leaf mode = %d, want 0", mode)
	}
	if count != 4 {
		t.Errorf("leaf count = %d, want 4", count)
	}
}

func TestBuildAlias(t *testing.T) {
	alias := BuildAlias("My App", ".background/background.png", nil)

	// Should start with userType = 0
	if binary.BigEndian.Uint32(alias[0:4]) != 0 {
		t.Error("userType should be 0")
	}

	// Version should be 2
	version := binary.BigEndian.Uint16(alias[6:8])
	if version != 2 {
		t.Errorf("version = %d, want 2", version)
	}

	// Kind should be 0 (file)
	kind := binary.BigEndian.Uint16(alias[8:10])
	if kind != 0 {
		t.Errorf("kind = %d, want 0", kind)
	}

	// Volume name Pascal string should start with length 6 ("My App")
	volNameLen := alias[10]
	if volNameLen != 6 {
		t.Errorf("volume name length = %d, want 6", volNameLen)
	}

	// Volume signature should be 0x482B ("H+")
	volSig := binary.BigEndian.Uint16(alias[42:44])
	if volSig != 0x482B {
		t.Errorf("volume signature = 0x%04X, want 0x482B", volSig)
	}

	// File name should be "background.png" (length 14)
	fileNameLen := alias[50]
	if fileNameLen != 14 {
		t.Errorf("filename length = %d, want 14", fileNameLen)
	}

	// Check alias size is reasonable
	if len(alias) < 150 {
		t.Errorf("alias too short: %d bytes (minimum 150 for header)", len(alias))
	}
}

func TestDSStoreFileFormat(t *testing.T) {
	var buf bytes.Buffer

	s := New()
	s.SetIconPosition("MyApp.app", 140, 180)
	s.SetIconPosition("Applications", 400, 180)
	s.SetViewStyle(ViewIcon)
	s.SetWindowPosition(100, 200, 480, 740)

	err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	data := buf.Bytes()

	// Minimum file checks
	if len(data) < 36 {
		t.Fatalf("file too short: %d bytes", len(data))
	}

	// Check alignment magic
	if data[0] != 0x00 || data[1] != 0x00 || data[2] != 0x00 || data[3] != 0x01 {
		t.Errorf("alignment magic = %x, want 00000001", data[0:4])
	}

	// Check buddy allocator magic "Bud1"
	if string(data[4:8]) != "Bud1" {
		t.Errorf("buddy magic = %q, want \"Bud1\"", string(data[4:8]))
	}

	// Bookkeeping offset at bytes 8-11 should match copy at bytes 16-19
	bkOffset1 := binary.BigEndian.Uint32(data[8:12])
	bkSize := binary.BigEndian.Uint32(data[12:16])
	bkOffset2 := binary.BigEndian.Uint32(data[16:20])
	if bkOffset1 != bkOffset2 {
		t.Errorf("bookkeeping offset mismatch: %d vs %d", bkOffset1, bkOffset2)
	}
	if bkSize == 0 {
		t.Error("bookkeeping size should be > 0")
	}

	t.Logf("File size: %d bytes", len(data))
	t.Logf("Bookkeeping offset: %d, size: %d", bkOffset1, bkSize)
}

func TestDSStoreWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".DS_Store")

	s := New()
	s.SetIconPosition("MyApp.app", 140, 180)
	s.SetIconPosition("Applications", 400, 180)
	s.SetViewStyle(ViewIcon)
	s.SetWindowPosition(100, 200, 480, 740)

	err := s.WriteFile(path)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file was created and is non-empty
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Size() == 0 {
		t.Error("file is empty")
	}

	// Read and verify header
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data[4:8]) != "Bud1" {
		t.Errorf("buddy magic = %q, want \"Bud1\"", string(data[4:8]))
	}

	t.Logf("Wrote .DS_Store: %d bytes to %s", len(data), path)
}

func TestDSStoreWithBackground(t *testing.T) {
	var buf bytes.Buffer

	s := New()
	s.SetBackgroundImage("My App", ".background/background.png", &IconViewOptions{
		IconSize:  128,
		TextSize:  12,
		GridSpace: 100,
	}, nil)
	s.SetIconPosition("MyApp.app", 140, 180)
	s.SetIconPosition("Applications", 400, 180)

	err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	data := buf.Bytes()

	// Check file structure
	if string(data[4:8]) != "Bud1" {
		t.Errorf("buddy magic = %q, want \"Bud1\"", string(data[4:8]))
	}

	t.Logf("File with background: %d bytes", len(data))
}

// sortRecords sorts records by filename (case-insensitive) then by code.
func sortRecords(records []Record) {
	for i := 1; i < len(records); i++ {
		for j := i; j > 0 && records[j].Less(records[j-1]); j-- {
			records[j], records[j-1] = records[j-1], records[j]
		}
	}
}
