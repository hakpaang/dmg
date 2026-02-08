package icns

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "image/jpeg"
	_ "image/png"
)

// ReadDotIcon reads a macOS Tahoe .icon package (Liquid Glass format) and
// returns the composited layers as a single image suitable for ICNS conversion.
//
// The .icon format is a directory package containing layer images in an Assets/
// subfolder and an icon.json manifest. This function finds the layer PNGs,
// determines their ordering (back → middle → front), and composites them
// using alpha blending.
func ReadDotIcon(path string) (image.Image, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("doticon: stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("doticon: %s is not a directory", path)
	}

	// Try to find layer images from icon.json first, fall back to scanning
	layers, err := findLayers(path)
	if err != nil || len(layers) == 0 {
		return nil, fmt.Errorf("doticon: no layer images found in %s", path)
	}

	// Load and composite layers (ordered back to front)
	var result *image.RGBA
	for _, layerPath := range layers {
		img, err := loadImage(layerPath)
		if err != nil {
			continue // skip unreadable layers
		}

		if result == nil {
			// First layer becomes the canvas
			bounds := img.Bounds()
			result = image.NewRGBA(bounds)
			draw.Draw(result, bounds, img, bounds.Min, draw.Src)
		} else {
			// Composite subsequent layers on top
			draw.Draw(result, result.Bounds(), img, img.Bounds().Min, draw.Over)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("doticon: could not load any layer images from %s", path)
	}

	return result, nil
}

// layerOrder assigns a sort priority to a layer based on its path/name.
// Lower values are drawn first (bottom). Higher values are drawn on top.
func layerOrder(path string) int {
	lower := strings.ToLower(filepath.Base(filepath.Dir(path)))
	name := strings.ToLower(filepath.Base(path))

	// Check directory name first, then filename
	for _, s := range []string{lower, name} {
		if strings.Contains(s, "back") {
			return 0
		}
		if strings.Contains(s, "mid") {
			return 1
		}
		if strings.Contains(s, "front") || strings.Contains(s, "fore") {
			return 2
		}
	}
	return 1 // default to middle
}

// findLayers locates layer image files inside a .icon package.
// It first tries to extract file references from icon.json,
// then falls back to scanning the Assets/ directory for PNGs.
func findLayers(iconDir string) ([]string, error) {
	// Try icon.json first
	layers, err := findLayersFromJSON(iconDir)
	if err == nil && len(layers) > 0 {
		return layers, nil
	}

	// Fall back to scanning for image files
	return scanForImages(iconDir)
}

// findLayersFromJSON attempts to parse icon.json and extract image file paths.
// Since the format is not publicly documented, this uses a flexible approach:
// it recursively searches the JSON for string values that look like image filenames.
func findLayersFromJSON(iconDir string) ([]string, error) {
	jsonPath := filepath.Join(iconDir, "icon.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Extract all string values that look like image references
	var refs []string
	extractImageRefs(raw, &refs)

	// Resolve relative paths against the icon directory
	var resolved []string
	for _, ref := range refs {
		// Try the reference as-is, then under Assets/
		candidates := []string{
			filepath.Join(iconDir, ref),
			filepath.Join(iconDir, "Assets", ref),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				resolved = append(resolved, c)
				break
			}
		}
	}

	// Sort by layer order
	sort.Slice(resolved, func(i, j int) bool {
		return layerOrder(resolved[i]) < layerOrder(resolved[j])
	})

	return resolved, nil
}

// extractImageRefs recursively searches a JSON structure for string values
// that look like image file references.
func extractImageRefs(v interface{}, refs *[]string) {
	switch val := v.(type) {
	case string:
		lower := strings.ToLower(val)
		if strings.HasSuffix(lower, ".png") ||
			strings.HasSuffix(lower, ".jpg") ||
			strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".svg") ||
			strings.HasSuffix(lower, ".pdf") {
			*refs = append(*refs, val)
		}
	case map[string]interface{}:
		for _, child := range val {
			extractImageRefs(child, refs)
		}
	case []interface{}:
		for _, child := range val {
			extractImageRefs(child, refs)
		}
	}
}

// scanForImages finds all PNG/JPEG files inside the .icon package directory,
// sorted by layer order (back → middle → front).
func scanForImages(iconDir string) ([]string, error) {
	var images []string

	// Walk the entire .icon directory looking for image files
	err := filepath.Walk(iconDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			images = append(images, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by layer order (back first, front last)
	sort.Slice(images, func(i, j int) bool {
		oi, oj := layerOrder(images[i]), layerOrder(images[j])
		if oi != oj {
			return oi < oj
		}
		return images[i] < images[j] // stable alphabetical within same priority
	})

	return images, nil
}

// loadImage reads and decodes an image file (PNG or JPEG).
func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return img, nil
}
