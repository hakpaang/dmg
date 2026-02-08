package dmg

import (
	"encoding/binary"
	"testing"
)

func TestBuildResourceFork_Structure(t *testing.T) {
	// Use a small fake ICNS payload
	icnsData := []byte("fake-icns-data-for-testing")
	result := buildResourceFork(icnsData)

	dataLen := len(icnsData)
	dataAreaLen := 4 + dataLen
	expectedTotal := 256 + dataAreaLen + 50
	if len(result) != expectedTotal {
		t.Fatalf("expected total size %d, got %d", expectedTotal, len(result))
	}

	// Verify header
	dataOffset := binary.BigEndian.Uint32(result[0:4])
	if dataOffset != 256 {
		t.Errorf("data offset: expected 256, got %d", dataOffset)
	}

	mapOffset := binary.BigEndian.Uint32(result[4:8])
	expectedMapOffset := uint32(256 + dataAreaLen)
	if mapOffset != expectedMapOffset {
		t.Errorf("map offset: expected %d, got %d", expectedMapOffset, mapOffset)
	}

	dataAreaSize := binary.BigEndian.Uint32(result[8:12])
	if int(dataAreaSize) != dataAreaLen {
		t.Errorf("data area size: expected %d, got %d", dataAreaLen, dataAreaSize)
	}

	mapSize := binary.BigEndian.Uint32(result[12:16])
	if mapSize != 50 {
		t.Errorf("map size: expected 50, got %d", mapSize)
	}

	// Verify data area contains the ICNS payload
	pos := 256
	storedLen := binary.BigEndian.Uint32(result[pos : pos+4])
	if int(storedLen) != dataLen {
		t.Errorf("stored data length: expected %d, got %d", dataLen, storedLen)
	}
	if string(result[pos+4:pos+4+dataLen]) != string(icnsData) {
		t.Error("stored data does not match input ICNS data")
	}

	// Verify resource map
	mpos := int(mapOffset)

	// Type list offset from map
	typeListOffset := binary.BigEndian.Uint16(result[mpos+24 : mpos+26])
	if typeListOffset != 28 {
		t.Errorf("type list offset: expected 28, got %d", typeListOffset)
	}

	// Name list offset
	nameListOffset := binary.BigEndian.Uint16(result[mpos+26 : mpos+28])
	if nameListOffset != 50 {
		t.Errorf("name list offset: expected 50, got %d", nameListOffset)
	}

	// Type list: count
	tlPos := mpos + 28
	typeCount := binary.BigEndian.Uint16(result[tlPos : tlPos+2])
	if typeCount != 0 {
		t.Errorf("type count-1: expected 0, got %d", typeCount)
	}

	// Type entry
	resType := string(result[tlPos+2 : tlPos+6])
	if resType != "icns" {
		t.Errorf("resource type: expected 'icns', got %q", resType)
	}

	resCount := binary.BigEndian.Uint16(result[tlPos+6 : tlPos+8])
	if resCount != 0 {
		t.Errorf("resource count-1: expected 0, got %d", resCount)
	}

	refOffset := binary.BigEndian.Uint16(result[tlPos+8 : tlPos+10])
	if refOffset != 10 {
		t.Errorf("ref list offset: expected 10, got %d", refOffset)
	}

	// Reference entry
	rlPos := tlPos + 10
	resID := binary.BigEndian.Uint16(result[rlPos : rlPos+2])
	if resID != 0xBFC9 {
		t.Errorf("resource ID: expected 0xBFC9 (-16455), got 0x%04X", resID)
	}

	nameOffset := binary.BigEndian.Uint16(result[rlPos+2 : rlPos+4])
	if nameOffset != 0xFFFF {
		t.Errorf("name offset: expected 0xFFFF, got 0x%04X", nameOffset)
	}

	attrs := result[rlPos+4]
	if attrs != 0 {
		t.Errorf("attributes: expected 0, got %d", attrs)
	}

	// 3-byte data offset should be 0
	if result[rlPos+5] != 0 || result[rlPos+6] != 0 || result[rlPos+7] != 0 {
		t.Errorf("data offset: expected 0, got %02x%02x%02x", result[rlPos+5], result[rlPos+6], result[rlPos+7])
	}
}

func TestBuildResourceFork_EmptyData(t *testing.T) {
	result := buildResourceFork([]byte{})
	// Should still produce a valid structure: 256 header + 4 data area + 50 map
	if len(result) != 310 {
		t.Fatalf("expected 310 bytes for empty data, got %d", len(result))
	}

	// Data length should be 0
	storedLen := binary.BigEndian.Uint32(result[256:260])
	if storedLen != 0 {
		t.Errorf("stored data length: expected 0, got %d", storedLen)
	}
}

func TestBuildResourceFork_LargePayload(t *testing.T) {
	// Simulate a realistic ICNS file (~100KB)
	icnsData := make([]byte, 100*1024)
	for i := range icnsData {
		icnsData[i] = byte(i % 256)
	}

	result := buildResourceFork(icnsData)
	expectedTotal := 256 + 4 + len(icnsData) + 50
	if len(result) != expectedTotal {
		t.Fatalf("expected %d bytes, got %d", expectedTotal, len(result))
	}

	// Verify payload integrity
	pos := 256 + 4
	for i := 0; i < len(icnsData); i++ {
		if result[pos+i] != icnsData[i] {
			t.Fatalf("payload mismatch at byte %d", i)
		}
	}
}
