package dmg

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/leaanthony/dmg/icns"

	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
)

// readIconData reads icon data from the given path, returning ICNS-format bytes.
// Supported inputs:
//   - .icns files (read directly)
//   - .png / .jpg / .jpeg files (converted to ICNS)
//   - .icon directories (macOS Tahoe Liquid Glass packages - layers are
//     composited and converted to ICNS)
func readIconData(path string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))

	// Check if it's a .icon directory package (macOS Tahoe Liquid Glass)
	if ext == ".icon" {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat icon path: %w", err)
		}
		if info.IsDir() {
			img, err := icns.ReadDotIcon(path)
			if err != nil {
				return nil, fmt.Errorf("read .icon package: %w", err)
			}
			return icns.IcnsFromImage(img)
		}
	}

	if ext == ".icns" {
		return os.ReadFile(path)
	}

	// Decode as image and convert to ICNS
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open icon file: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode icon image: %w", err)
	}

	var buf bytes.Buffer
	if err := icns.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("convert to ICNS: %w", err)
	}
	return buf.Bytes(), nil
}

// setFileIcon sets a custom Finder icon on a file by writing the ICNS data
// into the file's resource fork and setting the kHasCustomIcon flag.
func setFileIcon(filePath string, icnsData []byte) error {
	resFork := buildResourceFork(icnsData)
	if err := unix.Setxattr(filePath, resourceForkXattr, resFork, 0); err != nil {
		return fmt.Errorf("write resource fork: %w", err)
	}
	return setCustomIconFlag(filePath)
}
