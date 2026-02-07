package dmg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/leaanthony/dmg/dsstore"
)

// Build creates a styled DMG disk image according to the provided options.
//
// The pipeline:
//  1. Create a writable DMG (UDRW format)
//  2. Attach/mount it
//  3. Copy files into the volume
//  4. Create /Applications symlink (if requested)
//  5. Copy background image to .background/ (if configured)
//  6. Generate .DS_Store with icon positions, background, window settings
//  7. Sync and detach
//  8. Shrink to minimum size
//  9. Convert to final compressed format
//  10. Clean up temporary files
func Build(opts Options) error {
	if err := validateOptions(&opts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Create temp DMG path
	tempDMG := opts.OutputPath + ".writable.dmg"
	os.Remove(tempDMG) // Remove stale temp from previous run
	defer os.Remove(tempDMG)

	// Step 1: Create writable DMG
	sizeMB := opts.SizeHint
	if sizeMB == 0 {
		sizeMB = calculateSize(opts)
	}
	if err := hdiutilCreate(opts.VolumeName, opts.Filesystem, sizeMB, tempDMG); err != nil {
		return fmt.Errorf("create DMG: %w", err)
	}

	// Step 2: Attach
	mountPoint, err := hdiutilAttach(tempDMG)
	if err != nil {
		return fmt.Errorf("attach DMG: %w", err)
	}
	// Ensure we detach even on error
	defer hdiutilDetach(mountPoint)

	// Step 3: Copy files
	if err := copyFiles(opts, mountPoint); err != nil {
		return fmt.Errorf("copy files: %w", err)
	}

	// Step 4: Applications symlink
	if opts.AddApplicationsSymlink {
		linkPath := filepath.Join(mountPoint, "Applications")
		if err := os.Symlink("/Applications", linkPath); err != nil {
			return fmt.Errorf("create Applications symlink: %w", err)
		}
	}

	// Step 5: Background image
	var bgRelativePath string
	if opts.Background != nil && opts.Background.File != "" {
		bgRelativePath, err = copyBackground(opts.Background.File, mountPoint)
		if err != nil {
			return fmt.Errorf("copy background: %w", err)
		}
	}

	// Step 6: Volume icon
	if opts.VolumeIcon != "" {
		if err := copyVolumeIcon(opts.VolumeIcon, mountPoint); err != nil {
			return fmt.Errorf("copy volume icon: %w", err)
		}
	}

	// Step 7: Generate .DS_Store
	if err := generateDSStore(opts, mountPoint, bgRelativePath); err != nil {
		return fmt.Errorf("generate .DS_Store: %w", err)
	}

	// Step 8: Sync filesystem
	syscall.Sync()

	// Step 9: Detach
	if err := hdiutilDetach(mountPoint); err != nil {
		return fmt.Errorf("detach DMG: %w", err)
	}

	// Step 10: Shrink writable DMG
	if err := hdiutilResize(tempDMG); err != nil {
		// Non-fatal: resize failure just means slightly larger output
		fmt.Fprintf(os.Stderr, "warning: resize failed: %v\n", err)
	}

	// Step 11: Convert to final format
	// Remove output if it exists (hdiutil won't overwrite)
	os.Remove(opts.OutputPath)
	if err := hdiutilConvert(tempDMG, opts.OutputPath, opts.Format); err != nil {
		return fmt.Errorf("convert DMG: %w", err)
	}

	return nil
}

// BuildWithContents creates a DMG using a simpler content-item based API.
// This is a convenience wrapper around Build for the common case.
func BuildWithContents(volumeName, outputPath string, contents []ContentItem, opts *Options) error {
	o := Options{
		VolumeName:             volumeName,
		OutputPath:             outputPath,
		Files:                  make(map[string]string),
		AddApplicationsSymlink: false,
		Format:                 FormatUDZO,
		Filesystem:             FSHFSPlus,
		Window: WindowConfig{
			X: 100, Y: 100, Width: 540, Height: 380,
		},
		Icon: IconConfig{
			Size: 128, TextSize: 12, GridSpace: 100,
		},
	}

	if opts != nil {
		if opts.Format != "" {
			o.Format = opts.Format
		}
		if opts.Filesystem != "" {
			o.Filesystem = opts.Filesystem
		}
		if opts.Window.Width > 0 {
			o.Window = opts.Window
		}
		if opts.Icon.Size > 0 {
			o.Icon = opts.Icon
		}
		if opts.Background != nil {
			o.Background = opts.Background
		}
		if opts.VolumeIcon != "" {
			o.VolumeIcon = opts.VolumeIcon
		}
		if opts.SizeHint > 0 {
			o.SizeHint = opts.SizeHint
		}
	}

	for _, item := range contents {
		name := item.Name
		if name == "" {
			name = filepath.Base(item.Path)
		}
		if item.Link != "" {
			// Links are handled separately
			continue
		}
		o.Files[name] = item.Path
	}

	return Build(o)
}

// validateOptions checks and applies defaults to the options.
func validateOptions(opts *Options) error {
	if opts.VolumeName == "" {
		return fmt.Errorf("volume name is required")
	}
	if opts.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if opts.Format == "" {
		opts.Format = FormatUDZO
	}
	if opts.Filesystem == "" {
		opts.Filesystem = FSHFSPlus
	}
	if opts.Window.Width == 0 {
		opts.Window = WindowConfig{X: 100, Y: 100, Width: 540, Height: 380}
	}
	if opts.Icon.Size == 0 {
		opts.Icon = IconConfig{Size: 128, TextSize: 12, GridSpace: 100}
	}
	return nil
}

// calculateSize estimates the DMG size needed for the given files.
func calculateSize(opts Options) int {
	var totalBytes int64
	for _, src := range opts.Files {
		totalBytes += dirSize(src)
	}
	if opts.Background != nil && opts.Background.File != "" {
		info, err := os.Stat(opts.Background.File)
		if err == nil {
			totalBytes += info.Size()
		}
	}
	// Add overhead: 32MB base + 2x file size
	sizeMB := int(totalBytes/(1024*1024))*2 + 32
	if sizeMB < 32 {
		sizeMB = 32
	}
	return sizeMB
}

// dirSize returns the total size of a file or directory in bytes.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// copyFiles copies all configured files into the mounted volume.
func copyFiles(opts Options, mountPoint string) error {
	for name, src := range opts.Files {
		dst := filepath.Join(mountPoint, name)
		if err := dittoCopy(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return nil
}

// dittoCopy uses /usr/bin/ditto to copy files, preserving code signatures and metadata.
func dittoCopy(src, dst string) error {
	cmd := exec.Command("/usr/bin/ditto", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ditto: %w\n%s", err, out)
	}
	return nil
}

// copyBackground copies the background image to the volume root as a hidden file
// (e.g., ".background.png") and returns the relative path.
// This matches dmgbuild behavior where the file is at root, not in a subdirectory.
func copyBackground(bgPath, mountPoint string) (string, error) {
	ext := filepath.Ext(bgPath)
	bgName := ".background" + ext
	dst := filepath.Join(mountPoint, bgName)
	if err := dittoCopy(bgPath, dst); err != nil {
		return "", err
	}

	return bgName, nil
}

// copyVolumeIcon copies the .icns file and sets the volume icon flag.
func copyVolumeIcon(iconPath, mountPoint string) error {
	dst := filepath.Join(mountPoint, ".VolumeIcon.icns")
	if err := dittoCopy(iconPath, dst); err != nil {
		return err
	}
	// Set custom icon attribute on the volume root
	cmd := exec.Command("SetFile", "-a", "C", mountPoint)
	cmd.Run() // best-effort, SetFile may not be available
	return nil
}

// generateDSStore creates the .DS_Store file on the mounted volume.
func generateDSStore(opts Options, mountPoint, bgRelativePath string) error {
	s := dsstore.New()

	// Window settings (bwsp record - modern format)
	top := opts.Window.Y
	left := opts.Window.X
	bottom := opts.Window.Y + opts.Window.Height
	right := opts.Window.X + opts.Window.Width
	s.SetWindowSettings(dsstore.WindowSettings{
		Top:    top,
		Left:   left,
		Bottom: bottom,
		Right:  right,
	})

	// Icon view options (icvp record - modern format)
	iconViewOpts := &dsstore.IconViewOptions{
		IconSize:  float64(opts.Icon.Size),
		TextSize:  opts.Icon.TextSize,
		GridSpace: opts.Icon.GridSpace,
		LabelOnBottom: true,
	}

	if bgRelativePath != "" {
		aliasInfo := getAliasFileInfo(mountPoint, bgRelativePath)
		s.SetBackgroundImage(opts.VolumeName, bgRelativePath, iconViewOpts, aliasInfo)
	} else if opts.Background != nil && opts.Background.Color != nil {
		iconViewOpts.BackgroundType = dsstore.BackgroundColor
		iconViewOpts.BackgroundRed = opts.Background.Color.R
		iconViewOpts.BackgroundGreen = opts.Background.Color.G
		iconViewOpts.BackgroundBlue = opts.Background.Color.B
		s.SetIconViewOptions(*iconViewOpts)
	} else {
		s.SetIconViewOptions(*iconViewOpts)
	}

	// Auto-position icons for all files
	items := sortedFileNames(opts)
	if opts.AddApplicationsSymlink {
		items = append(items, "Applications")
	}
	positionIcons(s, items, opts.Window.Width, opts.Window.Height)

	// Write .DS_Store
	dsStorePath := filepath.Join(mountPoint, ".DS_Store")
	return s.WriteFile(dsStorePath)
}

// sortedFileNames returns file names from opts.Files in sorted order.
func sortedFileNames(opts Options) []string {
	names := make([]string, 0, len(opts.Files))
	for name := range opts.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// positionIcons sets icon positions for items, distributing them evenly
// across the window width at ~45% of the window height.
func positionIcons(s *dsstore.Store, items []string, windowWidth, windowHeight int) {
	if len(items) == 0 {
		return
	}
	y := windowHeight * 45 / 100 // slightly above center for label space
	spacing := windowWidth / (len(items) + 1)
	for i, name := range items {
		x := spacing * (i + 1)
		s.SetIconPosition(name, x, y)
	}
}

// unixToHFS converts a Unix timestamp to a Mac HFS timestamp.
// HFS epoch is 1904-01-01, Unix epoch is 1970-01-01. Difference = 2082844800 seconds.
func unixToHFS(unixTime int64) uint32 {
	const hfsEpochDelta = 2082844800
	return uint32(unixTime + hfsEpochDelta)
}

// getAliasFileInfo gathers real filesystem metadata from the mounted volume
// for building an accurate alias record that Finder can resolve.
func getAliasFileInfo(mountPoint, relativePath string) *dsstore.AliasFileInfo {
	info := &dsstore.AliasFileInfo{DriveType: 0}

	// Volume creation date
	var volStat syscall.Stat_t
	if err := syscall.Stat(mountPoint, &volStat); err == nil {
		info.VolCreationDate = unixToHFS(volStat.Birthtimespec.Sec)
	}

	// File CNID and creation date
	filePath := filepath.Join(mountPoint, relativePath)
	var fileStat syscall.Stat_t
	if err := syscall.Stat(filePath, &fileStat); err == nil {
		info.FileCNID = uint32(fileStat.Ino)
		info.FileCreationDate = unixToHFS(fileStat.Birthtimespec.Sec)
	}

	// Parent directory CNID
	parentPath := filepath.Dir(filePath)
	var parentStat syscall.Stat_t
	if err := syscall.Stat(parentPath, &parentStat); err == nil {
		info.ParentCNID = uint32(parentStat.Ino)
	}

	return info
}
