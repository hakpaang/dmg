// Command purego-dmg builds a mountable macOS .dmg from a directory using only
// Go — no hdiutil, no ditto, no macOS. Because the HFS+ volume and the UDIF
// container are both written in pure Go, this builds (and runs) on any platform,
// so you can produce a macOS disk image from a Linux or Windows CI runner.
//
// The resulting image contains your files plus an /Applications drag-link, which
// is the standard "drag the app to Applications to install" layout. (Window
// background and icon positioning are styled via the higher-level dmg package on
// macOS; this example focuses on the cross-platform image construction.)
//
// Usage:
//
//	purego-dmg <source-dir> <output.dmg> [VolumeName]
//
// Example:
//
//	purego-dmg ./MyApp.app MyApp.dmg MyApp
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/leaanthony/dmg/hfsplus"
	"github.com/leaanthony/dmg/udif"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: purego-dmg <source-dir> <output.dmg> [VolumeName]")
		os.Exit(2)
	}
	src, out := os.Args[1], os.Args[2]
	volName := "Install"
	if len(os.Args) > 3 {
		volName = os.Args[3]
	}

	// Walk the source directory into a volume tree, then add the Applications
	// symlink so users can drag the app across to install it.
	children, err := walk(src)
	if err != nil {
		fatal(err)
	}
	children = append(children, &hfsplus.Entry{
		Name:   "Applications",
		Kind:   hfsplus.KindSymlink,
		Target: "/Applications",
	})

	vol := &hfsplus.Volume{
		Name:      volName,
		CreatedAt: time.Now(),
		Children:  children,
	}

	// hfsplus.Build -> raw HFS+ image; udif.Build -> compressed .dmg container.
	img, err := hfsplus.Build(vol)
	if err != nil {
		fatal(err)
	}

	f, err := os.Create(out)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	if err := udif.Build(f, img, "whole disk (Apple_HFS : 0)"); err != nil {
		fatal(err)
	}

	fmt.Printf("wrote %s (%d entries, volume %q)\n", out, len(children), volName)
}

// walk reads a directory tree into hfsplus.Entry children, preserving file
// permissions and symlinks.
func walk(dir string) ([]*hfsplus.Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*hfsplus.Entry
	for _, de := range des {
		p := filepath.Join(dir, de.Name())
		fi, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return nil, err
			}
			out = append(out, &hfsplus.Entry{Name: de.Name(), Kind: hfsplus.KindSymlink, Target: target})
		case fi.IsDir():
			kids, err := walk(p)
			if err != nil {
				return nil, err
			}
			out = append(out, &hfsplus.Entry{Name: de.Name(), Kind: hfsplus.KindDir, Children: kids})
		default:
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, err
			}
			out = append(out, &hfsplus.Entry{
				Name: de.Name(), Kind: hfsplus.KindFile,
				Data: data, Mode: uint16(fi.Mode().Perm()),
			})
		}
	}
	return out, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
