// Package crdt implements a sequence CRDT (RGA, Roh et al.) for
// collaborative plain text. The same code runs on the server and inside the
// wasm client, so both sides merge operations identically and converge.
//
// IDs are (site, lamport) pairs; deletes are tombstones. Integration follows
// the original RGA rule: insert after the anchor, skipping consecutive
// characters whose ID is greater than the new one.
package crdt

import "strings"

// ID identifies a character: Site is the replica, Seq a Lamport timestamp.
type ID struct {
	Site uint32 `json:"s"`
	Seq  uint64 `json:"q"`
}

func (a ID) IsZero() bool { return a.Site == 0 && a.Seq == 0 }

// Less orders IDs by (Seq, Site) — Lamport order, ties broken by site.
func (a ID) Less(b ID) bool {
	if a.Seq != b.Seq {
		return a.Seq < b.Seq
	}
	return a.Site < b.Site
}

// Char is one character of the sequence (possibly tombstoned).
type Char struct {
	ID      ID     `json:"id"`
	After   ID     `json:"af"`          // anchor: ID of the char it was typed after
	Rune    string `json:"r"`           // exactly one rune
	Deleted bool   `json:"d,omitempty"` // tombstone
}

// Op is a replicated operation.
type Op struct {
	Kind string `json:"k"` // "i" insert, "d" delete
	Char Char   `json:"c"` // delete uses only Char.ID
}

// RGA is one replica of the shared text.
type RGA struct {
	site    uint32
	clock   uint64
	chars   []Char
	index   map[ID]int
	applied map[ID]bool
	pending []Op // ops whose anchor has not arrived yet
}

func New(site uint32) *RGA {
	return &RGA{site: site, index: map[ID]int{}, applied: map[ID]bool{}}
}

// Load builds a replica from a snapshot.
func Load(site uint32, chars []Char) *RGA {
	r := New(site)
	r.chars = append(r.chars, chars...)
	for i, c := range r.chars {
		r.index[c.ID] = i
		r.applied[c.ID] = true
		if c.ID.Seq > r.clock {
			r.clock = c.ID.Seq
		}
	}
	return r
}

// Snapshot returns a copy of the full sequence including tombstones.
func (r *RGA) Snapshot() []Char {
	out := make([]Char, len(r.chars))
	copy(out, r.chars)
	return out
}

// String returns the visible text.
func (r *RGA) String() string {
	var sb strings.Builder
	for _, c := range r.chars {
		if !c.Deleted {
			sb.WriteString(c.Rune)
		}
	}
	return sb.String()
}

// Len returns the number of visible characters.
func (r *RGA) Len() int {
	n := 0
	for _, c := range r.chars {
		if !c.Deleted {
			n++
		}
	}
	return n
}

// visibleIndex returns the chars index of the pos-th visible character.
func (r *RGA) visibleIndex(pos int) int {
	seen := 0
	for i, c := range r.chars {
		if c.Deleted {
			continue
		}
		if seen == pos {
			return i
		}
		seen++
	}
	return -1
}

// visiblePosOf returns how many visible chars precede chars[idx].
func (r *RGA) visiblePosOf(idx int) int {
	pos := 0
	for i := 0; i < idx && i < len(r.chars); i++ {
		if !r.chars[i].Deleted {
			pos++
		}
	}
	return pos
}

func (r *RGA) reindexFrom(from int) {
	for i := from; i < len(r.chars); i++ {
		r.index[r.chars[i].ID] = i
	}
}

// integrate places c following the RGA rule and returns his array index.
func (r *RGA) integrate(c Char) int {
	pos := 0
	if !c.After.IsZero() {
		pos = r.index[c.After] + 1
	}
	for pos < len(r.chars) && c.ID.Less(r.chars[pos].ID) {
		pos++
	}
	r.chars = append(r.chars, Char{})
	copy(r.chars[pos+1:], r.chars[pos:])
	r.chars[pos] = c
	r.reindexFrom(pos)
	if c.ID.Seq > r.clock {
		r.clock = c.ID.Seq
	}
	return pos
}

// LocalInsert inserts rune at the visible position and returns the op to
// replicate.
func (r *RGA) LocalInsert(pos int, ru rune) Op {
	r.clock++
	c := Char{ID: ID{Site: r.site, Seq: r.clock}, Rune: string(ru)}
	if pos > 0 {
		if idx := r.visibleIndex(pos - 1); idx >= 0 {
			c.After = r.chars[idx].ID
		}
	}
	r.applied[c.ID] = true
	r.integrate(c)
	return Op{Kind: "i", Char: c}
}

// LocalDelete tombstones the visible char at pos and returns the op, or
// ok=false when pos is out of range.
func (r *RGA) LocalDelete(pos int) (Op, bool) {
	idx := r.visibleIndex(pos)
	if idx < 0 {
		return Op{}, false
	}
	r.chars[idx].Deleted = true
	return Op{Kind: "d", Char: Char{ID: r.chars[idx].ID}}, true
}

// Apply merges a remote op. It returns the visible position affected
// (insert: where the char landed; delete: where it used to be) and whether
// the op changed the document. Duplicated ops are ignored; inserts whose
// anchor has not arrived yet are buffered and retried.
func (r *RGA) Apply(op Op) (visPos int, changed bool) {
	switch op.Kind {
	case "i":
		if r.applied[op.Char.ID] {
			return 0, false
		}
		if !op.Char.After.IsZero() {
			if _, ok := r.index[op.Char.After]; !ok {
				r.pending = append(r.pending, op)
				return 0, false
			}
		}
		r.applied[op.Char.ID] = true
		idx := r.integrate(op.Char)
		visPos = r.visiblePosOf(idx)
		r.drainPending()
		return visPos, true
	case "d":
		idx, ok := r.index[op.Char.ID]
		if !ok || r.chars[idx].Deleted {
			return 0, false
		}
		visPos = r.visiblePosOf(idx)
		r.chars[idx].Deleted = true
		return visPos, true
	}
	return 0, false
}

func (r *RGA) drainPending() {
	for progress := true; progress && len(r.pending) > 0; {
		progress = false
		rest := r.pending[:0]
		queue := r.pending
		r.pending = nil
		for _, op := range queue {
			if _, ok := r.index[op.Char.After]; ok && !r.applied[op.Char.ID] {
				r.applied[op.Char.ID] = true
				r.integrate(op.Char)
				progress = true
			} else if !r.applied[op.Char.ID] {
				rest = append(rest, op)
			}
		}
		r.pending = rest
	}
}

// DiffOps computes the local ops that turn the current text into target
// (single edited span: common prefix/suffix). Returns the ops to replicate.
func (r *RGA) DiffOps(target string) []Op {
	cur := []rune(r.String())
	tgt := []rune(target)
	p := 0
	for p < len(cur) && p < len(tgt) && cur[p] == tgt[p] {
		p++
	}
	s := 0
	for s < len(cur)-p && s < len(tgt)-p && cur[len(cur)-1-s] == tgt[len(tgt)-1-s] {
		s++
	}
	var ops []Op
	for i := len(cur) - s - 1; i >= p; i-- { // delete back to front
		if op, ok := r.LocalDelete(i); ok {
			ops = append(ops, op)
		}
	}
	for i := p; i < len(tgt)-s; i++ {
		ops = append(ops, r.LocalInsert(i, tgt[i]))
	}
	return ops
}
