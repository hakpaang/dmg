package udif_test

import (
	"bytes"
	"fmt"

	"github.com/leaanthony/dmg/udif"
)

// Wrap a raw, sector-aligned filesystem image (such as one produced by the
// hfsplus package) into a UDZO-compressed .dmg container.
func ExampleBuild() {
	// In practice this is a real HFS+ volume image; here it is a stand-in that
	// is a multiple of the 512-byte sector size.
	raw := make([]byte, 4*1024*1024)
	copy(raw, []byte("filesystem image goes here"))

	var dmg bytes.Buffer
	if err := udif.Build(&dmg, raw, "whole disk (Apple_HFS : 0)"); err != nil {
		panic(err)
	}

	// The output is a complete .dmg; mostly-zero input compresses heavily.
	fmt.Printf("%dKiB raw -> %d byte .dmg\n", len(raw)/1024, dmg.Len())
}
