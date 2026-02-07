package dsstore

import (
	"io"
	"os"
	"sort"

	"howett.net/plist"
)

// BackgroundType specifies the type of background for icon view.
type BackgroundType int

const (
	BackgroundDefault BackgroundType = iota // No custom background
	BackgroundColor                         // Solid color
	BackgroundPicture                       // Image file
)

// IconViewOptions configures the icon view settings for the icvp record.
type IconViewOptions struct {
	IconSize  float64 // Icon size in pixels (default: 128)
	TextSize  float64 // Label text size in points (default: 12)
	GridSpace float64 // Grid spacing between icons (default: 100, max: 100)
	GridOffX  float64 // Horizontal grid offset
	GridOffY  float64 // Vertical grid offset

	LabelOnBottom   bool // Label below icon (true) or to the right (false)
	ShowIconPreview bool // Show file preview as icon
	ShowItemInfo    bool // Show item info below icon

	BackgroundType  BackgroundType
	BackgroundRed   float64 // 0.0-1.0 (for BackgroundColor)
	BackgroundGreen float64 // 0.0-1.0 (for BackgroundColor)
	BackgroundBlue  float64 // 0.0-1.0 (for BackgroundColor)
	BackgroundAlias []byte  // Raw alias/bookmark bytes (for BackgroundPicture)
}

// WindowSettings configures the Finder window geometry.
type WindowSettings struct {
	Top    int
	Left   int
	Bottom int
	Right  int
}

// Store builds a .DS_Store file from a set of records.
type Store struct {
	records []Record
}

// New creates a new empty Store.
func New() *Store {
	return &Store{}
}

// AddRecord adds a raw record to the store.
func (s *Store) AddRecord(r Record) {
	s.records = append(s.records, r)
}

// SetIconPosition sets the icon position for a file/folder in the Finder window.
func (s *Store) SetIconPosition(filename string, x, y int) {
	s.records = append(s.records, NewIlocRecord(filename, uint32(x), uint32(y)))
}

// SetViewStyle sets the Finder view style (icon, list, column, coverflow).
func (s *Store) SetViewStyle(style FourCharCode) {
	s.records = append(s.records, NewVstlRecord(style))
}

// SetWindowPosition sets the Finder window position and size.
func (s *Store) SetWindowPosition(top, left, bottom, right int) {
	s.records = append(s.records, NewFwi0Record(
		uint16(top), uint16(left), uint16(bottom), uint16(right), ViewIcon,
	))
}

// SetWindowSettings sets the window bounds via the modern bwsp plist record.
func (s *Store) SetWindowSettings(ws WindowSettings) {
	m := map[string]interface{}{
		"WindowBounds":          formatRect(ws.Top, ws.Left, ws.Bottom, ws.Right),
		"ShowSidebar":           false,
		"ShowToolbar":           false,
		"ShowStatusBar":         false,
		"ShowTabView":           false,
		"ContainerShowSidebar":  false,
		"PreviewPaneVisibility": false,
		"ShowPathbar":           false,
		"SidebarWidth":          180,
	}
	data, err := plist.Marshal(m, plist.BinaryFormat)
	if err != nil {
		return
	}
	s.records = append(s.records, Record{
		Filename: ".",
		Code:     CodeBwsp,
		DataType: DataBlob,
		Value:    data,
	})
}

// SetIconViewOptions configures the icon view appearance.
// This generates the icvp record (binary plist) which controls icon size,
// grid spacing, background, and other icon view properties.
func (s *Store) SetIconViewOptions(opts IconViewOptions) {
	// Apply defaults
	if opts.IconSize == 0 {
		opts.IconSize = 128
	}
	if opts.TextSize == 0 {
		opts.TextSize = 12
	}
	if opts.GridSpace == 0 {
		opts.GridSpace = 100
	} else if opts.GridSpace > 100 {
		// macOS Finder silently rejects the entire icvp plist if gridSpacing > 100,
		// causing all icon view settings to be ignored.
		opts.GridSpace = 100
	}

	m := map[string]interface{}{
		"viewOptionsVersion": 1,
		"iconSize":           opts.IconSize,
		"textSize":           opts.TextSize,
		"gridSpacing":        opts.GridSpace,
		"gridOffsetX":        opts.GridOffX,
		"gridOffsetY":        opts.GridOffY,
		"labelOnBottom":      opts.LabelOnBottom,
		"showIconPreview":    opts.ShowIconPreview,
		"showItemInfo":       opts.ShowItemInfo,
		"arrangeBy":          "none",
		"scrollPositionX":    float64(0),
		"scrollPositionY":    float64(0),
	}

	switch opts.BackgroundType {
	case BackgroundDefault:
		// Finder rejects backgroundType=0, so use white color as default
		m["backgroundType"] = 1
		m["backgroundColorRed"] = float64(1)
		m["backgroundColorGreen"] = float64(1)
		m["backgroundColorBlue"] = float64(1)
	case BackgroundColor:
		m["backgroundType"] = 1
		m["backgroundColorRed"] = opts.BackgroundRed
		m["backgroundColorGreen"] = opts.BackgroundGreen
		m["backgroundColorBlue"] = opts.BackgroundBlue
	case BackgroundPicture:
		m["backgroundType"] = 2
		if opts.BackgroundAlias != nil {
			m["backgroundImageAlias"] = opts.BackgroundAlias
		}
		// White color fallback (matches dmgbuild golden output)
		m["backgroundColorRed"] = float64(1)
		m["backgroundColorGreen"] = float64(1)
		m["backgroundColorBlue"] = float64(1)
	}

	data, err := plist.Marshal(m, plist.BinaryFormat)
	if err != nil {
		return
	}
	s.records = append(s.records, Record{
		Filename: ".",
		Code:     CodeIcvp,
		DataType: DataBlob,
		Value:    data,
	})

	// Add icvl (icon view layout) record - required by modern Finder
	s.records = append(s.records, Record{
		Filename: ".",
		Code:     CodeIcvl,
		DataType: DataType,
		Value:    ViewIcon,
	})
}

// SetBackgroundColor sets a solid color background for the Finder window.
func (s *Store) SetBackgroundColor(r, g, b float64) {
	s.SetIconViewOptions(IconViewOptions{
		BackgroundType:  BackgroundColor,
		BackgroundRed:   r,
		BackgroundGreen: g,
		BackgroundBlue:  b,
		LabelOnBottom:   true,
		ShowIconPreview: true,
	})
}

// SetBackgroundImage sets a background image for the Finder window.
// volumeName is the DMG volume name, relativePath is the image path
// relative to the volume root (e.g., ".background/background.png").
func (s *Store) SetBackgroundImage(volumeName, relativePath string, opts *IconViewOptions, aliasInfo *AliasFileInfo) {
	alias := BuildAlias(volumeName, relativePath, aliasInfo)

	viewOpts := IconViewOptions{
		BackgroundType:  BackgroundPicture,
		BackgroundAlias: alias,
		LabelOnBottom:   true,
	}
	if opts != nil {
		viewOpts.IconSize = opts.IconSize
		viewOpts.TextSize = opts.TextSize
		viewOpts.GridSpace = opts.GridSpace
		viewOpts.GridOffX = opts.GridOffX
		viewOpts.GridOffY = opts.GridOffY
		if opts.LabelOnBottom {
			viewOpts.LabelOnBottom = true
		}
		if opts.ShowIconPreview {
			viewOpts.ShowIconPreview = true
		}
	}
	s.SetIconViewOptions(viewOpts)
}

// EnsureDefaults adds standard records if not already present:
// vSrn (version=1) is always required.
func (s *Store) EnsureDefaults() {
	hasVSrn := false
	for _, r := range s.records {
		if r.Code == CodeVSrn {
			hasVSrn = true
		}
	}
	if !hasVSrn {
		s.records = append(s.records, NewVSrnRecord())
	}
}

// WriteTo serializes the .DS_Store file to w.
func (s *Store) WriteTo(w io.Writer) error {
	s.EnsureDefaults()

	// Sort records (case-insensitive filename, then code)
	sorted := make([]Record, len(s.records))
	copy(sorted, s.records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Less(sorted[j])
	})

	// Build B-tree
	tree := buildBTree(sorted)

	// Assemble via buddy allocator
	builder := newBuddyBuilder()

	// Block 0 is reserved for bookkeeping (handled by buddyBuilder).
	// DSDB master block gets block ID 1+.
	masterID := builder.allocate(tree.masterBlock)
	builder.setTOC("DSDB", masterID)

	// B-tree node blocks
	nodeBlockIDs := make([]uint32, len(tree.nodeBlocks))
	for i, nodeData := range tree.nodeBlocks {
		nodeBlockIDs[i] = builder.allocate(nodeData)
	}

	// Rewrite the master block with corrected root node block ID.
	// The B-tree's rootBlockID (from buildBTree) is an index into nodeBlocks.
	// The actual block ID is nodeBlockIDs[rootIndex].
	rootIndex := readU32(tree.masterBlock, 0) // original root index
	correctedMaster := make([]byte, len(tree.masterBlock))
	copy(correctedMaster, tree.masterBlock)
	putU32(correctedMaster, 0, nodeBlockIDs[rootIndex])
	builder.setBlockData(masterID, correctedMaster)

	// Tell the buddy allocator which block is the root B-tree node,
	// so it can write the packed address into the Bud1 header at offset 16.
	builder.setRootNodeBlockID(nodeBlockIDs[rootIndex])

	// Also fix internal node child pointers
	for i, nodeData := range tree.nodeBlocks {
		mode := readU32(nodeData, 0)
		if mode > 0 {
			// Internal node: fix all child block IDs
			fixed := fixInternalNodeBlockIDs(nodeData, nodeBlockIDs)
			builder.setBlockData(nodeBlockIDs[i], fixed)
		}
	}

	return builder.writeTo(w)
}

// WriteFile writes the .DS_Store to a file.
func (s *Store) WriteFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.WriteTo(f)
}

// fixInternalNodeBlockIDs rewrites an internal node's child block IDs
// using the mapping from btree indices to actual buddy block IDs.
func fixInternalNodeBlockIDs(nodeData []byte, mapping []uint32) []byte {
	fixed := make([]byte, len(nodeData))
	copy(fixed, nodeData)

	// Fix mode (rightmost child)
	mode := readU32(fixed, 0)
	if mode < uint32(len(mapping)) {
		putU32(fixed, 0, mapping[mode])
	}

	// Fix interleaved child pointers
	count := readU32(fixed, 4)
	offset := 8
	for i := uint32(0); i < count; i++ {
		childID := readU32(fixed, offset)
		if childID < uint32(len(mapping)) {
			putU32(fixed, offset, mapping[childID])
		}
		offset += 4

		// Skip over the record data — we need to calculate its size
		offset += recordSizeAt(fixed, offset)
	}

	return fixed
}

// recordSizeAt reads a serialized record starting at offset in data
// and returns its total byte size.
func recordSizeAt(data []byte, offset int) int {
	if offset+4 > len(data) {
		return 0
	}
	// Filename: 4 bytes length + 2*length bytes
	nameLen := int(readU32(data, offset))
	pos := offset + 4 + nameLen*2

	// Structure type: 4 bytes
	pos += 4

	// Data type: 4 bytes
	if pos+4 > len(data) {
		return pos - offset
	}
	var dt FourCharCode
	copy(dt[:], data[pos:pos+4])
	pos += 4

	// Value payload
	switch dt {
	case DataLong:
		pos += 4
	case DataShor:
		pos += 4
	case DataBool:
		pos += 1
	case DataBlob:
		if pos+4 <= len(data) {
			blobLen := int(readU32(data, pos))
			pos += 4 + blobLen
		}
	case DataType:
		pos += 4
	case DataUstr:
		if pos+4 <= len(data) {
			strLen := int(readU32(data, pos))
			pos += 4 + strLen*2
		}
	case DataComp:
		pos += 8
	case DataDutc:
		pos += 8
	}

	return pos - offset
}

// readU32 reads a big-endian uint32 from data at offset.
func readU32(data []byte, offset int) uint32 {
	return uint32(data[offset])<<24 |
		uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 |
		uint32(data[offset+3])
}

// putU32 writes a big-endian uint32 into data at offset.
func putU32(data []byte, offset int, v uint32) {
	data[offset] = byte(v >> 24)
	data[offset+1] = byte(v >> 16)
	data[offset+2] = byte(v >> 8)
	data[offset+3] = byte(v)
}

// formatRect creates a Cocoa-style rect string "{{x, y}, {w, h}}".
func formatRect(top, left, bottom, right int) string {
	w := right - left
	h := bottom - top
	return "{{" + itoa(left) + ", " + itoa(top) + "}, {" + itoa(w) + ", " + itoa(h) + "}}"
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
