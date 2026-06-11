package main

// live-text binding: any element with a `live-text="docname"` attribute
// (textarea or input) becomes a collaborative editor. The element keeps a
// local CRDT replica (the same RGA package the server uses): local edits
// apply with zero latency and are replicated as ops; remote ops merge in
// while preserving the local cursor.

import (
	"encoding/json"
	"syscall/js"

	"github.com/arturoeanton/go-fiber-live-view/liveview/crdt"
)

type liveTextBinding struct {
	doc    string
	el     js.Value
	rep    *crdt.RGA
	ready  bool
	shadow string
}

var liveTexts = map[string]*liveTextBinding{}

type crdtWireMsg struct {
	Type  string      `json:"type"`
	Sub   string      `json:"sub"`
	Doc   string      `json:"doc"`
	Site  uint32      `json:"site,omitempty"`
	Chars []crdt.Char `json:"chars,omitempty"`
	Ops   []crdt.Op   `json:"ops,omitempty"`
}

func wsSend(v interface{}) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	ws.Call("send", string(payload))
}

// bindLiveTexts scans the DOM for live-text elements; new documents request
// a snapshot, re-rendered elements are re-attached with the replica value.
func bindLiveTexts() {
	defer func() { recover() }()
	list := document.Call("querySelectorAll", "[live-text]")
	n := list.Get("length").Int()
	for i := 0; i < n; i++ {
		el := list.Index(i)
		doc := el.Call("getAttribute", "live-text").String()
		if doc == "" {
			continue
		}
		b, ok := liveTexts[doc]
		if !ok {
			b = &liveTextBinding{doc: doc}
			liveTexts[doc] = b
			b.attach(el)
			wsSend(map[string]interface{}{"type": "crdt", "sub": "snap_req", "doc": doc})
			continue
		}
		if !b.el.Equal(el) { // the server re-rendered around it: rebind
			b.attach(el)
			if b.ready {
				el.Set("value", b.rep.String())
				b.shadow = b.rep.String()
			}
		}
	}
}

func (b *liveTextBinding) attach(el js.Value) {
	b.el = el
	el.Set("oninput", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		defer func() { recover() }()
		if !b.ready {
			return nil
		}
		val := b.el.Get("value").String()
		if val == b.shadow {
			return nil
		}
		ops := b.rep.DiffOps(val)
		b.shadow = val
		if len(ops) > 0 {
			wsSend(crdtWireMsg{Type: "crdt", Sub: "ops", Doc: b.doc, Ops: ops})
		}
		return nil
	}))
}

func handleCrdtMsg(raw string) {
	defer func() { recover() }()
	var m crdtWireMsg
	if json.Unmarshal([]byte(raw), &m) != nil {
		return
	}
	b := liveTexts[m.Doc]
	if b == nil {
		return
	}
	switch m.Sub {
	case "snap":
		b.rep = crdt.Load(m.Site, m.Chars)
		b.ready = true
		text := b.rep.String()
		b.el.Set("value", text)
		b.shadow = text
	case "ops":
		if !b.ready {
			return
		}
		selStart := b.el.Get("selectionStart").Int()
		selEnd := b.el.Get("selectionEnd").Int()
		for _, op := range m.Ops {
			pos, changed := b.rep.Apply(op)
			if !changed {
				continue
			}
			if op.Kind == "i" {
				if pos <= selStart {
					selStart++
				}
				if pos <= selEnd {
					selEnd++
				}
			} else {
				if pos < selStart {
					selStart--
				}
				if pos < selEnd {
					selEnd--
				}
			}
		}
		text := b.rep.String()
		b.el.Set("value", text)
		b.shadow = text
		if document.Get("activeElement").Equal(b.el) {
			b.el.Call("setSelectionRange", selStart, selEnd)
		}
	}
}
