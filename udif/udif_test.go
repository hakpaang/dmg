package udif

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBuild_StructureCrossPlatform exercises the pure-Go construction path with
// no host dependencies, so it runs (and is meaningful) on any platform.
func TestBuild_StructureCrossPlatform(t *testing.T) {
	// 4 MiB raw image: first sector non-zero, rest zero -> exercises both a
	// zlib chunk and a zero-fill chunk plus the terminator.
	raw := make([]byte, 4*1024*1024)
	copy(raw, []byte("HFS+ would go here"))

	var buf bytes.Buffer
	if err := Build(&buf, raw, "disk image"); err != nil {
		t.Fatalf("Build: %v", err)
	}
	out := buf.Bytes()

	if len(out) < 512 {
		t.Fatalf("output too small: %d bytes", len(out))
	}
	koly := out[len(out)-512:]
	if got := string(koly[0:4]); got != "koly" {
		t.Fatalf("trailer signature = %q, want koly", got)
	}
	// A 4 MiB mostly-zero image must compress far below its raw size.
	if len(out) > len(raw)/2 {
		t.Errorf("output %d bytes did not compress (raw %d)", len(out), len(raw))
	}
}

func TestBuild_RejectsUnalignedImage(t *testing.T) {
	if err := Build(&bytes.Buffer{}, make([]byte, 100), "x"); err == nil {
		t.Fatal("expected error for non-sector-aligned image")
	}
}

// TestBuild_HdiutilVerifyAndMount is the oracle test: it builds a real HFS+
// image with hdiutil, wraps it with our pure-Go writer, and confirms macOS
// both verifies the checksums and can mount and read the result.
func TestBuild_HdiutilVerifyAndMount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("hdiutil oracle requires macOS")
	}
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil not found")
	}

	dir := t.TempDir()

	// Produce a raw HFS+ filesystem image (no partition map) via hdiutil, with
	// a known file inside so we can verify the round-trip.
	udrw := filepath.Join(dir, "src.dmg")
	mustRun(t, "hdiutil", "create", "-size", "8m", "-fs", "HFS+",
		"-volname", "UDIFPOC", "-layout", "NONE", "-type", "UDIF", udrw)

	// Mount the UDRW, drop a file in, detach.
	mnt := attach(t, udrw)
	marker := "hello-from-pure-go-udif"
	if err := os.WriteFile(filepath.Join(mnt, "MARKER.txt"), []byte(marker), 0644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	detach(t, mnt)

	// Convert to UDTO to obtain the RAW filesystem bytes (hdiutil writes a
	// .cdr file that is the unwrapped image). UDRW/UDIF files are themselves
	// containers, not raw sectors.
	rawBase := filepath.Join(dir, "raw")
	mustRun(t, "hdiutil", "convert", udrw, "-format", "UDTO", "-o", rawBase)
	rawImage, err := os.ReadFile(rawBase + ".cdr")
	if err != nil {
		t.Fatalf("read raw image: %v", err)
	}
	if len(rawImage)%sectorSize != 0 {
		t.Fatalf("raw image not sector aligned: %d bytes", len(rawImage))
	}

	// Wrap with our pure-Go writer.
	out := filepath.Join(dir, "out.dmg")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := Build(f, rawImage, "whole disk (Apple_HFS : 0)"); err != nil {
		f.Close()
		t.Fatalf("Build: %v", err)
	}
	f.Close()

	// Oracle 1: hdiutil verify checks the UDIF checksums.
	if out, err := exec.Command("hdiutil", "verify", out).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil verify failed: %v\n%s", err, out)
	}

	// Oracle 2: mount our image and read the marker file back.
	mnt2 := attach(t, out)
	defer detach(t, mnt2)
	got, err := os.ReadFile(filepath.Join(mnt2, "MARKER.txt"))
	if err != nil {
		t.Fatalf("read marker from our image: %v", err)
	}
	if string(got) != marker {
		t.Fatalf("marker = %q, want %q", got, marker)
	}
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func attach(t *testing.T, dmg string) string {
	t.Helper()
	out, err := exec.Command("hdiutil", "attach", dmg, "-nobrowse", "-owners", "off").CombinedOutput()
	if err != nil {
		t.Fatalf("attach %s: %v\n%s", dmg, err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if i := strings.Index(line, "/Volumes/"); i >= 0 {
			return strings.TrimSpace(line[i:])
		}
	}
	t.Fatalf("no mount point in:\n%s", out)
	return ""
}

func detach(t *testing.T, mnt string) {
	t.Helper()
	exec.Command("hdiutil", "detach", mnt, "-force").Run()
}
