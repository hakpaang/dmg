// Package icns creates Apple Icon Image (.icns) files from standard Go images.
//
// Modern ICNS files store PNG-encoded images at various sizes. This package
// generates entries for all standard icon sizes up to the source image dimensions,
// using a box-filter downscale for high quality results.
package icns

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"

	_ "image/jpeg" // register JPEG decoder
)

// Standard ICNS icon types with their pixel sizes.
// Listed largest-first so we generate the biggest entry first.
var standardSizes = []struct {
	osType [4]byte
	size   int
}{
	{[4]byte{'i', 'c', '1', '0'}, 1024},
	{[4]byte{'i', 'c', '0', '9'}, 512},
	{[4]byte{'i', 'c', '0', '8'}, 256},
	{[4]byte{'i', 'c', '0', '7'}, 128},
	{[4]byte{'i', 'c', 'p', '6'}, 64},
	{[4]byte{'i', 'c', 'p', '5'}, 32},
	{[4]byte{'i', 'c', 'p', '4'}, 16},
}

// Encode writes ICNS format data to w containing the provided image at all
// standard icon sizes up to the source dimensions. The source image should
// be square; non-square images are center-cropped to a square first.
func Encode(w io.Writer, img image.Image) error {
	img = cropToSquare(img)
	srcSize := img.Bounds().Dx()

	if srcSize < 16 {
		return fmt.Errorf("icns: source image too small (%dx%d), minimum 16x16", srcSize, srcSize)
	}

	// Generate PNG data for each standard size <= source size
	type entry struct {
		osType  [4]byte
		pngData []byte
	}
	var entries []entry

	for _, st := range standardSizes {
		if st.size > srcSize {
			continue
		}
		resized := resizeImage(img, st.size)
		var buf bytes.Buffer
		if err := png.Encode(&buf, resized); err != nil {
			return fmt.Errorf("icns: encode %dx%d PNG: %w", st.size, st.size, err)
		}
		entries = append(entries, entry{osType: st.osType, pngData: buf.Bytes()})
	}

	if len(entries) == 0 {
		return fmt.Errorf("icns: no standard sizes fit source image (%dx%d)", srcSize, srcSize)
	}

	// Calculate total ICNS file size: 8-byte header + sum of entries
	totalSize := 8
	for _, e := range entries {
		totalSize += 8 + len(e.pngData) // 4 type + 4 length + data
	}

	// Write ICNS header
	var header [8]byte
	copy(header[0:4], "icns")
	binary.BigEndian.PutUint32(header[4:8], uint32(totalSize))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	// Write each entry
	for _, e := range entries {
		var entryHeader [8]byte
		copy(entryHeader[0:4], e.osType[:])
		binary.BigEndian.PutUint32(entryHeader[4:8], uint32(8+len(e.pngData)))
		if _, err := w.Write(entryHeader[:]); err != nil {
			return err
		}
		if _, err := w.Write(e.pngData); err != nil {
			return err
		}
	}

	return nil
}

// EncodeFile reads an image file (PNG or JPEG) and writes an ICNS file.
func EncodeFile(icnsPath, imagePath string) error {
	f, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("icns: open source: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("icns: decode source: %w", err)
	}

	out, err := os.Create(icnsPath)
	if err != nil {
		return fmt.Errorf("icns: create output: %w", err)
	}
	defer out.Close()

	return Encode(out, img)
}

// IcnsFromImage returns ICNS data as bytes for the given image.
func IcnsFromImage(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// cropToSquare center-crops a non-square image to a square.
func cropToSquare(src image.Image) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == h {
		return src
	}
	size := min(w, h)
	x0 := bounds.Min.X + (w-size)/2
	y0 := bounds.Min.Y + (h-size)/2
	cropped := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(cropped, cropped.Bounds(), src, image.Pt(x0, y0), draw.Src)
	return cropped
}

// resizeImage scales src to the given square size using a box filter (area average).
// This produces high quality results for downscaling.
func resizeImage(src image.Image, size int) image.Image {
	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()

	if srcW == size && srcH == size {
		// Already the right size, just ensure it's RGBA
		dst := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Src)
		return dst
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			// Map destination pixel to source region
			sx0 := dx * srcW / size
			sy0 := dy * srcH / size
			sx1 := (dx + 1) * srcW / size
			sy1 := (dy + 1) * srcH / size
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			if sy1 <= sy0 {
				sy1 = sy0 + 1
			}

			// Average all source pixels in the mapped region (premultiplied alpha)
			var rAcc, gAcc, bAcc, aAcc uint64
			var n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					r, g, b, a := src.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
					rAcc += uint64(r)
					gAcc += uint64(g)
					bAcc += uint64(b)
					aAcc += uint64(a)
					n++
				}
			}
			dst.SetRGBA(dx, dy, color.RGBA{
				R: uint8(rAcc / n >> 8),
				G: uint8(gAcc / n >> 8),
				B: uint8(bAcc / n >> 8),
				A: uint8(aAcc / n >> 8),
			})
		}
	}
	return dst
}
