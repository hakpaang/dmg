package dmg_test

import (
	"github.com/leaanthony/dmg/dmg"
)

// Build a styled application DMG with a background image, an Applications
// drag-link, and a custom volume icon. This path uses macOS hdiutil.
func ExampleBuild() {
	opts := dmg.Options{
		VolumeName:             "MyApp",
		OutputPath:             "MyApp.dmg",
		Files:                  map[string]string{"MyApp.app": "/path/to/MyApp.app"},
		AddApplicationsSymlink: true,
		Window:                 dmg.WindowConfig{X: 200, Y: 200, Width: 540, Height: 380},
		Icon:                   dmg.IconConfig{Size: 128, TextSize: 12, GridSpace: 100},
		Background:             &dmg.BackgroundConfig{File: "/path/to/background.png"},
		VolumeIcon:             "/path/to/icon.png", // auto-converted to ICNS
		Format:                 dmg.FormatUDZO,
	}
	if err := dmg.Build(opts); err != nil {
		panic(err)
	}
}

// DefaultOptions gives a sensible starting point for a typical app DMG; tweak
// the returned Options and add your app to Files.
func ExampleDefaultOptions() {
	opts := dmg.DefaultOptions("/path/to/MyApp.app", "MyApp.dmg")
	opts.VolumeName = "MyApp"
	opts.Files["MyApp.app"] = "/path/to/MyApp.app"
	if err := dmg.Build(opts); err != nil {
		panic(err)
	}
}
