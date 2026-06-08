// Command styled-dmg builds a styled application .dmg with a gradient background,
// positioned icons, and an /Applications drag-link using the high-level dmg
// package. This path uses macOS hdiutil for image creation, so it must run on
// macOS (for a cross-platform image, see ../purego-dmg).
//
// Usage:
//
//	styled-dmg <App.app> <output.dmg>
package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"github.com/leaanthony/dmg/dmg"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: styled-dmg <App.app> <output.dmg>")
		os.Exit(2)
	}
	appPath, out := os.Args[1], os.Args[2]
	appName := filepath.Base(appPath)

	// Generate a simple gradient background sized to the Finder window.
	const w, h = 540, 380
	bg := filepath.Join(os.TempDir(), "dmg-background.png")
	if err := dmg.GradientBackground(w, h,
		color.RGBA{30, 40, 80, 255}, color.RGBA{10, 15, 35, 255},
		dmg.GradientTopToBottom, bg); err != nil {
		fatal(err)
	}
	defer os.Remove(bg)

	opts := dmg.Options{
		VolumeName:             appName,
		OutputPath:             out,
		Files:                  map[string]string{appName: appPath},
		AddApplicationsSymlink: true,
		Window:                 dmg.WindowConfig{X: 200, Y: 200, Width: w, Height: h},
		Icon:                   dmg.IconConfig{Size: 128, TextSize: 12, GridSpace: 100},
		Background:             &dmg.BackgroundConfig{File: bg},
		Format:                 dmg.FormatUDZO,
	}

	if err := dmg.Build(opts); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote styled DMG %s\n", out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
