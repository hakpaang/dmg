//go:build windows

package dmg

import "fmt"

// Custom Finder icons rely on macOS extended attributes, which do not exist on
// Windows. These stubs satisfy compilation; the native backend only calls
// setFileIcon under runtime.GOOS == "darwin", so they are never reached at
// runtime in a normal cross-compiled build.

func setCustomIconFlag(path string) error {
	return fmt.Errorf("dmg: custom Finder icons are only supported on macOS")
}

func setResourceFork(path string, data []byte) error {
	return fmt.Errorf("dmg: resource forks are only supported on macOS")
}
