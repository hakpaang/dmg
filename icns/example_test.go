package icns_test

import (
	"bytes"
	"image"
	"image/color"

	"github.com/leaanthony/dmg/icns"
)

// Convert a PNG or JPEG file directly to an .icns icon file.
func ExampleEncodeFile() {
	if err := icns.EncodeFile("AppIcon.icns", "icon.png"); err != nil {
		panic(err)
	}
}

// Encode an in-memory image.Image to ICNS bytes.
func ExampleEncode() {
	img := image.NewRGBA(image.Rect(0, 0, 512, 512))
	for x := 0; x < 512; x++ {
		for y := 0; y < 512; y++ {
			img.Set(x, y, color.RGBA{uint8(x / 2), uint8(y / 2), 200, 255})
		}
	}

	var buf bytes.Buffer
	if err := icns.Encode(&buf, img); err != nil {
		panic(err)
	}
	// buf now holds a complete .icns file.
}
