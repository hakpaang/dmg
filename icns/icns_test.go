package icns

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestEncode_ValidICNS(t *testing.T) {
	// Create a 128x128 red image
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	data := buf.Bytes()

	// Verify ICNS magic
	if string(data[0:4]) != "icns" {
		t.Errorf("expected magic 'icns', got %q", string(data[0:4]))
	}

	// Verify total size matches
	totalSize := binary.BigEndian.Uint32(data[4:8])
	if int(totalSize) != len(data) {
		t.Errorf("header size %d != actual size %d", totalSize, len(data))
	}

	// Should contain entries for 128, 64, 32, 16 (4 entries)
	entries := countEntries(data)
	if entries != 4 {
		t.Errorf("expected 4 entries for 128x128 image, got %d", entries)
	}
}

func TestEncode_LargeImage(t *testing.T) {
	// Create a 512x512 image
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Should contain entries for 512, 256, 128, 64, 32, 16 (6 entries)
	entries := countEntries(buf.Bytes())
	if entries != 6 {
		t.Errorf("expected 6 entries for 512x512 image, got %d", entries)
	}
}

func TestEncode_1024Image(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Should contain all 7 standard entries
	entries := countEntries(buf.Bytes())
	if entries != 7 {
		t.Errorf("expected 7 entries for 1024x1024 image, got %d", entries)
	}
}

func TestEncode_TooSmall(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))

	var buf bytes.Buffer
	err := Encode(&buf, img)
	if err == nil {
		t.Fatal("expected error for 8x8 image")
	}
}

func TestEncode_NonSquare(t *testing.T) {
	// Create a 200x100 image - should be center-cropped to 100x100
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// 100x100 should produce entries for 64, 32, 16 (3 entries)
	entries := countEntries(buf.Bytes())
	if entries != 3 {
		t.Errorf("expected 3 entries for 100x100 cropped image, got %d", entries)
	}
}

func TestEncode_EntryTypes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))

	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	types := extractEntryTypes(buf.Bytes())
	expected := []string{"ic08", "ic07", "icp6", "icp5", "icp4"}
	if len(types) != len(expected) {
		t.Fatalf("expected %d types, got %d: %v", len(expected), len(types), types)
	}
	for i, exp := range expected {
		if types[i] != exp {
			t.Errorf("entry %d: expected type %q, got %q", i, exp, types[i])
		}
	}
}

func TestIcnsFromImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	data, err := IcnsFromImage(img)
	if err != nil {
		t.Fatalf("IcnsFromImage failed: %v", err)
	}
	if len(data) < 8 {
		t.Fatal("output too short")
	}
	if string(data[0:4]) != "icns" {
		t.Errorf("expected magic 'icns', got %q", string(data[0:4]))
	}
}

func TestResizeImage_SameSize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 128, G: 64, B: 32, A: 255})
		}
	}

	result := resizeImage(img, 64)
	bounds := result.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("expected 64x64, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestResizeImage_Downscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	result := resizeImage(img, 32)
	bounds := result.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Errorf("expected 32x32, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Check a pixel - should be approximately the source color
	r, g, b, a := result.At(16, 16).RGBA()
	if r>>8 < 190 || r>>8 > 210 {
		t.Errorf("unexpected red value: %d", r>>8)
	}
	if g>>8 < 90 || g>>8 > 110 {
		t.Errorf("unexpected green value: %d", g>>8)
	}
	if b>>8 < 40 || b>>8 > 60 {
		t.Errorf("unexpected blue value: %d", b>>8)
	}
	if a>>8 != 255 {
		t.Errorf("unexpected alpha value: %d", a>>8)
	}
}

func TestCropToSquare(t *testing.T) {
	// Wide image
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	cropped := cropToSquare(img)
	if cropped.Bounds().Dx() != 100 || cropped.Bounds().Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", cropped.Bounds().Dx(), cropped.Bounds().Dy())
	}

	// Tall image
	img2 := image.NewRGBA(image.Rect(0, 0, 50, 150))
	cropped2 := cropToSquare(img2)
	if cropped2.Bounds().Dx() != 50 || cropped2.Bounds().Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", cropped2.Bounds().Dx(), cropped2.Bounds().Dy())
	}

	// Already square
	img3 := image.NewRGBA(image.Rect(0, 0, 64, 64))
	cropped3 := cropToSquare(img3)
	if cropped3.Bounds().Dx() != 64 || cropped3.Bounds().Dy() != 64 {
		t.Errorf("expected 64x64, got %dx%d", cropped3.Bounds().Dx(), cropped3.Bounds().Dy())
	}
}

// countEntries counts the number of icon entries in ICNS data.
func countEntries(data []byte) int {
	count := 0
	offset := 8 // skip header
	for offset+8 <= len(data) {
		entryLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		if entryLen < 8 || offset+entryLen > len(data) {
			break
		}
		count++
		offset += entryLen
	}
	return count
}

// extractEntryTypes returns the OSType codes of all entries in ICNS data.
func extractEntryTypes(data []byte) []string {
	var types []string
	offset := 8
	for offset+8 <= len(data) {
		osType := string(data[offset : offset+4])
		entryLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		if entryLen < 8 || offset+entryLen > len(data) {
			break
		}
		types = append(types, osType)
		offset += entryLen
	}
	return types
}
