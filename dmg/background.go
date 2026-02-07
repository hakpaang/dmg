package dmg

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// GradientDirection specifies the direction of a gradient.
type GradientDirection int

const (
	GradientTopToBottom GradientDirection = iota
	GradientLeftToRight
	GradientTopLeftToBottomRight
	GradientTopRightToBottomLeft
)

// GradientBackground generates a gradient background image as a PNG file.
// The gradient interpolates linearly between the two colors.
func GradientBackground(width, height int, from, to color.Color, direction GradientDirection, outputPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fr, fg, fb, _ := from.RGBA()
	tr, tg, tb, _ := to.RGBA()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var t float64
			switch direction {
			case GradientTopToBottom:
				t = float64(y) / float64(height-1)
			case GradientLeftToRight:
				t = float64(x) / float64(width-1)
			case GradientTopLeftToBottomRight:
				t = (float64(x)/float64(width-1) + float64(y)/float64(height-1)) / 2
			case GradientTopRightToBottomLeft:
				t = (float64(width-1-x)/float64(width-1) + float64(y)/float64(height-1)) / 2
			}
			r := uint8((float64(fr>>8)*(1-t) + float64(tr>>8)*t))
			g := uint8((float64(fg>>8)*(1-t) + float64(tg>>8)*t))
			b := uint8((float64(fb>>8)*(1-t) + float64(tb>>8)*t))
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return writePNG(img, outputPath)
}

// SolidBackground generates a solid color background image as a PNG file.
func SolidBackground(width, height int, c color.Color, outputPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	return writePNG(img, outputPath)
}

// IconSlot defines a position where an icon will be placed on the DMG.
// The background generator can create visual "wells" or cleared areas at these positions.
type IconSlot struct {
	X, Y   int // Center position of the icon
	Label  string
}

// InstallerBackground generates a typical macOS installer-style background:
// a gradient with an arrow pointing from app to Applications folder,
// and cleared areas around the icon positions.
func InstallerBackground(width, height int, from, to color.Color, slots []IconSlot, iconSize int, outputPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw gradient base
	fr, fg, fb, _ := from.RGBA()
	tr, tg, tb, _ := to.RGBA()
	for y := 0; y < height; y++ {
		t := float64(y) / float64(height-1)
		r := uint8(float64(fr>>8)*(1-t) + float64(tr>>8)*t)
		g := uint8(float64(fg>>8)*(1-t) + float64(tg>>8)*t)
		b := uint8(float64(fb>>8)*(1-t) + float64(tb>>8)*t)
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// Draw arrow between first two slots if we have at least 2
	if len(slots) >= 2 {
		drawArrow(img, slots[0].X, slots[0].Y, slots[1].X, slots[1].Y, iconSize, color.RGBA{R: 255, G: 255, B: 255, A: 80})
	}

	// Draw subtle circles (wells) around icon slots
	for _, slot := range slots {
		drawCircle(img, slot.X, slot.Y, iconSize/2+20, color.RGBA{R: 255, G: 255, B: 255, A: 30})
	}

	return writePNG(img, outputPath)
}

// drawArrow draws a simple arrow between two points, avoiding the icon areas.
func drawArrow(img *image.RGBA, x1, y1, x2, y2, iconSize int, c color.RGBA) {
	// Draw a thick line between the two points, but skip the icon areas
	halfIcon := iconSize/2 + 10
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist == 0 {
		return
	}

	// Normalized direction
	nx := dx / dist
	ny := dy / dist

	// Start and end points, offset to avoid icons
	startDist := float64(halfIcon)
	endDist := dist - float64(halfIcon)

	for d := startDist; d < endDist; d += 1.0 {
		px := float64(x1) + nx*d
		py := float64(y1) + ny*d
		// Draw a thick line (3px)
		for t := -1; t <= 1; t++ {
			ix := int(px + ny*float64(t))
			iy := int(py - nx*float64(t))
			if ix >= 0 && ix < img.Bounds().Max.X && iy >= 0 && iy < img.Bounds().Max.Y {
				blendPixel(img, ix, iy, c)
			}
		}
	}

	// Draw arrowhead
	arrowSize := 12.0
	tipX := float64(x1) + nx*endDist
	tipY := float64(y1) + ny*endDist
	for i := 0.0; i < arrowSize; i += 0.5 {
		spread := i * 0.6
		bx := tipX - nx*i
		by := tipY - ny*i
		for s := -spread; s <= spread; s += 0.5 {
			ix := int(bx + ny*s)
			iy := int(by - nx*s)
			if ix >= 0 && ix < img.Bounds().Max.X && iy >= 0 && iy < img.Bounds().Max.Y {
				blendPixel(img, ix, iy, c)
			}
		}
	}
}

// drawCircle draws a filled circle with alpha blending.
func drawCircle(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= float64(radius) {
				if x >= 0 && x < img.Bounds().Max.X && y >= 0 && y < img.Bounds().Max.Y {
					// Smooth edge with anti-aliasing
					edgeDist := float64(radius) - dist
					alpha := c.A
					if edgeDist < 2.0 {
						alpha = uint8(float64(alpha) * edgeDist / 2.0)
					}
					blendPixel(img, x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: alpha})
				}
			}
		}
	}
}

// blendPixel alpha-blends a color onto an existing pixel.
func blendPixel(img *image.RGBA, x, y int, c color.RGBA) {
	existing := img.RGBAAt(x, y)
	a := float64(c.A) / 255.0
	r := uint8(float64(existing.R)*(1-a) + float64(c.R)*a)
	g := uint8(float64(existing.G)*(1-a) + float64(c.G)*a)
	b := uint8(float64(existing.B)*(1-a) + float64(c.B)*a)
	img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}

// DrawText renders simple pixel text on an image. Uses a basic built-in font.
// This is intentionally simple - for high-quality text, use an external image
// with proper font rendering.
func DrawText(img *image.RGBA, text string, x, y int, c color.Color) {
	// Simple 5x7 bitmap font for basic labels
	// Only supports A-Z, a-z, 0-9, space, and common punctuation
	for i, ch := range text {
		drawChar(img, ch, x+i*6, y, c)
	}
}

// drawChar draws a single character at the given position.
func drawChar(img *image.RGBA, ch rune, x, y int, c color.Color) {
	glyph, ok := font5x7[ch]
	if !ok {
		return
	}
	for row := 0; row < 7; row++ {
		for col := 0; col < 5; col++ {
			if glyph[row]&(1<<(4-col)) != 0 {
				px := x + col
				py := y + row
				if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
					img.Set(px, py, c)
				}
			}
		}
	}
}

// AddLogo composites a logo image (PNG) onto the background at the specified position.
// The logo is placed with its center at (cx, cy).
func AddLogo(backgroundPath, logoPath string, cx, cy int, outputPath string) error {
	bgFile, err := os.Open(backgroundPath)
	if err != nil {
		return fmt.Errorf("open background: %w", err)
	}
	defer bgFile.Close()

	bgImg, _, err := image.Decode(bgFile)
	if err != nil {
		return fmt.Errorf("decode background: %w", err)
	}

	logoFile, err := os.Open(logoPath)
	if err != nil {
		return fmt.Errorf("open logo: %w", err)
	}
	defer logoFile.Close()

	logoImg, _, err := image.Decode(logoFile)
	if err != nil {
		return fmt.Errorf("decode logo: %w", err)
	}

	// Create output image from background
	bounds := bgImg.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, bgImg.At(x, y))
		}
	}

	// Composite logo centered at (cx, cy)
	logoBounds := logoImg.Bounds()
	offsetX := cx - logoBounds.Dx()/2
	offsetY := cy - logoBounds.Dy()/2

	for y := logoBounds.Min.Y; y < logoBounds.Max.Y; y++ {
		for x := logoBounds.Min.X; x < logoBounds.Max.X; x++ {
			px := offsetX + x - logoBounds.Min.X
			py := offsetY + y - logoBounds.Min.Y
			if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
				lr, lg, lb, la := logoImg.At(x, y).RGBA()
				if la == 0 {
					continue
				}
				a := float64(la) / 0xFFFF
				br, bg, bb, _ := out.At(px, py).RGBA()
				r := uint8((float64(lr>>8)*a + float64(br>>8)*(1-a)))
				g := uint8((float64(lg>>8)*a + float64(bg>>8)*(1-a)))
				b := uint8((float64(lb>>8)*a + float64(bb>>8)*(1-a)))
				out.SetRGBA(px, py, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}

	return writePNG(out, outputPath)
}

// writePNG writes an image to a PNG file.
func writePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}
	return nil
}

// font5x7 is a minimal 5x7 bitmap font for basic text rendering.
// Each character is 7 rows of 5-bit wide bitmaps (MSB = leftmost pixel).
var font5x7 = map[rune][7]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	'A': {0x04, 0x0A, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0E},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I': {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'J': {0x07, 0x02, 0x02, 0x02, 0x02, 0x12, 0x0C},
	'K': {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q': {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0E, 0x11, 0x10, 0x0E, 0x01, 0x11, 0x0E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V': {0x11, 0x11, 0x11, 0x11, 0x0A, 0x0A, 0x04},
	'W': {0x11, 0x11, 0x11, 0x15, 0x15, 0x1B, 0x11},
	'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'Z': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'a': {0x00, 0x00, 0x0E, 0x01, 0x0F, 0x11, 0x0F},
	'b': {0x10, 0x10, 0x1E, 0x11, 0x11, 0x11, 0x1E},
	'c': {0x00, 0x00, 0x0E, 0x11, 0x10, 0x11, 0x0E},
	'd': {0x01, 0x01, 0x0F, 0x11, 0x11, 0x11, 0x0F},
	'e': {0x00, 0x00, 0x0E, 0x11, 0x1F, 0x10, 0x0E},
	'f': {0x06, 0x09, 0x08, 0x1C, 0x08, 0x08, 0x08},
	'g': {0x00, 0x00, 0x0F, 0x11, 0x0F, 0x01, 0x0E},
	'h': {0x10, 0x10, 0x1E, 0x11, 0x11, 0x11, 0x11},
	'i': {0x04, 0x00, 0x0C, 0x04, 0x04, 0x04, 0x0E},
	'j': {0x02, 0x00, 0x06, 0x02, 0x02, 0x12, 0x0C},
	'k': {0x10, 0x10, 0x12, 0x14, 0x18, 0x14, 0x12},
	'l': {0x0C, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'm': {0x00, 0x00, 0x1A, 0x15, 0x15, 0x11, 0x11},
	'n': {0x00, 0x00, 0x1E, 0x11, 0x11, 0x11, 0x11},
	'o': {0x00, 0x00, 0x0E, 0x11, 0x11, 0x11, 0x0E},
	'p': {0x00, 0x00, 0x1E, 0x11, 0x1E, 0x10, 0x10},
	'q': {0x00, 0x00, 0x0F, 0x11, 0x0F, 0x01, 0x01},
	'r': {0x00, 0x00, 0x16, 0x19, 0x10, 0x10, 0x10},
	's': {0x00, 0x00, 0x0F, 0x10, 0x0E, 0x01, 0x1E},
	't': {0x08, 0x08, 0x1C, 0x08, 0x08, 0x09, 0x06},
	'u': {0x00, 0x00, 0x11, 0x11, 0x11, 0x11, 0x0F},
	'v': {0x00, 0x00, 0x11, 0x11, 0x0A, 0x0A, 0x04},
	'w': {0x00, 0x00, 0x11, 0x11, 0x15, 0x15, 0x0A},
	'x': {0x00, 0x00, 0x11, 0x0A, 0x04, 0x0A, 0x11},
	'y': {0x00, 0x00, 0x11, 0x11, 0x0F, 0x01, 0x0E},
	'z': {0x00, 0x00, 0x1F, 0x02, 0x04, 0x08, 0x1F},
	'0': {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
	'1': {0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'2': {0x0E, 0x11, 0x01, 0x06, 0x08, 0x10, 0x1F},
	'3': {0x0E, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0E},
	'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5': {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
	'6': {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C},
	'.': {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04},
	',': {0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x08},
	'-': {0x00, 0x00, 0x00, 0x1F, 0x00, 0x00, 0x00},
	'+': {0x00, 0x04, 0x04, 0x1F, 0x04, 0x04, 0x00},
	'!': {0x04, 0x04, 0x04, 0x04, 0x04, 0x00, 0x04},
	'?': {0x0E, 0x11, 0x01, 0x06, 0x04, 0x00, 0x04},
	'/': {0x01, 0x02, 0x02, 0x04, 0x08, 0x08, 0x10},
	':': {0x00, 0x00, 0x04, 0x00, 0x00, 0x04, 0x00},
	'(': {0x02, 0x04, 0x08, 0x08, 0x08, 0x04, 0x02},
	')': {0x08, 0x04, 0x02, 0x02, 0x02, 0x04, 0x08},
}
