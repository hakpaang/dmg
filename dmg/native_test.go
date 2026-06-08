package dmg

import (
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveBackend(t *testing.T) {
	if got := resolveBackend(Options{Backend: BackendNative}); got != BackendNative {
		t.Errorf("explicit native = %v, want BackendNative", got)
	}
	if got := resolveBackend(Options{Backend: BackendHdiutil}); got != BackendHdiutil {
		t.Errorf("explicit hdiutil = %v, want BackendHdiutil", got)
	}
	// Auto resolves to a concrete backend (hdiutil on darwin w/ tool, else native).
	got := resolveBackend(Options{Backend: BackendAuto})
	if got != BackendNative && got != BackendHdiutil {
		t.Errorf("auto = %v, want a concrete backend", got)
	}
}

func TestBuildNative_RejectsUnsupported(t *testing.T) {
	if err := Build(Options{VolumeName: "X", OutputPath: "x.dmg", Backend: BackendNative, Format: FormatULFO}); err == nil {
		t.Error("expected error for non-UDZO format with native backend")
	}
	if err := Build(Options{VolumeName: "X", OutputPath: "x.dmg", Backend: BackendNative, Filesystem: FSAPFS}); err == nil {
		t.Error("expected error for APFS with native backend")
	}
}

// TestBuildNative_EndToEnd builds a styled DMG through dmg.Build using the
// pure-Go backend, then mounts it (via hdiutil, as a reader) to confirm the
// app's code signature survived and the styled layout files are present.
func TestBuildNative_EndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("mounting/codesign oracle requires macOS")
	}
	for _, tool := range []string{"hdiutil", "codesign"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found", tool)
		}
	}

	dir := t.TempDir()

	// Minimal ad-hoc signed app.
	app := filepath.Join(dir, "MyApp.app")
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
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"),
		[]byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>MyApp</string></dict></plist>`), 0644); err != nil {
		t.Fatal(err)
	}
	if o, err := exec.Command("codesign", "--force", "-s", "-", app).CombinedOutput(); err != nil {
		t.Skipf("codesign unavailable: %v\n%s", err, o)
	}

	// A background image to exercise the .DS_Store + alias path.
	bg := filepath.Join(dir, "bg.png")
	if err := SolidBackground(540, 380, color.RGBA{20, 20, 40, 255}, bg); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "MyApp.dmg")
	opts := Options{
		VolumeName:             "MyApp",
		OutputPath:             out,
		Files:                  map[string]string{"MyApp.app": app},
		AddApplicationsSymlink: true,
		Background:             &BackgroundConfig{File: bg},
		Backend:                BackendNative,
	}
	if err := Build(opts); err != nil {
		t.Fatalf("Build (native): %v", err)
	}

	if o, err := exec.Command("hdiutil", "verify", out).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil verify: %v\n%s", err, o)
	}
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

	// Code signature must survive the pure-Go packaging.
	if o, err := exec.Command("codesign", "--verify", "--strict", filepath.Join(mnt, "MyApp.app")).CombinedOutput(); err != nil {
		t.Fatalf("codesign --verify on native DMG: %v\n%s", err, o)
	}
	// Styled layout artifacts must be present.
	if fi, err := os.Lstat(filepath.Join(mnt, "Applications")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Applications symlink missing: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(mnt, ".DS_Store")); err != nil {
		t.Errorf(".DS_Store missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mnt, ".background.png")); err != nil {
		t.Errorf("background image missing: %v", err)
	}
}
