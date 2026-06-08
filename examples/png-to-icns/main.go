// Command png-to-icns converts a PNG or JPEG image into an Apple .icns icon
// file using the pure-Go icns package (no iconutil / Xcode required).
//
// Usage:
//
//	png-to-icns <image.png> <output.icns>
package main

import (
	"fmt"
	"os"

	"github.com/leaanthony/dmg/icns"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: png-to-icns <image.png> <output.icns>")
		os.Exit(2)
	}
	src, out := os.Args[1], os.Args[2]

	if err := icns.EncodeFile(out, src); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", out)
}
