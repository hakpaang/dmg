package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/leaanthony/dmg/dmg"
)

func main() {
	// Create a dummy .app
	tmpDir, _ := os.MkdirTemp("", "dmgtest")
	defer os.RemoveAll(tmpDir)
	appDir := filepath.Join(tmpDir, "MyApp.app", "Contents", "MacOS")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "MyApp"), []byte("#!/bin/sh\necho hello\n"), 0755)
	appPath := filepath.Join(tmpDir, "MyApp.app")

	// Use unique volume name to avoid Finder caching
	volName := fmt.Sprintf("GoDMG%d", time.Now().Unix())
	outPath := "/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/test-image-bg.dmg"
	os.Remove(outPath)

	err := dmg.Build(dmg.Options{
		VolumeName:             volName,
		OutputPath:             outPath,
		Files:                  map[string]string{"MyApp.app": appPath},
		AddApplicationsSymlink: true,
		Window:                 dmg.WindowConfig{X: 200, Y: 200, Width: 540, Height: 380},
		Icon:                   dmg.IconConfig{Size: 128, TextSize: 12, GridSpace: 100},
		Background:             &dmg.BackgroundConfig{File: "/Users/leaanthony/Downloads/dmg-upstream/test-dmgs/test-bg.png"},
		Format:                 dmg.FormatUDZO,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("OK:", outPath)
	fmt.Println("Volume:", volName)

	// Auto-open
	exec.Command("open", outPath).Run()
}
