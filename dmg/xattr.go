package dmg

import "golang.org/x/sys/unix"

const (
	finderInfoXattr   = "com.apple.FinderInfo"
	resourceForkXattr = "com.apple.ResourceFork"
)

// setCustomIconFlag sets the kHasCustomIcon bit in the com.apple.FinderInfo
// extended attribute on the given path. This tells Finder to look for a custom
// icon (either in .VolumeIcon.icns for volumes, or in the resource fork for files).
func setCustomIconFlag(path string) error {
	// FinderInfo is a fixed 32-byte structure.
	// Read existing value to preserve other flags.
	data := make([]byte, 32)
	sz, err := unix.Getxattr(path, finderInfoXattr, data)
	if err != nil || sz != 32 {
		data = make([]byte, 32)
	}

	// kHasCustomIcon (0x0400) lives in fdFlags at bytes 8-9 (big-endian uint16).
	// Setting byte 8 bit 0x04 sets the 0x0400 flag.
	data[8] |= 0x04

	return unix.Setxattr(path, finderInfoXattr, data, 0)
}
