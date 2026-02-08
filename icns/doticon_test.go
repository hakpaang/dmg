package icns

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestReadDotIcon_SingleLayer(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "Test.icon")
	assetsDir := filepath.Join(iconDir, "Assets")
	os.MkdirAll(assetsDir, 0755)

	// Create a single layer PNG
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	writePNG(t, filepath.Join(assetsDir, "foreground.png"), img)

	result, err := ReadDotIcon(iconDir)
	if err != nil {
		t.Fatalf("ReadDotIcon failed: %v", err)
	}

	bounds := result.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("expected 64x64, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Check that the red pixel is present
	r, _, _, _ := result.At(32, 32).RGBA()
	if r>>8 < 200 {
		t.Errorf("expected red pixel, got R=%d", r>>8)
	}
}

func TestReadDotIcon_MultipleLayers(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "Test.icon")

	// Create layers in subdirectories matching common patterns
	backDir := filepath.Join(iconDir, "Assets", "Back")
	frontDir := filepath.Join(iconDir, "Assets", "Front")
	os.MkdirAll(backDir, 0755)
	os.MkdirAll(frontDir, 0755)

	// Back layer: solid blue
	back := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			back.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	writePNG(t, filepath.Join(backDir, "Content.png"), back)

	// Front layer: semi-transparent red in center
	front := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 32; y < 96; y++ {
		for x := 32; x < 96; x++ {
			front.SetRGBA(x, y, color.RGBA{R: 255, A: 128})
		}
	}
	writePNG(t, filepath.Join(frontDir, "Content.png"), front)

	result, err := ReadDotIcon(iconDir)
	if err != nil {
		t.Fatalf("ReadDotIcon failed: %v", err)
	}

	bounds := result.Bounds()
	if bounds.Dx() != 128 || bounds.Dy() != 128 {
		t.Errorf("expected 128x128, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Corner should be pure blue (only back layer)
	_, _, b, _ := result.At(0, 0).RGBA()
	if b>>8 < 200 {
		t.Errorf("corner should be blue, got B=%d", b>>8)
	}

	// Center should be a blend of red and blue
	r, _, _, _ := result.At(64, 64).RGBA()
	if r>>8 < 50 {
		t.Errorf("center should have red component, got R=%d", r>>8)
	}
}

func TestReadDotIcon_WithIconJSON(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "Test.icon")
	assetsDir := filepath.Join(iconDir, "Assets")
	os.MkdirAll(assetsDir, 0755)

	// Create a simple icon.json referencing an image
	jsonContent := `{"layers": [{"filename": "icon.png"}]}`
	os.WriteFile(filepath.Join(iconDir, "icon.json"), []byte(jsonContent), 0644)

	// Create the referenced image
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.SetRGBA(x, y, color.RGBA{G: 255, A: 255})
		}
	}
	writePNG(t, filepath.Join(assetsDir, "icon.png"), img)

	result, err := ReadDotIcon(iconDir)
	if err != nil {
		t.Fatalf("ReadDotIcon failed: %v", err)
	}

	// Should have found the image via JSON reference
	_, g, _, _ := result.At(16, 16).RGBA()
	if g>>8 < 200 {
		t.Errorf("expected green pixel, got G=%d", g>>8)
	}
}

func TestReadDotIcon_NotDirectory(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test*.icon")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, err = ReadDotIcon(f.Name())
	if err == nil {
		t.Error("expected error for non-directory .icon path")
	}
}

func TestReadDotIcon_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	iconDir := filepath.Join(dir, "Empty.icon")
	os.MkdirAll(iconDir, 0755)

	_, err := ReadDotIcon(iconDir)
	if err == nil {
		t.Error("expected error for empty .icon directory")
	}
}

func TestLayerOrder(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		{"/Assets/Back/Content.png", 0},
		{"/Assets/background/icon.png", 0},
		{"/Assets/Middle/Content.png", 1},
		{"/Assets/midground/icon.png", 1},
		{"/Assets/Front/Content.png", 2},
		{"/Assets/foreground/icon.png", 2},
		{"/Assets/unknown/icon.png", 1}, // default to middle
	}

	for _, tt := range tests {
		got := layerOrder(tt.path)
		if got != tt.expected {
			t.Errorf("layerOrder(%q) = %d, want %d", tt.path, got, tt.expected)
		}
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
