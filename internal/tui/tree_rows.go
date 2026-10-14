package tui

import (
	"fmt"
	"strings"

	"mvn-search/internal/maven"
)

type rowKind int

const (
	rowGroup   rowKind = iota // groupId path node (expandable)
	rowPackage                // artifactId package (expandable; copy = latest)
	rowLeaf                   // version leaf (copy = that version)
)

// row is one visible display line of the result list.
type row struct {
	kind     rowKind
	depth    int
	branch   string
	text     string
	leafIdx  int    // result index used for copy/detail
	key      string // stable expand key
	parent   string // parent expand key ("" = root)
	expanded bool
	childCnt int
}

// expandState tracks explicitly toggled tree nodes (missing key = default).
type expandState map[string]bool

func isOpen(expanded expandState, key string, def bool) bool {
	if v, ok := expanded[key]; ok {
		return v
	}
	return def
}

// buildRowsWithQuery builds a path-prefix tree from the package level up:
//
//	com.illtamer
//	  ├─ lib:easy-plugin          ·3  ±
//	  └─ plugin                   ·6  ±
//	       ├─ custom-vault        ·1  ±
//	       └─ acquisitionstore:acquisition-store  ·1  ±
//
// groupId segments form the spine (`.` paths nest); artifactIds hang under the
// exact groupId node; versions hang under packages. Single-child paths fold.
func buildRowsWithQuery(results []maven.Artifact, query string, expanded expandState) []row {
	if len(results) == 0 {
		return nil
	}
	if strings.TrimSpace(query) == "" {
		return flatRows(results)
	}
	if expanded == nil {
		expanded = expandState{}
	}
	return decorate(buildPathTree(results, expanded))
}

func flatRows(results []maven.Artifact) []row {
	out := make([]row, len(results))
	for i, a := range results {
		out[i] = row{
			kind:    rowLeaf,
			depth:   0,
			text:    a.GAV(),
			leafIdx: i,
			key:     a.GAV(),
		}
	}
	return out
}

// --- trie over groupId path segments + artifactId + version ---

type pkgNode struct {
	id       string
	key      string // ga
	versions []int  // result indexes
}

type pathNode struct {
	seg  string // last segment label
	key  string // full dotted path ("" for root)

	paths map[string]*pathNode // child path segments (insertion order via order)
	pkgs  map[string]*pkgNode  // artifactIds with this exact groupId

	pathOrder []string
	pkgOrder  []string

	// subtree stats
	pkgCount int // packages in this subtree
	resFirst int // first result index in subtree (for copy)
	resCount int
}

func newPath(seg, key string) *pathNode {
	return &pathNode{
		seg:  seg,
		key:  key,
		paths: map[string]*pathNode{},
		pkgs:  map[string]*pkgNode{},
		resFirst: -1,
	}
}

func (n *pathNode) ensurePath(seg string) *pathNode {
	if c, ok := n.paths[seg]; ok {
		return c
	}
	key := seg
	if n.key != "" {
		key = n.key + "." + seg
	}
	c := newPath(seg, key)
	n.paths[seg] = c
	n.pathOrder = append(n.pathOrder, seg)
	return c
}

func (n *pathNode) ensurePkg(id string) *pkgNode {
	if p, ok := n.pkgs[id]; ok {
		return p
	}
	ga := id
	if n.key != "" {
		ga = n.key + ":" + id
	}
	// For folded display we want full ga even when key is path.
	if n.key != "" {
		ga = n.key + ":" + id
	}
	p := &pkgNode{id: id, key: ga}
	n.pkgs[id] = p
	n.pkgOrder = append(n.pkgOrder, id)
	return p
}

func insertPath(root *pathNode, a maven.Artifact, idx int) {
	segs := strings.Split(a.GroupID, ".")
	cur := root
	for _, s := range segs {
		if s == "" {
			continue
		}
		cur = cur.ensurePath(s)
	}
	p := cur.ensurePkg(a.ArtifactID)
	p.versions = append(p.versions, idx)
}

func countSubtree(n *pathNode) (pkgs, res int) {
	for _, p := range n.pkgs {
		pkgs++
		res += len(p.versions)
	}
	for _, c := range n.paths {
		p, r := countSubtree(c)
		pkgs += p
		res += r
	}
	return pkgs, res
}

func firstResult(n *pathNode) int {
	if len(n.pkgs) > 0 {
		// first package in order
		id := n.pkgOrder[0]
		if p := n.pkgs[id]; len(p.versions) > 0 {
			return p.versions[0]
		}
	}
	for _, seg := range n.pathOrder {
		if i := firstResult(n.paths[seg]); i >= 0 {
			return i
		}
	}
	return -1
}

// buildPathTree emits rows from the groupId path trie.
func buildPathTree(results []maven.Artifact, expanded expandState) []row {
	root := newPath("", "")
	for i, a := range results {
		insertPath(root, a, i)
	}
	// compute subtree stats
	var walk func(n *pathNode)
	walk = func(n *pathNode) {
		n.pkgCount, n.resCount = countSubtree(n)
		n.resFirst = firstResult(n)
		for _, seg := range n.pathOrder {
			walk(n.paths[seg])
		}
	}
	walk(root)

	var rows []row
	emitPathChildren(root, &rows, "", 0, results, expanded)
	return rows
}

// emitPathChildren emits this node's children: local packages first, then
// folded path subtrees (so `com.illtamer.plugin` lists custom-vault before
// nested `com.illtamer.plugin.acquisitionstore`).
func emitPathChildren(n *pathNode, rows *[]row, parentKey string, depth int, results []maven.Artifact, expanded expandState) {
	for _, id := range n.pkgOrder {
		p := n.pkgs[id]
		emitPackage(p, rows, parentKey, depth, results, expanded)
	}
	for _, seg := range n.pathOrder {
		child := n.paths[seg]
		label, leaf := foldPath(child)
		emitFoldedPath(leaf, label, rows, parentKey, depth, results, expanded)
	}
}

// foldPath collapses a single-child path chain (no local packages) into one label.
func foldPath(n *pathNode) (label string, end *pathNode) {
	label = n.seg
	cur := n
	for {
		if len(cur.pkgs) > 0 || len(cur.pathOrder) != 1 {
			return label, cur
		}
		next := cur.paths[cur.pathOrder[0]]
		label = label + "." + next.seg
		cur = next
	}
}

func emitFoldedPath(n *pathNode, label string, rows *[]row, parentKey string, depth int, results []maven.Artifact, expanded expandState) {
	// If this node has exactly one package and no path children, fold to package
	// row "label:artifact" (or just use package emit with custom text).
	if len(n.paths) == 0 && len(n.pkgs) == 1 && len(n.pkgOrder) == 1 {
		p := n.pkgs[n.pkgOrder[0]]
		text := label + ":" + p.id
		if n.key == "" {
			text = p.id
		}
		pOpen := isOpen(expanded, p.key, false)
		*rows = append(*rows, row{
			kind:     rowPackage,
			depth:    depth,
			text:     text,
			leafIdx:  p.versions[0],
			key:      p.key,
			parent:   parentKey,
			expanded: pOpen,
			childCnt: len(p.versions),
		})
		if pOpen {
			emitVersions(p, rows, depth+1, results)
		}
		return
	}

	key := n.key
	gOpen := isOpen(expanded, key, true) // path nodes default open
	*rows = append(*rows, row{
		kind:     rowGroup,
		depth:    depth,
		text:     label,
		leafIdx:  n.resFirst,
		key:      key,
		parent:   parentKey,
		expanded: gOpen,
		childCnt: n.pkgCount,
	})
	if !gOpen {
		return
	}
	emitPathChildren(n, rows, key, depth+1, results, expanded)
}

func emitPackage(p *pkgNode, rows *[]row, parentKey string, depth int, results []maven.Artifact, expanded expandState) {
	pOpen := isOpen(expanded, p.key, false)
	*rows = append(*rows, row{
		kind:     rowPackage,
		depth:    depth,
		text:     p.id,
		leafIdx:  p.versions[0],
		key:      p.key,
		parent:   parentKey,
		expanded: pOpen,
		childCnt: len(p.versions),
	})
	if pOpen {
		emitVersions(p, rows, depth+1, results)
	}
}

func emitVersions(p *pkgNode, rows *[]row, depth int, results []maven.Artifact) {
	for _, i := range p.versions {
		*rows = append(*rows, row{
			kind:    rowLeaf,
			depth:   depth,
			text:    results[i].Version,
			leafIdx: i,
			key:     p.key + "\x00" + results[i].Version,
			parent:  p.key,
		})
	}
}

// decorate prefixes ├─ / └─ / │ connectors for non-root rows.
func decorate(rows []row) []row {
	n := len(rows)
	if n == 0 {
		return rows
	}
	out := make([]row, n)
	copy(out, rows)

	children := make(map[string][]int)
	for i, r := range rows {
		children[r.parent] = append(children[r.parent], i)
	}
	parentOf := make(map[string]string, n)
	for _, r := range rows {
		parentOf[r.key] = r.parent
	}
	rowOf := make(map[string]int, n)
	for i, r := range rows {
		rowOf[r.key] = i
	}

	for i, r := range out {
		if r.depth == 0 {
			out[i].branch = ""
			continue
		}
		sibs := children[r.parent]
		last := len(sibs) > 0 && sibs[len(sibs)-1] == i
		branch := "├─ "
		if last {
			branch = "└─ "
		}

		var b strings.Builder
		var chain []string
		for k := r.parent; k != ""; k = parentOf[k] {
			chain = append(chain, k)
		}
		for l, k := 0, len(chain)-1; l < k; l, k = l+1, k-1 {
			chain[l], chain[k] = chain[k], chain[l]
		}
		for _, ancKey := range chain {
			ancSibs := children[parentOf[ancKey]]
			ancIdx := rowOf[ancKey]
			if len(ancSibs) > 0 && ancSibs[len(ancSibs)-1] != ancIdx {
				b.WriteString("│  ")
			} else {
				b.WriteString("   ")
			}
		}
		out[i].branch = b.String() + branch
	}
	return out
}

func summaryCount(n int) string {
	return fmt.Sprintf("·%d", n)
}
