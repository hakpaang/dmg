package hfsplus_test

import (
	"fmt"
	"os"
	"time"

	"github.com/leaanthony/dmg/hfsplus"
	"github.com/leaanthony/dmg/udif"
)

// Build a case-insensitive HFS+ volume from a tree of files, folders, and a
// symlink, then wrap it as a .dmg — entirely in Go, no macOS required.
func ExampleBuild() {
	vol := &hfsplus.Volume{
		Name:      "Install",
		CreatedAt: time.Unix(1700000000, 0),
		Children: []*hfsplus.Entry{
			{Name: "README.txt", Kind: hfsplus.KindFile, Data: []byte("hello")},
			{Name: "MyApp.app", Kind: hfsplus.KindDir, Children: []*hfsplus.Entry{
				{Name: "run", Kind: hfsplus.KindFile, Data: []byte("#!/bin/sh\n"), Mode: 0755},
			}},
			// The standard drag-to-install link.
			{Name: "Applications", Kind: hfsplus.KindSymlink, Target: "/Applications"},
		},
	}

	img, err := hfsplus.Build(vol)
	if err != nil {
		panic(err)
	}

	f, err := os.Create("Install.dmg")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := udif.Build(f, img, "whole disk (Apple_HFS : 0)"); err != nil {
		panic(err)
	}

	fmt.Printf("raw volume is %d bytes\n", len(img))
}

// Set CaseSensitive for an HFSX volume (required for non-ASCII file names).
func ExampleVolume_caseSensitive() {
	vol := &hfsplus.Volume{
		Name:          "Café",
		CreatedAt:     time.Unix(1700000000, 0),
		CaseSensitive: true,
		Children: []*hfsplus.Entry{
			{Name: "résumé.txt", Kind: hfsplus.KindFile, Data: []byte("…")},
		},
	}
	if _, err := hfsplus.Build(vol); err != nil {
		panic(err)
	}
}
