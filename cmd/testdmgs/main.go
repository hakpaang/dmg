package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/leaanthony/dmg/dmg"
)

func main() {
	outDir := "/Users/leaanthony/Downloads/dmg-upstream/test-dmgs"
	os.MkdirAll(outDir, 0755)

	// Create a dummy .app
	tmpDir, _ := os.MkdirTemp("", "dmgtest")
	defer os.RemoveAll(tmpDir)
	appDir := filepath.Join(tmpDir, "MyApp.app", "Contents", "MacOS")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "MyApp"), []byte("#!/bin/sh\necho hello\n"), 0755)
	appPath := filepath.Join(tmpDir, "MyApp.app")

	// Create background images using the dmg package helpers
	bgBlue := filepath.Join(tmpDir, "bg-blue.png")
	dmg.GradientBackground(600, 400,
		color.RGBA{R: 30, G: 60, B: 120, A: 255},
		color.RGBA{R: 80, G: 140, B: 220, A: 255},
		dmg.GradientTopToBottom, bgBlue)

	bgDark := filepath.Join(tmpDir, "bg-dark.png")
	dmg.GradientBackground(540, 380,
		color.RGBA{R: 30, G: 30, B: 30, A: 255},
		color.RGBA{R: 60, G: 60, B: 80, A: 255},
		dmg.GradientTopToBottom, bgDark)

	bgGreen := filepath.Join(tmpDir, "bg-green.png")
	dmg.GradientBackground(700, 500,
		color.RGBA{R: 20, G: 80, B: 20, A: 255},
		color.RGBA{R: 60, G: 180, B: 80, A: 255},
		dmg.GradientTopToBottom, bgGreen)

	// Create an installer-style background with icon wells and arrow
	bgInstaller := filepath.Join(tmpDir, "bg-installer.png")
	dmg.InstallerBackground(540, 380,
		color.RGBA{R: 40, G: 40, B: 70, A: 255},
		color.RGBA{R: 70, G: 70, B: 120, A: 255},
		[]dmg.IconSlot{
			{X: 180, Y: 170, Label: "App"},
			{X: 360, Y: 170, Label: "Applications"},
		}, 128, bgInstaller)

	tests := []struct {
		name string
		opts dmg.Options
	}{
		{
			name: "01-blue-bg-large-icons",
			opts: dmg.Options{
				VolumeName: "Blue Background",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 200, Y: 200, Width: 600, Height: 400},
				Icon:       dmg.IconConfig{Size: 128, TextSize: 12, GridSpace: 100},
				Background: &dmg.BackgroundConfig{File: bgBlue},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "02-red-color-bg",
			opts: dmg.Options{
				VolumeName: "Red Color BG",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 200, Y: 200, Width: 540, Height: 380},
				Icon:       dmg.IconConfig{Size: 96, TextSize: 11, GridSpace: 80},
				Background: &dmg.BackgroundConfig{Color: &dmg.ColorRGB{R: 0.85, G: 0.2, B: 0.2}},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "03-white-color-bg-big-icons",
			opts: dmg.Options{
				VolumeName: "White BG Big Icons",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 100, Y: 100, Width: 640, Height: 480},
				Icon:       dmg.IconConfig{Size: 160, TextSize: 14, GridSpace: 120},
				Background: &dmg.BackgroundConfig{Color: &dmg.ColorRGB{R: 1.0, G: 1.0, B: 1.0}},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "04-dark-bg-small-icons",
			opts: dmg.Options{
				VolumeName: "Dark Theme",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 200, Y: 200, Width: 540, Height: 380},
				Icon:       dmg.IconConfig{Size: 64, TextSize: 10, GridSpace: 80},
				Background: &dmg.BackgroundConfig{File: bgDark},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "05-green-bg-wide-window",
			opts: dmg.Options{
				VolumeName: "Wide Green",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 100, Y: 100, Width: 700, Height: 500},
				Icon:       dmg.IconConfig{Size: 128, TextSize: 13, GridSpace: 110},
				Background: &dmg.BackgroundConfig{File: bgGreen},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "06-no-bg-default-style",
			opts: dmg.Options{
				VolumeName: "No Background",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 200, Y: 200, Width: 540, Height: 380},
				Icon:       dmg.IconConfig{Size: 128, TextSize: 12, GridSpace: 100},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "07-yellow-color-no-apps-link",
			opts: dmg.Options{
				VolumeName: "Yellow No Link",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: false,
				Window:     dmg.WindowConfig{X: 150, Y: 150, Width: 500, Height: 350},
				Icon:       dmg.IconConfig{Size: 96, TextSize: 11, GridSpace: 90},
				Background: &dmg.BackgroundConfig{Color: &dmg.ColorRGB{R: 1.0, G: 0.9, B: 0.3}},
				Format:     dmg.FormatUDZO,
			},
		},
		{
			name: "08-installer-style",
			opts: dmg.Options{
				VolumeName: "Installer Style",
				Files:      map[string]string{"MyApp.app": appPath},
				AddApplicationsSymlink: true,
				Window:     dmg.WindowConfig{X: 200, Y: 200, Width: 540, Height: 380},
				Icon:       dmg.IconConfig{Size: 128, TextSize: 12, GridSpace: 100},
				Background: &dmg.BackgroundConfig{File: bgInstaller},
				Format:     dmg.FormatUDZO,
			},
		},
	}

	for _, tt := range tests {
		outPath := filepath.Join(outDir, tt.name+".dmg")
		// Remove previous if exists
		os.Remove(outPath)
		tt.opts.OutputPath = outPath
		fmt.Printf("Building %s ...\n", tt.name)
		if err := dmg.Build(tt.opts); err != nil {
			fmt.Fprintf(os.Stderr, "  FAILED: %v\n", err)
			continue
		}
		info, _ := os.Stat(outPath)
		fmt.Printf("  OK: %s (%.1f KB)\n", outPath, float64(info.Size())/1024)
	}

	fmt.Println("\nAll DMGs created in:", outDir)
	fmt.Println("Open them with: open /Users/leaanthony/Downloads/dmg-upstream/test-dmgs/*.dmg")
}

