//go:build darwin

package dmg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/leaanthony/dmg/dsstore"
)

// buildHdiutil creates the DMG using the macOS hdiutil pipeline:
//
//  1. Create a writable DMG (UDRW)
//  2. Attach/mount it
//  3. Copy files into the volume (ditto, preserving signatures)
//  4. Create /Applications symlink (if requested)
//  5. Copy background image to the volume root (if configured)
//  6. Copy volume icon (if configured)
//  7. Generate .DS_Store
//  8. Sync and detach
//  9. Shrink to minimum size
//  10. Convert to the final compressed format
//  11. Set the file icon on the final DMG (if configured)
func buildHdiutil(opts Options) error {
	tempDMG := opts.OutputPath + ".writable.dmg"
	os.Remove(tempDMG)
	defer os.Remove(tempDMG)

	sizeMB := opts.SizeHint
	if sizeMB == 0 {
		sizeMB = calculateSize(opts)
	}
	if err := hdiutilCreate(opts.VolumeName, opts.Filesystem, sizeMB, tempDMG); err != nil {
		return fmt.Errorf("create DMG: %w", err)
	}

	mountPoint, err := hdiutilAttach(tempDMG)
	if err != nil {
		return fmt.Errorf("attach DMG: %w", err)
	}
	defer hdiutilDetach(mountPoint)

	if err := copyFiles(opts, mountPoint); err != nil {
		return fmt.Errorf("copy files: %w", err)
	}

	if opts.AddApplicationsSymlink {
		linkPath := filepath.Join(mountPoint, "Applications")
		if err := os.Symlink("/Applications", linkPath); err != nil {
			return fmt.Errorf("create Applications symlink: %w", err)
		}
	}

	var bgRelativePath string
	if opts.Background != nil && opts.Background.File != "" {
		bgRelativePath, err = copyBackground(opts.Background.File, mountPoint)
		if err != nil {
			return fmt.Errorf("copy background: %w", err)
		}
	}

	if opts.VolumeIcon != "" {
		if err := copyVolumeIcon(opts.VolumeIcon, mountPoint); err != nil {
			return fmt.Errorf("copy volume icon: %w", err)
		}
	}

	if err := generateDSStore(opts, mountPoint, bgRelativePath); err != nil {
		return fmt.Errorf("generate .DS_Store: %w", err)
	}

	syncFilesystem()

	if err := hdiutilDetach(mountPoint); err != nil {
		return fmt.Errorf("detach DMG: %w", err)
	}

	if err := hdiutilResize(tempDMG); err != nil {
		fmt.Fprintf(os.Stderr, "warning: resize failed: %v\n", err)
	}

	os.Remove(opts.OutputPath)
	if err := hdiutilConvert(tempDMG, opts.OutputPath, opts.Format); err != nil {
		return fmt.Errorf("convert DMG: %w", err)
	}

	if opts.FileIcon != "" {
		icnsData, err := readIconData(opts.FileIcon)
		if err != nil {
			return fmt.Errorf("read file icon: %w", err)
		}
		if err := setFileIcon(opts.OutputPath, icnsData); err != nil {
			return fmt.Errorf("set file icon: %w", err)
		}
	}

	return nil
}

func syncFilesystem() { syscall.Sync() }

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
func copyBackground(bgPath, mountPoint string) (string, error) {
	bgName := ".background" + filepath.Ext(bgPath)
	if err := dittoCopy(bgPath, filepath.Join(mountPoint, bgName)); err != nil {
		return "", err
	}
	return bgName, nil
}

// copyVolumeIcon copies the icon as .VolumeIcon.icns and sets the custom icon flag.
func copyVolumeIcon(iconPath, mountPoint string) error {
	icnsData, err := readIconData(iconPath)
	if err != nil {
		return fmt.Errorf("read volume icon: %w", err)
	}
	dst := filepath.Join(mountPoint, ".VolumeIcon.icns")
	if err := os.WriteFile(dst, icnsData, 0644); err != nil {
		return fmt.Errorf("write .VolumeIcon.icns: %w", err)
	}
	return setCustomIconFlag(mountPoint)
}

// generateDSStore writes the .DS_Store file onto the mounted volume.
func generateDSStore(opts Options, mountPoint, bgRelativePath string) error {
	var alias *dsstore.AliasFileInfo
	if bgRelativePath != "" {
		alias = getAliasFileInfo(mountPoint, bgRelativePath)
	}
	s := newStyledStore(opts, bgRelativePath, alias)
	return s.WriteFile(filepath.Join(mountPoint, ".DS_Store"))
}

// getAliasFileInfo gathers real filesystem metadata from the mounted volume
// for building an accurate alias record that Finder can resolve.
func getAliasFileInfo(mountPoint, relativePath string) *dsstore.AliasFileInfo {
	info := &dsstore.AliasFileInfo{DriveType: 0}

	var volStat syscall.Stat_t
	if err := syscall.Stat(mountPoint, &volStat); err == nil {
		info.VolCreationDate = unixToHFS(volStat.Birthtimespec.Sec)
	}

	filePath := filepath.Join(mountPoint, relativePath)
	var fileStat syscall.Stat_t
	if err := syscall.Stat(filePath, &fileStat); err == nil {
		info.FileCNID = uint32(fileStat.Ino)
		info.FileCreationDate = unixToHFS(fileStat.Birthtimespec.Sec)
	}

	parentPath := filepath.Dir(filePath)
	var parentStat syscall.Stat_t
	if err := syscall.Stat(parentPath, &parentStat); err == nil {
		info.ParentCNID = uint32(parentStat.Ino)
	}

	return info
}
