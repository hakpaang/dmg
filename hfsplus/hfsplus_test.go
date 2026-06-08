package hfsplus

import (
	"fmt"
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
	img, err := BuildEmpty("TESTVOL", time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("BuildEmpty: %v", err)
	}
	if len(img) < minVolumeBytes || len(img)%blockSize != 0 {
		t.Fatalf("image size = %d, want >= %d and block-aligned", len(img), minVolumeBytes)
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

func TestBuild_RejectsEmptyName(t *testing.T) {
	if _, err := Build(&Volume{}); err == nil {
		t.Fatal("expected error for empty volume name")
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
	img, err := BuildEmpty("PUREGO", time.Unix(1700000000, 0))
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

// TestBuild_FilesFoldersSymlink is the full oracle test: it builds a volume with
// a nested folder, two files (one empty, one multi-block) and a symlink, mounts
// it, and reads everything back to confirm contents and structure are correct.
func TestBuild_FilesFoldersSymlink(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("hdiutil oracle requires macOS")
	}
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil not found")
	}

	big := make([]byte, 10000) // spans multiple 4096-byte blocks
	for i := range big {
		big[i] = byte(i)
	}

	vol := &Volume{
		Name:      "TREE",
		CreatedAt: time.Unix(1700000000, 0),
		Children: []*Entry{
			{Name: "readme.txt", Kind: KindFile, Data: []byte("hello pure go hfs+")},
			{Name: "empty.dat", Kind: KindFile, Data: nil},
			{Name: "Applications", Kind: KindSymlink, Target: "/Applications"},
			{Name: "sub", Kind: KindDir, Children: []*Entry{
				{Name: "big.bin", Kind: KindFile, Data: big},
			}},
		},
	}

	img, err := Build(vol)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "tree.dmg")
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

	// readme.txt
	if got, _ := os.ReadFile(filepath.Join(mnt, "readme.txt")); string(got) != "hello pure go hfs+" {
		t.Errorf("readme.txt = %q", got)
	}
	// empty.dat
	if got, err := os.ReadFile(filepath.Join(mnt, "empty.dat")); err != nil || len(got) != 0 {
		t.Errorf("empty.dat err=%v len=%d", err, len(got))
	}
	// big.bin in subfolder
	if got, err := os.ReadFile(filepath.Join(mnt, "sub", "big.bin")); err != nil {
		t.Errorf("read big.bin: %v", err)
	} else if len(got) != len(big) || got[9999] != big[9999] {
		t.Errorf("big.bin mismatch: len=%d", len(got))
	}
	// symlink target
	if tgt, err := os.Readlink(filepath.Join(mnt, "Applications")); err != nil {
		t.Errorf("readlink Applications: %v", err)
	} else if tgt != "/Applications" {
		t.Errorf("symlink target = %q, want /Applications", tgt)
	}
}

// TestBuild_ManyFiles forces a multi-level catalog B-tree (many leaf nodes plus
// at least one index node) and verifies every file mounts and reads back.
func TestBuild_ManyFiles(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("hdiutil oracle requires macOS")
	}
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil not found")
	}

	const n = 600
	vol := &Volume{Name: "MANY", CreatedAt: time.Unix(1700000000, 0)}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("file_%04d.txt", i)
		vol.Children = append(vol.Children, &Entry{
			Name: name, Kind: KindFile, Data: []byte(fmt.Sprintf("content-%d", i)),
		})
	}

	img, err := Build(vol)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "many.dmg")
	f, _ := os.Create(out)
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

	ents, err := os.ReadDir(mnt)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(ents) != n {
		t.Errorf("entry count = %d, want %d", len(ents), n)
	}
	// Spot-check a few across the range to exercise B-tree lookups.
	for _, i := range []int{0, 1, 299, 300, 599} {
		name := fmt.Sprintf("file_%04d.txt", i)
		got, err := os.ReadFile(filepath.Join(mnt, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if want := fmt.Sprintf("content-%d", i); string(got) != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
