// Package dmg creates styled macOS disk images (DMGs) with custom Finder
// window appearance including background images, icon positions, and window geometry.
//
// It uses the dsstore package to generate .DS_Store files and hdiutil for
// disk image creation and conversion.
package dmg

// Format specifies the DMG compression format.
type Format string

const (
	FormatUDRW Format = "UDRW" // Read-write (uncompressed, for development)
	FormatUDZO Format = "UDZO" // zlib compressed (maximum compatibility)
	FormatULFO Format = "ULFO" // LZFSE compressed (macOS 10.11+, good balance)
	FormatULMO Format = "ULMO" // LZMA compressed (smallest, slower decompression)
	FormatUDBZ Format = "UDBZ" // bzip2 compressed (deprecated)
)

// Filesystem specifies the DMG filesystem type.
type Filesystem string

const (
	FSHFSPlus Filesystem = "HFS+"
	FSAPFS    Filesystem = "APFS"
)

// Options configures the DMG creation process.
type Options struct {
	// Volume name displayed in Finder title bar
	VolumeName string

	// Output path for the final DMG file
	OutputPath string

	// Files to include in the DMG. Keys are destination names inside the volume,
	// values are source paths on disk.
	Files map[string]string

	// AddApplicationsSymlink creates a symlink to /Applications in the DMG.
	AddApplicationsSymlink bool

	// Window configures the Finder window geometry.
	Window WindowConfig

	// Icon configures icon appearance.
	Icon IconConfig

	// Background configures the Finder window background.
	Background *BackgroundConfig

	// Format for the final DMG (default: FormatUDZO)
	Format Format

	// Filesystem type (default: FSHFSPlus)
	Filesystem Filesystem

	// SizeHint is the writable DMG size in megabytes.
	// If 0, it's calculated automatically from the file sizes + overhead.
	SizeHint int

	// VolumeIcon is the path to a .icns file for the volume icon (optional).
	VolumeIcon string
}

// WindowConfig defines the Finder window position and size.
type WindowConfig struct {
	X      int // Window X position on screen
	Y      int // Window Y position on screen
	Width  int // Window width in pixels
	Height int // Window height in pixels
}

// IconConfig defines icon appearance settings.
type IconConfig struct {
	Size      int     // Icon size in pixels (default: 128)
	TextSize  float64 // Label text size in points (default: 12)
	GridSpace float64 // Grid spacing between icons (default: 100)
}

// BackgroundConfig defines the Finder window background.
type BackgroundConfig struct {
	// Path to the background image file on disk.
	// The image will be copied into the DMG's .background/ folder.
	File string

	// Color sets a solid color background instead of an image.
	// Only used if File is empty. Values are 0.0-1.0.
	Color *ColorRGB
}

// ColorRGB represents an RGB color with float components (0.0-1.0).
type ColorRGB struct {
	R, G, B float64
}

// ContentItem defines a file or link to include in the DMG with a positioned icon.
type ContentItem struct {
	// Source path on disk (for files/directories)
	Path string

	// Name inside the DMG volume (defaults to base name of Path)
	Name string

	// Icon position in the Finder window
	X, Y int

	// Link makes this a symlink to the given target instead of copying a file.
	// Set to "/Applications" for the standard app-drop-link.
	Link string
}

// DefaultOptions returns Options with sensible defaults for a typical app DMG.
func DefaultOptions(appPath, outputPath string) Options {
	return Options{
		VolumeName: "Application",
		OutputPath: outputPath,
		Files:      map[string]string{},
		Window: WindowConfig{
			X:      100,
			Y:      100,
			Width:  540,
			Height: 380,
		},
		Icon: IconConfig{
			Size:      128,
			TextSize:  12,
			GridSpace: 100,
		},
		Format:                 FormatUDZO,
		Filesystem:             FSHFSPlus,
		AddApplicationsSymlink: true,
	}
}
