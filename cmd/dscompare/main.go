package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"time"

	"howett.net/plist"
)

type RecordKey struct {
	Filename   string
	StructCode string
}

type Record struct {
	Key      RecordKey
	DataType string
	Value    interface{}
	RawBlob  []byte // Store raw blob for comparison
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file1.DS_Store> <file2.DS_Store>\n", os.Args[0])
		os.Exit(1)
	}

	records1, err := parseFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}

	records2, err := parseFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", os.Args[2], err)
		os.Exit(1)
	}

	// Display both files
	fmt.Printf("=== FILE: %s ===\n", os.Args[1])
	displayRecords(records1)
	fmt.Println()

	fmt.Printf("=== FILE: %s ===\n", os.Args[2])
	displayRecords(records2)
	fmt.Println()

	// Show differences
	fmt.Println("=== DIFFERENCES ===")
	compareRecords(records1, records2)
}

func parseFile(path string) (map[RecordKey]*Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	records := make(map[RecordKey]*Record)

	// Known record codes to search for
	codes := []string{"Iloc", "icvp", "bwsp", "vSrn", "vstl", "BKGD", "fwi0", "pict"}

	for _, code := range codes {
		findRecords(data, code, records)
	}

	return records, nil
}

func findRecords(data []byte, code string, records map[RecordKey]*Record) {
	codeBytes := []byte(code)

	for i := 0; i < len(data)-4; i++ {
		if bytes.Equal(data[i:i+4], codeBytes) {
			// Try to parse record starting before this position
			if rec := tryParseRecordAt(data, i); rec != nil {
				records[rec.Key] = rec
			}
		}
	}
}

func tryParseRecordAt(data []byte, codePos int) *Record {
	// Work backwards to find filename
	// Format: uint32 length + UTF-16BE chars + 4-byte code + 4-byte dataType + value

	if codePos < 8 {
		return nil
	}

	// Try to read dataType after code
	if codePos+8 > len(data) {
		return nil
	}

	code := string(data[codePos : codePos+4])
	dataType := string(data[codePos+4 : codePos+8])

	// Validate dataType
	validTypes := map[string]bool{
		"blob": true, "long": true, "shor": true, "bool": true,
		"type": true, "ustr": true, "comp": true, "dutc": true,
	}
	if !validTypes[dataType] {
		return nil
	}

	// Try to find filename before code
	// Search backwards for reasonable UTF-16BE string
	filenameStart := -1
	filenameLen := uint32(0)

	for offset := codePos - 4; offset >= 0 && offset > codePos-200; offset -= 4 {
		if offset < 4 {
			break
		}

		// Try reading length
		possibleLen := binary.BigEndian.Uint32(data[offset : offset+4])

		// Reasonable filename length (1-100 chars)
		if possibleLen > 0 && possibleLen <= 100 {
			expectedEnd := offset + 4 + int(possibleLen)*2
			if expectedEnd == codePos {
				// Verify it looks like UTF-16BE
				if isValidUTF16BE(data[offset+4 : expectedEnd]) {
					filenameStart = offset
					filenameLen = possibleLen
					break
				}
			}
		}
	}

	if filenameStart == -1 {
		return nil
	}

	// Parse filename
	filename := decodeUTF16BE(data[filenameStart+4 : filenameStart+4+int(filenameLen)*2])

	// Parse value based on dataType
	valueStart := codePos + 8
	var value interface{}
	var rawBlob []byte

	switch dataType {
	case "long":
		if valueStart+4 > len(data) {
			return nil
		}
		value = binary.BigEndian.Uint32(data[valueStart : valueStart+4])
		rawBlob = data[valueStart : valueStart+4]

	case "shor":
		if valueStart+4 > len(data) {
			return nil
		}
		value = binary.BigEndian.Uint32(data[valueStart : valueStart+4])
		rawBlob = data[valueStart : valueStart+4]

	case "bool":
		if valueStart+1 > len(data) {
			return nil
		}
		value = data[valueStart] != 0
		rawBlob = data[valueStart : valueStart+1]

	case "blob":
		if valueStart+4 > len(data) {
			return nil
		}
		blobLen := binary.BigEndian.Uint32(data[valueStart : valueStart+4])
		if valueStart+4+int(blobLen) > len(data) {
			return nil
		}
		rawBlob = data[valueStart+4 : valueStart+4+int(blobLen)]
		value = rawBlob

	case "type":
		if valueStart+4 > len(data) {
			return nil
		}
		value = string(data[valueStart : valueStart+4])
		rawBlob = data[valueStart : valueStart+4]

	case "ustr":
		if valueStart+4 > len(data) {
			return nil
		}
		ustrLen := binary.BigEndian.Uint32(data[valueStart : valueStart+4])
		if valueStart+4+int(ustrLen)*2 > len(data) {
			return nil
		}
		value = decodeUTF16BE(data[valueStart+4 : valueStart+4+int(ustrLen)*2])
		rawBlob = data[valueStart : valueStart+4+int(ustrLen)*2]

	case "comp", "dutc":
		if valueStart+8 > len(data) {
			return nil
		}
		value = binary.BigEndian.Uint64(data[valueStart : valueStart+8])
		rawBlob = data[valueStart : valueStart+8]

	default:
		return nil
	}

	return &Record{
		Key: RecordKey{
			Filename:   filename,
			StructCode: code,
		},
		DataType: dataType,
		Value:    value,
		RawBlob:  rawBlob,
	}
}

func isValidUTF16BE(data []byte) bool {
	if len(data)%2 != 0 {
		return false
	}
	// Basic check: most chars should be in ASCII or common Unicode range
	for i := 0; i < len(data); i += 2 {
		if i+1 >= len(data) {
			return false
		}
		// Allow reasonable Unicode range
		high := data[i]
		low := data[i+1]
		if high > 0x20 || (high == 0 && low < 0x20 && low != 0) {
			return false
		}
	}
	return true
}

func decodeUTF16BE(data []byte) string {
	if len(data)%2 != 0 {
		return ""
	}
	runes := make([]rune, len(data)/2)
	for i := 0; i < len(runes); i++ {
		runes[i] = rune(binary.BigEndian.Uint16(data[i*2 : i*2+2]))
	}
	return string(runes)
}

func displayRecords(records map[RecordKey]*Record) {
	// Sort by filename, then struct code
	var keys []RecordKey
	for k := range records {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Filename != keys[j].Filename {
			return keys[i].Filename < keys[j].Filename
		}
		return keys[i].StructCode < keys[j].StructCode
	})

	for _, key := range keys {
		rec := records[key]
		fmt.Printf("Record: %s / %s (%s", rec.Key.Filename, rec.Key.StructCode, rec.DataType)

		switch rec.DataType {
		case "blob":
			blob := rec.Value.([]byte)
			fmt.Printf(", %d bytes)\n", len(blob))

			// Try to decode as plist
			if rec.Key.StructCode == "icvp" || rec.Key.StructCode == "bwsp" {
				var plistData interface{}
				if _, err := plist.Unmarshal(blob, &plistData); err == nil {
					displayPlist(plistData, "  ")

					// Special handling for icvp backgroundImageAlias
					if rec.Key.StructCode == "icvp" {
						if dict, ok := plistData.(map[string]interface{}); ok {
							if aliasData, ok := dict["backgroundImageAlias"].([]byte); ok {
								fmt.Println("  backgroundImageAlias: [alias record]")
								displayAlias(aliasData, "    ")
							}
						}
					}
				} else {
					fmt.Printf("  [binary blob, hex: %s...]\n", hex.EncodeToString(blob[:min(16, len(blob))]))
				}
			} else if rec.Key.StructCode == "Iloc" {
				// Parse Iloc: x(4), y(4), padding(6), tail(2)
				if len(blob) >= 16 {
					x := binary.BigEndian.Uint32(blob[0:4])
					y := binary.BigEndian.Uint32(blob[4:8])
					fmt.Printf("  = (%d, %d)\n", x, y)
				} else {
					fmt.Printf("  [invalid Iloc length: %d]\n", len(blob))
				}
			} else {
				fmt.Printf("  [binary blob, hex: %s...]\n", hex.EncodeToString(blob[:min(16, len(blob))]))
			}

		case "long", "shor":
			fmt.Printf(") = %d\n", rec.Value)

		case "bool":
			fmt.Printf(") = %t\n", rec.Value)

		case "type":
			fmt.Printf(") = %q\n", rec.Value)

		case "ustr":
			fmt.Printf(") = %q\n", rec.Value)

		case "comp", "dutc":
			fmt.Printf(") = 0x%016X\n", rec.Value)
		}
	}
}

func displayPlist(data interface{}, indent string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if key == "backgroundImageAlias" {
				// Skip here, we'll handle it specially
				continue
			}
			fmt.Printf("%s%s: ", indent, key)
			if nested, ok := val.(map[string]interface{}); ok {
				fmt.Println("{")
				displayPlist(nested, indent+"  ")
				fmt.Printf("%s}\n", indent)
			} else if arr, ok := val.([]interface{}); ok {
				fmt.Println("[")
				displayPlist(arr, indent+"  ")
				fmt.Printf("%s]\n", indent)
			} else {
				fmt.Printf("%v\n", val)
			}
		}
	case []interface{}:
		for _, val := range v {
			fmt.Printf("%s- %v\n", indent, val)
		}
	default:
		fmt.Printf("%s%v\n", indent, v)
	}
}

func displayAlias(data []byte, indent string) {
	if len(data) < 150 {
		fmt.Printf("%s[alias too short: %d bytes]\n", indent, len(data))
		return
	}

	offset := 0

	// Skip userType (4 bytes)
	offset += 4

	// Read header
	aliasSize := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	version := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	kind := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	kindStr := "file"
	if kind == 1 {
		kindStr = "dir"
	}

	// Volume name (Pascal string, 28 bytes total)
	volNameLen := int(data[offset])
	offset++
	volumeName := string(data[offset : offset+min(volNameLen, 27)])
	offset += 27

	volCreationDate := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	volSignature := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	driveType := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2
	parentCNID := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Filename (Pascal string, 64 bytes total)
	filenameLen := int(data[offset])
	offset++
	filename := string(data[offset : offset+min(filenameLen, 63)])
	offset += 63

	fileCNID := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	fileCreationDate := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	fmt.Printf("%sAlias Header:\n", indent)
	fmt.Printf("%s  aliasSize: %d, version: %d, kind: %s\n", indent, aliasSize, version, kindStr)
	fmt.Printf("%s  volume: %q, volCreDate: 0x%08X (%s)\n", indent, volumeName, volCreationDate, macTimeToString(volCreationDate))
	fmt.Printf("%s  volSignature: 0x%04X, driveType: 0x%04X\n", indent, volSignature, driveType)
	fmt.Printf("%s  parentCNID: %d\n", indent, parentCNID)
	fmt.Printf("%s  filename: %q, fileCNID: %d\n", indent, filename, fileCNID)
	fmt.Printf("%s  fileCreationDate: 0x%08X (%s)\n", indent, fileCreationDate, macTimeToString(fileCreationDate))

	// Skip to tagged data (after header + reserved fields)
	// fileType(4) + fileCreator(4) + nlvlFrom(2) + nlvlTo(2) + volAttrs(4) + volFSID(2) + reserved(10)
	offset += 4 + 4 + 2 + 2 + 4 + 2 + 10

	// Parse tagged data
	fmt.Printf("%sAlias Tags:\n", indent)
	for offset+4 <= len(data) {
		tag := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2
		if tag == 0xFFFF {
			break
		}

		length := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		if offset+int(length) > len(data) {
			fmt.Printf("%s  [incomplete tag 0x%04X]\n", indent, tag)
			break
		}

		tagData := data[offset : offset+int(length)]
		offset += int(length)

		// Decode tag
		switch tag {
		case 0x0000:
			fmt.Printf("%s  0x%04X (parent dir name): %q\n", indent, tag, string(tagData))
		case 0x0001:
			// CNID path (array of uint32)
			cnids := make([]uint32, len(tagData)/4)
			for i := range cnids {
				cnids[i] = binary.BigEndian.Uint32(tagData[i*4 : i*4+4])
			}
			fmt.Printf("%s  0x%04X (CNID path): %v\n", indent, tag, cnids)
		case 0x0002:
			fmt.Printf("%s  0x%04X (Carbon path): %q\n", indent, tag, string(tagData))
		case 0x000E:
			// Filename UTF-16BE
			if len(tagData) >= 2 {
				strLen := binary.BigEndian.Uint16(tagData[0:2])
				if int(strLen)*2+2 <= len(tagData) {
					fname := decodeUTF16BE(tagData[2 : 2+int(strLen)*2])
					fmt.Printf("%s  0x%04X (filename): %q\n", indent, tag, fname)
				}
			}
		case 0x000F:
			// Volume name UTF-16BE
			if len(tagData) >= 2 {
				strLen := binary.BigEndian.Uint16(tagData[0:2])
				if int(strLen)*2+2 <= len(tagData) {
					vname := decodeUTF16BE(tagData[2 : 2+int(strLen)*2])
					fmt.Printf("%s  0x%04X (volume name): %q\n", indent, tag, vname)
				}
			}
		case 0x0010:
			if len(tagData) == 8 {
				hiResDate := binary.BigEndian.Uint64(tagData)
				fmt.Printf("%s  0x%04X (hi-res vol creation date): 0x%016X\n", indent, tag, hiResDate)
			}
		case 0x0011:
			if len(tagData) == 8 {
				hiResDate := binary.BigEndian.Uint64(tagData)
				fmt.Printf("%s  0x%04X (hi-res file creation date): 0x%016X\n", indent, tag, hiResDate)
			}
		case 0x0012:
			fmt.Printf("%s  0x%04X (POSIX path): %q\n", indent, tag, string(tagData))
		case 0x0013:
			fmt.Printf("%s  0x%04X (mount point): %q\n", indent, tag, string(tagData))
		case 0x0014:
			if len(tagData) == 4 {
				fileID := binary.BigEndian.Uint32(tagData)
				fmt.Printf("%s  0x%04X (file ID): %d\n", indent, tag, fileID)
			}
		case 0x0015:
			fmt.Printf("%s  0x%04X (volume UUID): %q\n", indent, tag, string(tagData))
		default:
			fmt.Printf("%s  0x%04X: [%d bytes: %s...]\n", indent, tag, length, hex.EncodeToString(tagData[:min(8, len(tagData))]))
		}
	}
}

func macTimeToString(macTime uint32) string {
	// Mac epoch: Jan 1, 1904 00:00:00 GMT
	// Unix epoch: Jan 1, 1970 00:00:00 GMT
	// Difference: 2082844800 seconds
	if macTime == 0 {
		return "none"
	}
	unixTime := int64(macTime) - 2082844800
	t := time.Unix(unixTime, 0)
	return t.Format("2006-01-02 15:04:05")
}

func compareRecords(records1, records2 map[RecordKey]*Record) {
	// Collect all keys
	allKeys := make(map[RecordKey]bool)
	for k := range records1 {
		allKeys[k] = true
	}
	for k := range records2 {
		allKeys[k] = true
	}

	// Sort keys
	var keys []RecordKey
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Filename != keys[j].Filename {
			return keys[i].Filename < keys[j].Filename
		}
		return keys[i].StructCode < keys[j].StructCode
	})

	for _, key := range keys {
		rec1, exists1 := records1[key]
		rec2, exists2 := records2[key]

		if !exists1 {
			fmt.Printf("Record %s / %s: MISSING in file1, present in file2\n", key.Filename, key.StructCode)
			continue
		}
		if !exists2 {
			fmt.Printf("Record %s / %s: present in file1, MISSING in file2\n", key.Filename, key.StructCode)
			continue
		}

		// Compare values
		if rec1.DataType != rec2.DataType {
			fmt.Printf("Record %s / %s: dataType differs (%s vs %s)\n", key.Filename, key.StructCode, rec1.DataType, rec2.DataType)
			continue
		}

		// For blobs, do detailed comparison
		if rec1.DataType == "blob" {
			if !bytes.Equal(rec1.RawBlob, rec2.RawBlob) {
				fmt.Printf("Record %s / %s: DIFFER\n", key.Filename, key.StructCode)

				// Try plist comparison
				if key.StructCode == "icvp" || key.StructCode == "bwsp" {
					var plist1, plist2 interface{}
					_, err1 := plist.Unmarshal(rec1.RawBlob, &plist1)
					_, err2 := plist.Unmarshal(rec2.RawBlob, &plist2)

					if err1 == nil && err2 == nil {
						comparePlists(plist1, plist2, "  ")
					}
				} else if key.StructCode == "Iloc" {
					if len(rec1.RawBlob) >= 8 && len(rec2.RawBlob) >= 8 {
						x1 := binary.BigEndian.Uint32(rec1.RawBlob[0:4])
						y1 := binary.BigEndian.Uint32(rec1.RawBlob[4:8])
						x2 := binary.BigEndian.Uint32(rec2.RawBlob[0:4])
						y2 := binary.BigEndian.Uint32(rec2.RawBlob[4:8])
						fmt.Printf("  Position: (%d, %d) vs (%d, %d)\n", x1, y1, x2, y2)
					}
				} else {
					fmt.Printf("  Hex diff: %s... vs %s...\n",
						hex.EncodeToString(rec1.RawBlob[:min(16, len(rec1.RawBlob))]),
						hex.EncodeToString(rec2.RawBlob[:min(16, len(rec2.RawBlob))]))
				}
			}
		} else {
			// Simple value comparison
			if !bytes.Equal(rec1.RawBlob, rec2.RawBlob) {
				fmt.Printf("Record %s / %s: %v vs %v\n", key.Filename, key.StructCode, rec1.Value, rec2.Value)
			}
		}
	}
}

func comparePlists(p1, p2 interface{}, indent string) {
	dict1, ok1 := p1.(map[string]interface{})
	dict2, ok2 := p2.(map[string]interface{})

	if !ok1 || !ok2 {
		fmt.Printf("%sNot comparable (different types)\n", indent)
		return
	}

	// Collect all keys
	allKeys := make(map[string]bool)
	for k := range dict1 {
		allKeys[k] = true
	}
	for k := range dict2 {
		allKeys[k] = true
	}

	for key := range allKeys {
		v1, exists1 := dict1[key]
		v2, exists2 := dict2[key]

		if !exists1 {
			fmt.Printf("%s%s: MISSING in file1\n", indent, key)
			continue
		}
		if !exists2 {
			fmt.Printf("%s%s: MISSING in file2\n", indent, key)
			continue
		}

		// Special handling for backgroundImageAlias
		if key == "backgroundImageAlias" {
			b1, ok1 := v1.([]byte)
			b2, ok2 := v2.([]byte)
			if ok1 && ok2 {
				if !bytes.Equal(b1, b2) {
					fmt.Printf("%s%s: DIFFER (comparing aliases)\n", indent, key)
					compareAliases(b1, b2, indent+"  ")
				}
			}
			continue
		}

		// Compare values
		if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			fmt.Printf("%s%s: %v vs %v\n", indent, key, v1, v2)
		}
	}
}

func compareAliases(a1, a2 []byte, indent string) {
	// Basic comparison - just show if they differ
	fmt.Printf("%sAlias records differ in %d bytes\n", indent, abs(len(a1)-len(a2)))

	// Could parse and compare tags here if needed
	// For now, just indicate they differ
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
