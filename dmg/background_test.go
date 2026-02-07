package dmg

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestGradientBackground(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "gradient.png")

	err := GradientBackground(540, 380,
		color.RGBA{R: 40, G: 40, B: 80, A: 255},
		color.RGBA{R: 100, G: 100, B: 200, A: 255},
		GradientTopToBottom, out)
	if err != nil {
		t.Fatalf("GradientBackground: %v", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output PNG is empty")
	}
	t.Logf("Generated gradient: %d bytes", info.Size())
}

func TestGradientDirections(t *testing.T) {
	dir := t.TempDir()
	directions := []GradientDirection{
		GradientTopToBottom,
		GradientLeftToRight,
		GradientTopLeftToBottomRight,
		GradientTopRightToBottomLeft,
	}
	for _, d := range directions {
		out := filepath.Join(dir, "gradient.png")
		err := GradientBackground(100, 100,
			color.RGBA{R: 255, A: 255},
			color.RGBA{B: 255, A: 255},
			d, out)
		if err != nil {
			t.Errorf("direction %d: %v", d, err)
		}
	}
}

func TestSolidBackground(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "solid.png")

	err := SolidBackground(540, 380, color.RGBA{R: 30, G: 30, B: 60, A: 255}, out)
	if err != nil {
		t.Fatalf("SolidBackground: %v", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output PNG is empty")
	}
	t.Logf("Generated solid: %d bytes", info.Size())
}

func TestInstallerBackground(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "installer.png")

	slots := []IconSlot{
		{X: 180, Y: 170, Label: "MyApp"},
		{X: 360, Y: 170, Label: "Applications"},
	}

	err := InstallerBackground(540, 380,
		color.RGBA{R: 30, G: 30, B: 60, A: 255},
		color.RGBA{R: 60, G: 60, B: 120, A: 255},
		slots, 128, out)
	if err != nil {
		t.Fatalf("InstallerBackground: %v", err)
	}

	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("output not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output PNG is empty")
	}
	t.Logf("Generated installer bg: %d bytes", info.Size())
}

func TestAddLogo(t *testing.T) {
	dir := t.TempDir()
	bgPath := filepath.Join(dir, "bg.png")
	logoPath := filepath.Join(dir, "logo.png")
	outPath := filepath.Join(dir, "combined.png")

	// Create a background
	if err := SolidBackground(540, 380, color.RGBA{R: 50, G: 50, B: 100, A: 255}, bgPath); err != nil {
		t.Fatal(err)
	}

	// Create a small "logo" (solid red square)
	if err := SolidBackground(40, 40, color.RGBA{R: 255, A: 255}, logoPath); err != nil {
		t.Fatal(err)
	}

	// Composite
	if err := AddLogo(bgPath, logoPath, 270, 50, outPath); err != nil {
		t.Fatalf("AddLogo: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("output not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output PNG is empty")
	}
	t.Logf("Generated logo composite: %d bytes", info.Size())
}
