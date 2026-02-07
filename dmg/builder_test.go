package dmg

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateOptions(t *testing.T) {
	// Missing volume name
	opts := Options{OutputPath: "test.dmg"}
	if err := validateOptions(&opts); err == nil {
		t.Error("expected error for missing volume name")
	}

	// Missing output path
	opts = Options{VolumeName: "Test"}
	if err := validateOptions(&opts); err == nil {
		t.Error("expected error for missing output path")
	}

	// Valid options with defaults applied
	opts = Options{VolumeName: "Test", OutputPath: "test.dmg"}
	if err := validateOptions(&opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Format != FormatUDZO {
		t.Errorf("default format = %s, want UDZO", opts.Format)
	}
	if opts.Filesystem != FSHFSPlus {
		t.Errorf("default filesystem = %s, want HFS+", opts.Filesystem)
	}
	if opts.Window.Width != 540 {
		t.Errorf("default window width = %d, want 540", opts.Window.Width)
	}
	if opts.Icon.Size != 128 {
		t.Errorf("default icon size = %d, want 128", opts.Icon.Size)
	}
}

func TestCalculateSize(t *testing.T) {
	// Create a temp file to use as source
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "test.app"))
	f.Write(make([]byte, 10*1024*1024)) // 10MB
	f.Close()

	opts := Options{
		Files: map[string]string{
			"test.app": filepath.Join(dir, "test.app"),
		},
	}
	size := calculateSize(opts)
	// Should be at least 32MB (our minimum) and account for the 10MB file
	if size < 32 {
		t.Errorf("calculated size = %d MB, want >= 32", size)
	}
	t.Logf("Calculated DMG size: %d MB for 10MB source", size)
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions("/path/to/app.app", "output.dmg")
	if opts.VolumeName != "Application" {
		t.Errorf("volume name = %q, want Application", opts.VolumeName)
	}
	if opts.Format != FormatUDZO {
		t.Errorf("format = %s, want UDZO", opts.Format)
	}
	if !opts.AddApplicationsSymlink {
		t.Error("AddApplicationsSymlink should be true by default")
	}
	if opts.Window.Width != 540 || opts.Window.Height != 380 {
		t.Errorf("window = %dx%d, want 540x380", opts.Window.Width, opts.Window.Height)
	}
}

// TestBuildIntegration is an integration test that actually creates a DMG.
// It requires macOS and hdiutil to be available.
func TestBuildIntegration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("DMG creation requires macOS")
	}
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil not found")
	}

	// Create temp source directory with a dummy app
	srcDir := t.TempDir()
	appDir := filepath.Join(srcDir, "TestApp.app", "Contents", "MacOS")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "TestApp"), []byte("#!/bin/sh\necho hello\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a simple background image (1x1 pixel PNG)
	bgDir := t.TempDir()
	bgPath := filepath.Join(bgDir, "background.png")
	// Minimal valid 1x1 white PNG
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(bgPath, pngData, 0644); err != nil {
		t.Fatal(err)
	}

	// Build DMG
	outputPath := filepath.Join(t.TempDir(), "TestApp.dmg")
	opts := Options{
		VolumeName: "Test App",
		OutputPath: outputPath,
		Files: map[string]string{
			"TestApp.app": filepath.Join(srcDir, "TestApp.app"),
		},
		AddApplicationsSymlink: true,
		Window: WindowConfig{
			X: 100, Y: 100, Width: 540, Height: 380,
		},
		Icon: IconConfig{
			Size:      128,
			TextSize:  12,
			GridSpace: 100,
		},
		Background: &BackgroundConfig{
			File: bgPath,
		},
		Format: FormatUDZO,
	}

	err := Build(opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify DMG was created
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output DMG not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("output DMG is empty")
	}
	t.Logf("Created DMG: %s (%d bytes)", outputPath, info.Size())

	// Mount and verify contents
	mountPoint, err := hdiutilAttach(outputPath)
	if err != nil {
		t.Fatalf("failed to mount DMG: %v", err)
	}
	defer hdiutilDetach(mountPoint)

	// Check .app exists
	if _, err := os.Stat(filepath.Join(mountPoint, "TestApp.app")); err != nil {
		t.Error("TestApp.app not found in DMG")
	}

	// Check Applications symlink
	target, err := os.Readlink(filepath.Join(mountPoint, "Applications"))
	if err != nil {
		t.Error("Applications symlink not found in DMG")
	} else if target != "/Applications" {
		t.Errorf("Applications symlink target = %q, want /Applications", target)
	}

	// Check .DS_Store exists
	if _, err := os.Stat(filepath.Join(mountPoint, ".DS_Store")); err != nil {
		t.Error(".DS_Store not found in DMG")
	}

	// Check background image exists at root (e.g. .background.png)
	if _, err := os.Stat(filepath.Join(mountPoint, ".background.png")); err != nil {
		t.Error(".background.png not found in DMG")
	}

	t.Log("DMG verification complete: all contents present")
}
