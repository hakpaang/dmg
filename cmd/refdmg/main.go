package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// This tool creates a reference DMG using AppleScript to set Finder window styling.
// The resulting .DS_Store can be compared against our programmatic one.
func main() {
	tmpDir, _ := os.MkdirTemp("", "refdmg")
	defer os.RemoveAll(tmpDir)

	// Create dummy app
	appDir := filepath.Join(tmpDir, "MyApp.app", "Contents", "MacOS")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "MyApp"), []byte("#!/bin/sh\necho hello\n"), 0755)

	writableDMG := filepath.Join(tmpDir, "ref.writable.dmg")
	finalDMG := "/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/reference-applescript.dmg"
	refDSStore := "/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/reference.DS_Store"

	// Create writable DMG
	fmt.Println("Creating writable DMG...")
	run("hdiutil", "create", "-volname", "Reference App", "-fs", "HFS+", "-size", "32m", writableDMG)

	// Mount
	fmt.Println("Mounting...")
	out, _ := exec.Command("hdiutil", "attach", writableDMG, "-owners", "off").CombinedOutput()
	mountPoint := ""
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.Index(line, "/Volumes/"); idx >= 0 {
			mountPoint = strings.TrimSpace(line[idx:])
		}
	}
	if mountPoint == "" {
		fmt.Fprintf(os.Stderr, "Failed to mount: %s\n", out)
		os.Exit(1)
	}
	fmt.Printf("Mounted at: %s\n", mountPoint)

	// Copy app
	run("/usr/bin/ditto", filepath.Join(tmpDir, "MyApp.app"), filepath.Join(mountPoint, "MyApp.app"))

	// Create Applications symlink
	os.Symlink("/Applications", filepath.Join(mountPoint, "Applications"))

	// Use AppleScript to configure Finder window
	fmt.Println("Configuring Finder window via AppleScript...")
	appleScript := fmt.Sprintf(`
tell application "Finder"
	tell disk "%s"
		open
		set current view of container window to icon view
		set toolbar visible of container window to false
		set statusbar visible of container window to false
		set the bounds of container window to {200, 200, 800, 600}
		set viewOptions to the icon view options of container window
		set arrangement of viewOptions to not arranged
		set icon size of viewOptions to 128
		set text size of viewOptions to 12
		set background color of viewOptions to {65535, 65535, 65535}
		set position of item "MyApp.app" of container window to {140, 180}
		set position of item "Applications" of container window to {400, 180}
		close
		open
		update without registering applications
		delay 2
		close
	end tell
end tell
`, "Reference App")

	cmd := exec.Command("osascript", "-e", appleScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "AppleScript failed: %v\n", err)
		// Continue anyway - partial styling may have been applied
	}

	time.Sleep(2 * time.Second)

	// Copy the .DS_Store for analysis
	fmt.Println("Saving reference .DS_Store...")
	dsStorePath := filepath.Join(mountPoint, ".DS_Store")
	if data, err := os.ReadFile(dsStorePath); err == nil {
		os.WriteFile(refDSStore, data, 0644)
		fmt.Printf("Reference .DS_Store saved: %d bytes -> %s\n", len(data), refDSStore)
	} else {
		fmt.Fprintf(os.Stderr, "No .DS_Store found: %v\n", err)
	}

	// Detach
	fmt.Println("Detaching...")
	for i := 0; i < 5; i++ {
		err := exec.Command("hdiutil", "detach", mountPoint).Run()
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i+1) * time.Second)
		if i >= 3 {
			exec.Command("hdiutil", "detach", mountPoint, "-force").Run()
		}
	}

	// Convert to UDZO
	os.Remove(finalDMG)
	fmt.Println("Converting to UDZO...")
	run("hdiutil", "convert", writableDMG, "-format", "UDZO", "-o", finalDMG, "-imagekey", "zlib-level=9")

	fmt.Printf("Reference DMG: %s\n", finalDMG)
	fmt.Printf("Reference .DS_Store: %s\n", refDSStore)
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v failed: %v\n%s\n", name, args, err, out)
		os.Exit(1)
	}
}
