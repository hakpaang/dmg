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
	// Volume header signature 'H+' (case-insensitive default) lives at offset 1024.
	if sig := be.Uint16(img[1024:1026]); sig != sigHFSPlus {
		t.Errorf("signature = %#x, want %#x (H+)", sig, sigHFSPlus)
	}
	if ver := be.Uint16(img[1026:1028]); ver != versionHFSPlus {
		t.Errorf("version = %d, want %d", ver, versionHFSPlus)
	}
	// Alternate volume header must match the primary.
	if sig := be.Uint16(img[len(img)-1024 : len(img)-1022]); sig != sigHFSPlus {
		t.Errorf("alternate header signature = %#x, want %#x", sig, sigHFSPlus)
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

// dirToEntries walks a real directory into Entry children, preserving file
// modes and symlinks. Test-only helper (uses host calls).
func dirToEntries(t *testing.T, dir string) []*Entry {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var out []*Entry
	for _, de := range des {
		p := filepath.Join(dir, de.Name())
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			tgt, _ := os.Readlink(p)
			out = append(out, &Entry{Name: de.Name(), Kind: KindSymlink, Target: tgt})
		case fi.IsDir():
			out = append(out, &Entry{Name: de.Name(), Kind: KindDir, Children: dirToEntries(t, p)})
		default:
			data, _ := os.ReadFile(p)
			out = append(out, &Entry{Name: de.Name(), Kind: KindFile, Data: data, Mode: uint16(fi.Mode().Perm())})
		}
	}
	return out
}

// TestBuild_CodesignSurvives is the strongest faithfulness gate: it ad-hoc signs
// a real .app bundle, packages it through the pure-Go chain, mounts it, and
// confirms the code signature still validates — proving every byte and the exec
// bit survived. Runs in both case modes.
func TestBuild_CodesignSurvives(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("codesign oracle requires macOS")
	}
	for _, tool := range []string{"codesign", "hdiutil"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found", tool)
		}
	}

	// Build a minimal ad-hoc signed app bundle.
	app := filepath.Join(t.TempDir(), "MyApp.app")
	macos := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0755); err != nil {
		t.Fatal(err)
	}
	exe, err := os.ReadFile("/bin/echo")
	if err != nil {
		t.Skip("/bin/echo unavailable")
	}
	if err := os.WriteFile(filepath.Join(macos, "MyApp"), exe, 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>MyApp</string>` +
		`<key>CFBundleIdentifier</key><string>com.example.myapp</string>` +
		`<key>CFBundlePackageType</key><string>APPL</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	if o, err := exec.Command("codesign", "--force", "-s", "-", app).CombinedOutput(); err != nil {
		t.Skipf("codesign unavailable in this environment: %v\n%s", err, o)
	}

	for _, caseSensitive := range []bool{false, true} {
		t.Run(map[bool]string{false: "insensitive", true: "sensitive"}[caseSensitive], func(t *testing.T) {
			vol := &Volume{
				Name:          "APP",
				CreatedAt:     time.Unix(1700000000, 0),
				CaseSensitive: caseSensitive,
				Children:      []*Entry{{Name: "MyApp.app", Kind: KindDir, Children: dirToEntries(t, app)}},
			}
			img, err := Build(vol)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			out := filepath.Join(t.TempDir(), "app.dmg")
			f, _ := os.Create(out)
			if err := udif.Build(f, img, "whole disk (Apple_HFS : 0)"); err != nil {
				f.Close()
				t.Fatalf("udif.Build: %v", err)
			}
			f.Close()

			o, err := exec.Command("hdiutil", "attach", out, "-nobrowse", "-owners", "off").CombinedOutput()
			if err != nil {
				t.Fatalf("attach: %v\n%s", err, o)
			}
			var mnt string
			for _, line := range strings.Split(strings.TrimSpace(string(o)), "\n") {
				if i := strings.Index(line, "/Volumes/"); i >= 0 {
					mnt = strings.TrimSpace(line[i:])
				}
			}
			defer exec.Command("hdiutil", "detach", mnt, "-force").Run()

			if o, err := exec.Command("codesign", "--verify", "--strict", mnt+"/MyApp.app").CombinedOutput(); err != nil {
				t.Fatalf("codesign --verify failed on mounted app: %v\n%s", err, o)
			}
		})
	}
}
