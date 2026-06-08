package hfsplus

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/leaanthony/dmg/udif"
)

// TestBuildEmpty_StructureCrossPlatform validates the pure-Go construction with
// no host dependencies, so it is meaningful on any platform / cross-compile.
func TestBuildEmpty_StructureCrossPlatform(t *testing.T) {
	img, err := BuildEmpty("TESTVOL", 8*1024*1024, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("BuildEmpty: %v", err)
	}
	if len(img) != 8*1024*1024 {
		t.Fatalf("image size = %d, want %d", len(img), 8*1024*1024)
	}
	// Volume header signature 'HX' lives at offset 1024.
	if sig := be.Uint16(img[1024:1026]); sig != sigHFSX {
		t.Errorf("signature = %#x, want %#x (HX)", sig, sigHFSX)
	}
	if ver := be.Uint16(img[1026:1028]); ver != versionHFSX {
		t.Errorf("version = %d, want %d", ver, versionHFSX)
	}
	// Alternate volume header must match the primary.
	if sig := be.Uint16(img[len(img)-1024 : len(img)-1022]); sig != sigHFSX {
		t.Errorf("alternate header signature = %#x, want %#x", sig, sigHFSX)
	}
	if bs := be.Uint32(img[1024+40 : 1024+44]); bs != blockSize {
		t.Errorf("blockSize = %d, want %d", bs, blockSize)
	}
}

func TestBuildEmpty_RejectsBadSize(t *testing.T) {
	if _, err := BuildEmpty("X", 100, time.Unix(0, 0)); err == nil {
		t.Fatal("expected error for non-block-aligned size")
	}
}

// TestBuildEmpty_HdiutilMount is the oracle test: it builds a pure-Go HFSX
// volume, wraps it with the pure-Go udif writer, and confirms macOS verifies
// and mounts the result with the expected volume name.
func TestBuildEmpty_HdiutilMount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("hdiutil oracle requires macOS")
	}
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil not found")
	}

	dir := t.TempDir()
	img, err := BuildEmpty("PUREGO", 8*1024*1024, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("BuildEmpty: %v", err)
	}

	out := filepath.Join(dir, "vol.dmg")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := udif.Build(f, img, "whole disk (Apple_HFS : 0)"); err != nil {
		f.Close()
		t.Fatalf("udif.Build: %v", err)
	}
	f.Close()

	if o, err := exec.Command("hdiutil", "verify", out).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil verify failed: %v\n%s", err, o)
	}

	o, err := exec.Command("hdiutil", "attach", out, "-nobrowse", "-owners", "off").CombinedOutput()
	if err != nil {
		t.Fatalf("hdiutil attach failed: %v\n%s", err, o)
	}
	var mnt string
	for _, line := range strings.Split(strings.TrimSpace(string(o)), "\n") {
		if i := strings.Index(line, "/Volumes/"); i >= 0 {
			mnt = strings.TrimSpace(line[i:])
		}
	}
	if mnt == "" {
		t.Fatalf("no mount point in:\n%s", o)
	}
	defer exec.Command("hdiutil", "detach", mnt, "-force").Run()

	if want := "/Volumes/PUREGO"; mnt != want {
		t.Errorf("mounted at %q, want %q", mnt, want)
	}
}
