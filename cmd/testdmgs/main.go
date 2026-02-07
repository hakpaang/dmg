package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
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

	// Create background images
	bgBlue := filepath.Join(tmpDir, "bg-blue.png")
	createGradientImage(bgBlue, 600, 400, color.RGBA{30, 60, 120, 255}, color.RGBA{80, 140, 220, 255})

	bgDark := filepath.Join(tmpDir, "bg-dark.png")
	createGradientImage(bgDark, 540, 380, color.RGBA{30, 30, 30, 255}, color.RGBA{60, 60, 80, 255})

	bgGreen := filepath.Join(tmpDir, "bg-green.png")
	createGradientImage(bgGreen, 700, 500, color.RGBA{20, 80, 20, 255}, color.RGBA{60, 180, 80, 255})

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

func createGradientImage(path string, w, h int, topColor, bottomColor color.RGBA) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		r := uint8(float64(topColor.R)*(1-t) + float64(bottomColor.R)*t)
		g := uint8(float64(topColor.G)*(1-t) + float64(bottomColor.G)*t)
		b := uint8(float64(topColor.B)*(1-t) + float64(bottomColor.B)*t)
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	f, _ := os.Create(path)
	defer f.Close()
	png.Encode(f, img)
}
