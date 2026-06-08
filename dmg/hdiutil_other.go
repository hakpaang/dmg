//go:build !darwin

package dmg

import "fmt"

// buildHdiutil is unavailable off macOS; BackendAuto never selects it there, so
// this only triggers when the hdiutil backend is requested explicitly.
func buildHdiutil(opts Options) error {
	return fmt.Errorf("dmg: the hdiutil backend requires macOS; use BackendNative or BackendAuto")
}
