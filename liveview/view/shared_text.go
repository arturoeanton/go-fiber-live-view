package view

import (
	"encoding/json"
	"sync"

	"github.com/arturoeanton/go-fiber-live-view/liveview/crdt"
)

// SharedText is a collaboratively edited text backed by a CRDT (RGA). Bind
// it in any template with `<textarea live-text="name"></textarea>` (inputs
// work too): the wasm client replicates the document, applies local edits
// with zero latency and merges remote ones automatically. Server-side code
// reads it with Text(), writes it with SetText() and observes it with
// OnChange().
type SharedText struct {
	Name string

	mu       sync.Mutex
	rga      *crdt.RGA
	subs     map[*Conn]uint32 // connection -> site id
	nextSite uint32
	onChange []func(string)
}

var sharedTexts sync.Map // name -> *SharedText

// NewSharedText returns the shared document with that name, creating it
// with the given initial text when it does not exist yet.
func NewSharedText(name string, initial ...string) *SharedText {
	st := &SharedText{
		Name:     name,
		rga:      crdt.New(1), // site 1 is the server
		subs:     map[*Conn]uint32{},
		nextSite: 1,
	}
	actual, loaded := sharedTexts.LoadOrStore(name, st)
	st = actual.(*SharedText)
	if !loaded && len(initial) > 0 && initial[0] != "" {
		st.rga.DiffOps(initial[0])
	}
	return st
}

// Text returns the current document text.
func (st *SharedText) Text() string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.rga.String()
}

// SetText replaces the document from server code; connected editors receive
// the change as a minimal diff.
func (st *SharedText) SetText(text string) {
	st.mu.Lock()
	ops := st.rga.DiffOps(text)
	conns := st.subscribersLocked()
	cur := st.rga.String()
	st.mu.Unlock()
	if len(ops) == 0 {
		return
	}
	broadcastOps(st.Name, ops, conns, nil)
	st.fireChange(cur)
}

// OnChange registers fx to run (in his own goroutine) every time the
// document changes, with the new text.
func (st *SharedText) OnChange(fx func(text string)) {
	st.mu.Lock()
	st.onChange = append(st.onChange, fx)
	st.mu.Unlock()
}

func (st *SharedText) fireChange(text string) {
	st.mu.Lock()
	handlers := append([]func(string){}, st.onChange...)
	st.mu.Unlock()
	for _, fx := range handlers {
		go func(fx func(string)) {
			defer HandleRecover()
			fx(text)
		}(fx)
	}
}

func (st *SharedText) subscribersLocked() []*Conn {
	out := make([]*Conn, 0, len(st.subs))
	for c := range st.subs {
		out = append(out, c)
	}
	return out
}

// ------------------------------------------------------- wire protocol --

type crdtMsg struct {
	Type  string      `json:"type"`
	Sub   string      `json:"sub"`
	Doc   string      `json:"doc"`
	Site  uint32      `json:"site,omitempty"`
	Chars []crdt.Char `json:"chars,omitempty"`
	Ops   []crdt.Op   `json:"ops,omitempty"`
}

func broadcastOps(doc string, ops []crdt.Op, conns []*Conn, skip *Conn) {
	msg := crdtMsg{Type: "crdt", Sub: "ops", Doc: doc, Ops: ops}
	for _, c := range conns {
		if c == skip {
			continue
		}
		c.WriteJSON(msg)
	}
}

// handleCrdt processes a crdt message arriving on a page websocket.
func handleCrdt(conn *Conn, raw []byte) {
	var m crdtMsg
	if err := json.Unmarshal(raw, &m); err != nil || m.Doc == "" {
		return
	}
	st := NewSharedText(m.Doc)
	switch m.Sub {
	case "snap_req":
		st.mu.Lock()
		st.nextSite++
		site := st.nextSite
		st.subs[conn] = site
		snap := crdtMsg{Type: "crdt", Sub: "snap", Doc: st.Name, Site: site, Chars: st.rga.Snapshot()}
		st.mu.Unlock()
		conn.WriteJSON(snap)
	case "ops":
		if len(m.Ops) == 0 {
			return
		}
		st.mu.Lock()
		changed := false
		for _, op := range m.Ops {
			if _, ch := st.rga.Apply(op); ch {
				changed = true
			}
		}
		conns := st.subscribersLocked()
		text := st.rga.String()
		st.mu.Unlock()
		if !changed {
			return
		}
		broadcastOps(st.Name, m.Ops, conns, conn)
		st.fireChange(text)
	}
}

// unsubscribeConn detaches a closed connection from every shared document.
func unsubscribeConn(conn *Conn) {
	sharedTexts.Range(func(_, v interface{}) bool {
		st := v.(*SharedText)
		st.mu.Lock()
		delete(st.subs, conn)
		st.mu.Unlock()
		return true
	})
}
