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
- Volume icons (.icns)
- Pure Go .DS_Store generation (no AppleScript/Finder scripting)

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
        Format: dmg.FormatUDZO,
    }

    if err := dmg.Build(opts); err != nil {
        panic(err)
    }
}
```

## Compression Formats

| Format | Constant | Notes |
|--------|----------|-------|
| zlib | `FormatUDZO` | Maximum compatibility (default) |
| LZFSE | `FormatULFO` | Good balance, macOS 10.11+ |
| LZMA | `FormatULMO` | Smallest output, slower |
| bzip2 | `FormatUDBZ` | Deprecated |
| Uncompressed | `FormatUDRW` | For development |

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

## License

MIT
