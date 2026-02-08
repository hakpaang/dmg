# dmg

A pure Go library for creating styled macOS DMG disk images with custom Finder window appearance.

## Features

- Custom Finder window position and size
- Background images (PNG, JPEG, etc.)
- Solid color backgrounds
- Automatic icon positioning
- Applications symlink for drag-to-install
- Multiple compression formats (zlib, LZFSE, LZMA, bzip2)
- HFS+ and APFS filesystem support
- Volume icons (icon shown when DMG is mounted)
- DMG file icons (icon shown on the .dmg file in Finder)
- Accepts `.icns`, `.png`, `.jpg`, and macOS Tahoe `.icon` packages as icon input
- Pure Go icon handling via extended attributes (no Xcode/`SetFile` dependency)
- Pure Go .DS_Store generation (no AppleScript/Finder scripting)
- ICNS generation from PNG images (`icns` sub-package)

## Installation

```bash
go get github.com/leaanthony/dmg
```

## Usage

```go
package main

import "github.com/leaanthony/dmg/dmg"

func main() {
    opts := dmg.Options{
        VolumeName: "MyApp",
        OutputPath: "MyApp.dmg",
        Files: map[string]string{
            "MyApp.app": "/path/to/MyApp.app",
        },
        AddApplicationsSymlink: true,
        Window: dmg.WindowConfig{
            X: 200, Y: 200,
            Width: 540, Height: 380,
        },
        Icon: dmg.IconConfig{
            Size: 128, TextSize: 12, GridSpace: 100,
        },
        Background: &dmg.BackgroundConfig{
            File: "/path/to/background.png",
        },
        VolumeIcon: "icon.png",     // icon when DMG is mounted
        FileIcon:   "icon.png",     // icon of the .dmg file in Finder
        Format:     dmg.FormatUDZO,
    }

    if err := dmg.Build(opts); err != nil {
        panic(err)
    }
}
```

## Icons

The library supports two distinct icon types:

| Icon Type | What It Is | Option |
|-----------|-----------|--------|
| **Volume Icon** | Shown when the DMG is mounted in Finder | `VolumeIcon` |
| **File Icon** | Shown on the `.dmg` file itself in Finder | `FileIcon` |

Both options accept any of the following formats:

| Format | Extension | Notes |
|--------|-----------|-------|
| ICNS | `.icns` | Used directly |
| PNG | `.png` | Auto-converted to ICNS |
| JPEG | `.jpg`, `.jpeg` | Auto-converted to ICNS |
| Liquid Glass | `.icon` (directory) | macOS Tahoe format - layers composited and converted to ICNS |

Icons are set using pure Go extended attribute manipulation (`com.apple.FinderInfo` and `com.apple.ResourceFork`), with no dependency on Xcode or the `SetFile` command.

## ICNS Package

The `icns` sub-package can be used independently to create Apple Icon Image (`.icns`) files from Go images:

```go
import "github.com/leaanthony/dmg/icns"

// Convert a PNG file to ICNS
err := icns.EncodeFile("output.icns", "source.png")

// Or work with Go images directly
img := loadMyImage()
data, err := icns.IcnsFromImage(img)

// Read a macOS Tahoe .icon package
composited, err := icns.ReadDotIcon("AppIcon.icon")
```

The encoder generates standard ICNS entries for all sizes from 16x16 up to 1024x1024, using a box-filter downscale for high quality results.

## API Reference

### Options

| Field | Type | Description |
|-------|------|-------------|
| `VolumeName` | `string` | Volume name displayed in Finder title bar |
| `OutputPath` | `string` | Output path for the final DMG file |
| `Files` | `map[string]string` | Files to include (destination name -> source path) |
| `AddApplicationsSymlink` | `bool` | Create `/Applications` symlink for drag-to-install |
| `Window` | `WindowConfig` | Finder window position and size |
| `Icon` | `IconConfig` | Icon appearance settings |
| `Background` | `*BackgroundConfig` | Window background (nil for default white) |
| `Format` | `Format` | Compression format (default: `FormatUDZO`) |
| `Filesystem` | `Filesystem` | `FSHFSPlus` (default) or `FSAPFS` |
| `SizeHint` | `int` | Writable DMG size in MB (0 = auto-calculated) |
| `VolumeIcon` | `string` | Path to icon file for the mounted volume |
| `FileIcon` | `string` | Path to icon file for the .dmg file in Finder |

### WindowConfig

| Field | Type | Description |
|-------|------|-------------|
| `X` | `int` | Window X position on screen |
| `Y` | `int` | Window Y position on screen |
| `Width` | `int` | Window width in pixels |
| `Height` | `int` | Window height in pixels |

### IconConfig

| Field | Type | Description |
|-------|------|-------------|
| `Size` | `int` | Icon size in pixels (default: 128) |
| `TextSize` | `float64` | Label text size in points (default: 12) |
| `GridSpace` | `float64` | Grid spacing between icons (default: 100, max: 100) |

> **Note:** macOS Finder silently rejects all icon view settings if `GridSpace` exceeds 100. Values above 100 are automatically clamped.

### BackgroundConfig

| Field | Type | Description |
|-------|------|-------------|
| `File` | `string` | Path to background image (PNG, JPEG, etc.) |
| `Color` | `*ColorRGB` | Solid color background (used when `File` is empty) |

When neither `File` nor `Color` is set, the DMG uses a default white background.

### ColorRGB

| Field | Type | Description |
|-------|------|-------------|
| `R` | `float64` | Red component (0.0 - 1.0) |
| `G` | `float64` | Green component (0.0 - 1.0) |
| `B` | `float64` | Blue component (0.0 - 1.0) |

### Background Image Helpers

The library includes helper functions for generating background images:

```go
// Solid color background
dmg.SolidBackground(width, height int, c color.Color, path string) error

// Gradient background
dmg.GradientBackground(width, height int, from, to color.Color,
    direction GradientDirection, path string) error

// Installer-style background with icon wells and arrow
dmg.InstallerBackground(width, height int, from, to color.Color,
    slots []IconSlot, iconSize int, path string) error
```

## Compression Formats

| Format | Constant | Notes |
|--------|----------|-------|
| zlib | `FormatUDZO` | Maximum compatibility (default) |
| LZFSE | `FormatULFO` | Good balance, macOS 10.11+ |
| LZMA | `FormatULMO` | Smallest output, slower |
| bzip2 | `FormatUDBZ` | Deprecated |
| Uncompressed | `FormatUDRW` | For development |

## CLI Tool

The `mkdmg` command-line tool provides a quick way to create DMGs:

```bash
go install github.com/leaanthony/dmg/cmd/mkdmg@latest

mkdmg -app /path/to/MyApp.app -output MyApp.dmg \
  -volume-icon icon.png \
  -file-icon icon.png \
  -background bg.png
```

Or use a JSON config file:

```json
{
  "volume_name": "MyApp",
  "output": "MyApp.dmg",
  "files": { "MyApp.app": "/path/to/MyApp.app" },
  "applications_symlink": true,
  "volume_icon": "icon.png",
  "file_icon": "icon.png",
  "background": { "file": "background.png" },
  "window": { "x": 200, "y": 200, "width": 540, "height": 380 },
  "icon": { "size": 128, "text_size": 12, "grid_space": 100 }
}
```

```bash
mkdmg -config dmg.json
```

## Requirements

- macOS (uses `hdiutil` for DMG creation)
- Go 1.21+

## How It Works

Instead of using AppleScript or Finder automation (which is slow and fragile), this library generates .DS_Store files directly in pure Go. The .DS_Store binary format uses a buddy allocator with a B-tree index, containing records that control Finder's window appearance:

- `bwsp` - Window position and size
- `icvp` - Icon view settings (size, text, background)
- `icvl` - Default view type
- `vSrn` - Version marker
- `Iloc` - Individual icon positions

Volume icons are set by writing `.VolumeIcon.icns` to the volume root and setting the `kHasCustomIcon` flag in `com.apple.FinderInfo`. File icons use a Classic Mac OS resource fork written to the `com.apple.ResourceFork` extended attribute. Both are handled in pure Go with no dependency on Xcode tools.

## License

MIT
