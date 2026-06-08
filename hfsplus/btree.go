package hfsplus

import "sort"

// catRecord is one catalog B-tree record: its key (for sorting and index
// records) and the full key+data bytes stored in leaf nodes.
type catRecord struct {
	key  []byte
	full []byte
}

// lessKey reports whether catalog key a sorts before b, using HFSX binary
// comparison: first by parentID, then by UTF-16 code unit (big-endian).
func lessKey(a, b []byte) bool {
	pa, pb := be.Uint32(a[2:6]), be.Uint32(b[2:6])
	if pa != pb {
		return pa < pb
	}
	na, nb := int(be.Uint16(a[6:8])), int(be.Uint16(b[6:8]))
	for i := 0; i < na && i < nb; i++ {
		ca, cb := be.Uint16(a[8+i*2:]), be.Uint16(b[8+i*2:])
		if ca != cb {
			return ca < cb
		}
	}
	return na < nb
}

// nodeBytesNeeded returns how many bytes a node holding records of the given
// lengths would consume (descriptor + records + offset table).
func nodeBytesNeeded(recordLens []int) int {
	total := 14 + 2*(len(recordLens)+1) // descriptor + offset entries (incl. free)
	for _, l := range recordLens {
		total += l
	}
	return total
}

// packBySize greedily groups items so each group fits in one node.
func packBySize(n int, lenOf func(i int) int) [][]int {
	var groups [][]int
	var cur []int
	var curLens []int
	for i := 0; i < n; i++ {
		l := lenOf(i)
		if len(cur) > 0 && nodeBytesNeeded(append(curLens, l)) > nodeSize {
			groups = append(groups, cur)
			cur, curLens = nil, nil
		}
		cur = append(cur, i)
		curLens = append(curLens, l)
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	return groups
}

// buildCatalogBTree bulk-loads a catalog B-tree from records (which need not be
// pre-sorted) and returns the serialized tree (a multiple of nodeSize) and the
// number of nodes used.
func buildCatalogBTree(input []catRecord) (data []byte, usedNodes int) {
	// Sort a copy so the caller's slice order (and any indices into it) stay
	// stable across calls.
	records := make([]catRecord, len(input))
	copy(records, input)
	sort.SliceStable(records, func(i, j int) bool { return lessKey(records[i].key, records[j].key) })

	// Group records into leaf nodes.
	leafGroups := packBySize(len(records), func(i int) int { return len(records[i].full) })
	numLeaves := len(leafGroups)

	nodes := map[int][]byte{}
	for i, grp := range leafGroups {
		leafNum := i + 1 // node 0 is the header
		var fLink, bLink uint32
		if i < numLeaves-1 {
			fLink = uint32(i + 2)
		}
		if i > 0 {
			bLink = uint32(i)
		}
		recs := make([][]byte, len(grp))
		for j, ri := range grp {
			recs[j] = records[ri].full
		}
		nodes[leafNum] = buildNode(kindLeaf, 1, fLink, bLink, recs)
	}

	// Index entry: the first key of a child node and that child's node number.
	type idxEntry struct {
		key  []byte
		node int
	}
	level := make([]idxEntry, numLeaves)
	for i, grp := range leafGroups {
		level[i] = idxEntry{key: records[grp[0]].key, node: i + 1}
	}

	nextNode := numLeaves + 1
	rootNode := 1
	treeDepth := 1
	height := 2
	for len(level) > 1 {
		groups := packBySize(len(level), func(i int) int { return len(level[i].key) + 4 })
		var next []idxEntry
		for _, grp := range groups {
			nodeNum := nextNode
			nextNode++
			recs := make([][]byte, len(grp))
			for j, gi := range grp {
				e := level[gi]
				r := make([]byte, len(e.key)+4)
				copy(r, e.key)
				be.PutUint32(r[len(e.key):], uint32(e.node))
				recs[j] = r
			}
			nodes[nodeNum] = buildNode(kindIndex, height, 0, 0, recs)
			next = append(next, idxEntry{key: level[grp[0]].key, node: nodeNum})
		}
		level = next
		rootNode = next[0].node
		treeDepth = height
		height++
	}

	usedNodes = nextNode // nodes 0..nextNode-1

	// Header node (node 0).
	h := btHeaderParams{
		treeDepth:      uint16(treeDepth),
		rootNode:       uint32(rootNode),
		leafRecords:    uint32(len(records)),
		firstLeaf:      1,
		lastLeaf:       uint32(numLeaves),
		totalNodes:     uint32(usedNodes),
		freeNodes:      0,
		maxKeyLength:   maxCatalogKeyLen,
		keyCompareType: keyCompareBinary,
		attributes:     btBigKeysMask | btVariableIndexKeysMask,
	}
	nodes[0] = buildHeaderNode(h, usedNodes)

	out := make([]byte, usedNodes*nodeSize)
	for num, n := range nodes {
		copy(out[num*nodeSize:], n)
	}
	return out, usedNodes
}
