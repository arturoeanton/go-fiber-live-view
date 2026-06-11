// Board: a collaborative whiteboard in the spirit of Excalidraw, rendered
// entirely from the server as SVG and persisted in SQLite.
//
//   - Tools: select/move, hand (pan), pen, rectangle, diamond, ellipse,
//     line, arrow, text and eraser
//   - Excalidraw-style palettes, fills and stroke widths
//   - Live collaboration: shared boards with named remote cursors
//   - Undo (Ctrl+Z), delete (Supr), zoom with the mouse wheel, SVG export
//   - Multiple boards, all stored in board.db (pure-Go sqlite driver)
//
// The only client-side code is a small pointer-capture glue: state,
// hit-testing, rendering and broadcast all live in Go.
package main

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------- model --

type Pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Element struct {
	ID         int64
	Kind       string // pen, rect, diamond, ellipse, line, arrow, text
	X, Y, W, H float64
	Points     []Pt
	Stroke     string
	Fill       string
	StrokeW    float64
	Text       string
}

func copyEl(e Element) Element {
	e.Points = append([]Pt(nil), e.Points...)
	return e
}

type Board struct {
	Name string
	Els  []*Element
}

func (b *Board) find(id int64) (*Element, int) {
	for i, e := range b.Els {
		if e.ID == id {
			return e, i
		}
	}
	return nil, -1
}

func (b *Board) remove(id int64) {
	if _, i := b.find(id); i >= 0 {
		b.Els = append(b.Els[:i], b.Els[i+1:]...)
	}
}

type undoOp struct {
	kind string // "add", "del", "mut"
	el   Element
}

type session struct {
	lid          string
	name, color  string
	board        string
	tool         string
	stroke, fill string
	width        float64
	selected     int64
	mode         string // "", "draw", "move", "erase"
	cur          *Element
	origEl       Element
	prevX, prevY float64
	curX, curY   float64
	hasCursor    bool
	undo         []undoOp
}

var (
	mu       sync.Mutex
	boards   = map[string]*Board{}
	sessions = map[string]*session{}
	userSeq  int
)

// getBoard lazily loads a board from sqlite. Callers must hold mu.
func getBoard(name string) *Board {
	if b, ok := boards[name]; ok {
		return b
	}
	b := &Board{Name: name, Els: dbLoadElements(name)}
	boards[name] = b
	return b
}

// ------------------------------------------------------------- palettes --

var (
	strokePalette = []string{"#1e1e1e", "#e03131", "#2f9e44", "#1971c2", "#f08c00", "#9c36b5"}
	fillPalette   = []string{"transparent", "#ffc9c9", "#b2f2bb", "#a5d8ff", "#ffec99", "#eebefa"}
	cursorColors  = []string{"#e03131", "#1971c2", "#2f9e44", "#f08c00", "#9c36b5", "#0c8599"}
	widthOptions  = []float64{2, 4, 6}
	validBoard    = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,24}$`)
)

func inPalette(p []string, c string) bool {
	for _, v := range p {
		if v == c {
			return true
		}
	}
	return false
}

type toolDef struct{ code, icon, title string }

var tools = []toolDef{
	{"select", "⬉", "Select / move"},
	{"hand", "✋", "Pan"},
	{"pen", "✏️", "Pen"},
	{"rect", "▭", "Rectangle"},
	{"diamond", "◇", "Diamond"},
	{"ellipse", "◯", "Ellipse"},
	{"line", "╱", "Line"},
	{"arrow", "↗", "Arrow"},
	{"text", "T", "Text"},
	{"eraser", "🧽", "Eraser"},
}

func validTool(code string) bool {
	for _, t := range tools {
		if t.code == code {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------- toolbar --

func renderToolbarLocked(s *session) string {
	var b strings.Builder
	for _, t := range tools {
		cls := "tb-btn"
		if s.tool == t.code {
			cls += " tb-active"
		}
		fmt.Fprintf(&b, `<button class="%s" title="%s" onclick="send_event('board','Tool','%s')">%s</button>`, cls, t.title, t.code, t.icon)
	}
	b.WriteString(`<span class="tb-sep"></span>`)
	for _, c := range strokePalette {
		cls := "tb-swatch"
		if s.stroke == c {
			cls += " tb-active"
		}
		fmt.Fprintf(&b, `<button class="%s" title="Stroke" style="background:%s" onclick="send_event('board','Stroke','%s')"></button>`, cls, c, c)
	}
	b.WriteString(`<span class="tb-sep"></span>`)
	for _, c := range fillPalette {
		cls := "tb-swatch"
		if s.fill == c {
			cls += " tb-active"
		}
		style := "background:" + c
		if c == "transparent" {
			style = "background:repeating-conic-gradient(#ddd 0 25%,#fff 0 50%) 0 0/10px 10px"
		}
		fmt.Fprintf(&b, `<button class="%s" title="Fill" style="%s" onclick="send_event('board','Fill','%s')"></button>`, cls, style, c)
	}
	b.WriteString(`<span class="tb-sep"></span>`)
	for i, w := range widthOptions {
		cls := "tb-btn"
		if s.width == w {
			cls += " tb-active"
		}
		label := []string{"S", "M", "L"}[i]
		fmt.Fprintf(&b, `<button class="%s" title="Stroke width" onclick="send_event('board','Width','%.0f')">%s</button>`, cls, w, label)
	}
	b.WriteString(`<span class="tb-sep"></span>`)
	b.WriteString(`<button class="tb-btn" title="Undo (Ctrl+Z)" onclick="send_event('board','Undo','')">↩</button>`)
	b.WriteString(`<button class="tb-btn" title="Delete selection (Supr)" onclick="send_event('board','Delete','')">🗑</button>`)
	b.WriteString(`<button class="tb-btn" title="Clear board" onclick="if(confirm('Clear the whole board?'))send_event('board','Clear','')">✕</button>`)
	b.WriteString(`<span class="tb-sep"></span>`)
	b.WriteString(`<button class="tb-btn" title="Zoom in" onclick="lvZoom(0.8)">＋</button>`)
	b.WriteString(`<button class="tb-btn" title="Zoom out" onclick="lvZoom(1.25)">－</button>`)
	b.WriteString(`<button class="tb-btn" title="Reset view" onclick="lvResetView()">⤢</button>`)
	b.WriteString(`<button class="tb-btn" title="Export SVG" onclick="lvExport()">⬇</button>`)
	b.WriteString(`<span class="tb-sep"></span>`)
	b.WriteString(`<select class="tb-select" onchange="send_event('board','Board',this.value)">`)
	for _, name := range dbListBoards() {
		sel := ""
		if name == s.board {
			sel = "selected"
		}
		fmt.Fprintf(&b, `<option value="%s" %s>%s</option>`, name, sel, name)
	}
	b.WriteString(`</select>`)
	b.WriteString(`<button class="tb-btn" title="New board" onclick="var n=prompt('Board name:'); if(n)send_event('board','NewBoard',n)">＋📋</button>`)
	return b.String()
}

func statusLocked(s *session) string {
	b := getBoard(s.board)
	users := 0
	for _, o := range sessions {
		if o.board == s.board {
			users++
		}
	}
	return fmt.Sprintf(`📋 <b>%s</b> · %d elements · %d online · you are <span style="color:%s">●</span> %s`,
		s.board, len(b.Els), users, s.color, s.name)
}

// ----------------------------------------------------------- client glue --

const glueJS = `
<script>
(function(){
	var svg=null, drawing=false, panning=false, lastMove=0, lastCursor=0, px=0, py=0;
	window.lvTool='pen';
	function ready(){ return typeof send_event==='function'; }
	function bp(e){
		var pt=svg.createSVGPoint(); pt.x=e.clientX; pt.y=e.clientY;
		var p=pt.matrixTransform(svg.getScreenCTM().inverse());
		return Math.round(p.x)+','+Math.round(p.y);
	}
	function vb(){ return svg.getAttribute('viewBox').split(' ').map(Number); }
	function setvb(p){ svg.setAttribute('viewBox', p.join(' ')); }
	function pan(dx,dy){ var p=vb(); var sc=p[2]/svg.clientWidth; p[0]-=dx*sc; p[1]-=dy*sc; setvb(p); }
	function zoomAt(f,cx,cy){
		var p=vb(), r=svg.getBoundingClientRect();
		var mx=p[0]+(cx-r.left)/r.width*p[2], my=p[1]+(cy-r.top)/r.height*p[3];
		p[2]*=f; p[3]*=f;
		p[0]=mx-(cx-r.left)/r.width*p[2]; p[1]=my-(cy-r.top)/r.height*p[3];
		setvb(p);
	}
	window.lvZoom=function(f){ var r=svg.getBoundingClientRect(); zoomAt(f, r.left+r.width/2, r.top+r.height/2); };
	window.lvResetView=function(){ setvb([0,0,1600,1000]); };
	window.lvExport=function(){
		var c=svg.cloneNode(true); var ov=c.querySelector('#overlay_layer'); if(ov)ov.remove();
		var s=new XMLSerializer().serializeToString(c);
		var a=document.createElement('a');
		a.href=URL.createObjectURL(new Blob([s],{type:'image/svg+xml'}));
		a.download='board.svg'; a.click();
	};
	function init(){
		svg=document.getElementById('board-svg');
		if(!svg){ setTimeout(init,300); return; }
		svg.addEventListener('pointerdown', function(e){
			if(!ready())return;
			if(window.lvTool==='hand'||e.button===1){ panning=true; px=e.clientX; py=e.clientY; return; }
			if(e.button!==0)return;
			if(window.lvTool==='text'){
				var t=prompt('Text:');
				if(t){ send_event('board','Text', bp(e)+','+t); }
				return;
			}
			drawing=true; svg.setPointerCapture(e.pointerId);
			send_event('board','Down', bp(e));
		});
		svg.addEventListener('pointermove', function(e){
			if(!ready())return;
			var now=Date.now();
			if(panning){ pan(e.clientX-px, e.clientY-py); px=e.clientX; py=e.clientY; return; }
			if(drawing){
				if(now-lastMove>33){ lastMove=now; send_event('board','Move', bp(e)); }
			} else if(now-lastCursor>100){
				lastCursor=now; send_event('board','Cursor', bp(e));
			}
		});
		svg.addEventListener('pointerup', function(e){
			if(panning){ panning=false; return; }
			if(drawing&&ready()){ drawing=false; send_event('board','Up', bp(e)); }
		});
		svg.addEventListener('wheel', function(e){ e.preventDefault(); zoomAt(e.deltaY<0?0.9:1.1, e.clientX, e.clientY); }, {passive:false});
		document.addEventListener('keydown', function(e){
			if(!ready())return;
			var tag=document.activeElement&&document.activeElement.tagName;
			if(tag==='INPUT'||tag==='TEXTAREA'||tag==='SELECT')return;
			if(e.key==='Delete'||e.key==='Backspace'){ send_event('board','Delete',''); }
			if((e.ctrlKey||e.metaKey)&&e.key==='z'){ e.preventDefault(); send_event('board','Undo',''); }
		});
	}
	init();
})();
</script>`

const boardCss = `
html, body, #content { height: 100%; margin: 0; overflow: hidden; }
.board-app { position: relative; height: 100vh; background: #fafafa; }
.board-toolbar {
	position: absolute; top: 12px; left: 50%; transform: translateX(-50%);
	background: #fff; border: 1px solid var(--lv-border); border-radius: 10px;
	box-shadow: 0 6px 20px rgba(15,23,42,.12); padding: .4rem .5rem;
	display: flex; align-items: center; gap: .2rem; flex-wrap: wrap;
	max-width: 96vw; z-index: 10;
}
.tb-btn {
	border: 1px solid transparent; background: none; border-radius: 8px;
	min-width: 2rem; height: 2rem; cursor: pointer; font-size: 1rem;
	display: inline-flex; align-items: center; justify-content: center;
}
.tb-btn:hover { background: #f1f5f9; }
.tb-btn.tb-active { background: #e0e7ff; border-color: #818cf8; }
.tb-swatch {
	width: 1.4rem; height: 1.4rem; border-radius: 6px; cursor: pointer;
	border: 1px solid rgba(0,0,0,.15); padding: 0; margin: 0 1px;
}
.tb-swatch.tb-active { outline: 2px solid #4f46e5; outline-offset: 1px; }
.tb-sep { width: 1px; height: 1.5rem; background: var(--lv-border); margin: 0 .35rem; }
.tb-select { border: 1px solid var(--lv-border); border-radius: 8px; height: 2rem; padding: 0 .4rem; font-size: .85rem; }
#board-svg { width: 100%; height: 100%; touch-action: none; cursor: crosshair; display: block; }
.board-status {
	position: absolute; bottom: 10px; left: 12px; background: #fff;
	border: 1px solid var(--lv-border); border-radius: 8px; padding: .3rem .7rem;
	font-size: .8rem; color: var(--lv-text-muted); box-shadow: var(--lv-shadow); z-index: 10;
}
`

// ------------------------------------------------------------------ app --

func parseXY(data interface{}) (float64, float64, bool) {
	parts := strings.SplitN(fmt.Sprint(data), ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, err1 := strconv.ParseFloat(parts[0], 64)
	y, err2 := strconv.ParseFloat(parts[1], 64)
	return x, y, err1 == nil && err2 == nil
}

func pushUndo(s *session, op undoOp) {
	s.undo = append(s.undo, op)
	if len(s.undo) > 50 {
		s.undo = s.undo[1:]
	}
}

func main() {
	if err := openDB("board.db"); err != nil {
		log.Fatal(err)
	}

	app := fiber.New()
	home := view.PageControl{
		Title:     "Board — go-fiber-live-view",
		Path:      "/",
		Router:    app,
		Css:       boardCss,
		AfterCode: glueJS,
	}

	home.Register(func() view.LiveDriver {
		lid := "board-" + uuid.NewString()

		mu.Lock()
		userSeq++
		s := &session{
			lid:    lid,
			name:   fmt.Sprintf("user-%d", userSeq),
			color:  cursorColors[userSeq%len(cursorColors)],
			board:  "main",
			tool:   "pen",
			stroke: strokePalette[0],
			fill:   "transparent",
			width:  2,
		}
		sessions[lid] = s
		mu.Unlock()

		sink := view.NewWithTemplate("board", ``)

		var document *view.ComponentDriver[*view.Layout]

		fillEls := func(els, ov, st string) {
			document.GetDriverById("els_layer").FillValue(els)
			document.GetDriverById("overlay_layer").FillValue(ov)
			document.GetDriverById("statusbar").FillValue(st)
		}

		refreshAllLocked := func() (tb, els, ov, st string) {
			return renderToolbarLocked(s), renderElsLocked(getBoard(s.board)), renderOverlayLocked(s), statusLocked(s)
		}

		// broadcast helpers: "E|board" elements changed, "C|board" cursors
		// moved, "T|*" board list changed.
		bcast := func(kind, board string) {
			view.SendToAllLayouts(kind + "|" + board)
		}

		sink.SetEvent("Tool", func(_ *view.None, data interface{}) {
			t := fmt.Sprint(data)
			if !validTool(t) {
				return
			}
			mu.Lock()
			s.tool = t
			if t != "select" {
				s.selected = 0
			}
			tb := renderToolbarLocked(s)
			ov := renderOverlayLocked(s)
			mu.Unlock()
			document.GetDriverById("toolbar").FillValue(tb)
			document.GetDriverById("overlay_layer").FillValue(ov)
			document.EvalScript("window.lvTool='" + t + "'")
		})

		applyStyle := func(apply func()) {
			mu.Lock()
			apply()
			var el *Element
			if s.selected != 0 {
				el, _ = getBoard(s.board).find(s.selected)
			}
			if el != nil {
				pushUndo(s, undoOp{kind: "mut", el: copyEl(*el)})
				el.Stroke, el.Fill, el.StrokeW = s.stroke, s.fill, s.width
				dbUpdate(el)
			}
			tb := renderToolbarLocked(s)
			board := s.board
			mu.Unlock()
			document.GetDriverById("toolbar").FillValue(tb)
			if el != nil {
				bcast("E", board)
			}
		}
		sink.SetEvent("Stroke", func(_ *view.None, data interface{}) {
			c := fmt.Sprint(data)
			if inPalette(strokePalette, c) {
				applyStyle(func() { s.stroke = c })
			}
		})
		sink.SetEvent("Fill", func(_ *view.None, data interface{}) {
			c := fmt.Sprint(data)
			if inPalette(fillPalette, c) {
				applyStyle(func() { s.fill = c })
			}
		})
		sink.SetEvent("Width", func(_ *view.None, data interface{}) {
			w, err := strconv.ParseFloat(fmt.Sprint(data), 64)
			if err == nil && (w == 2 || w == 4 || w == 6) {
				applyStyle(func() { s.width = w })
			}
		})

		sink.SetEvent("Down", func(_ *view.None, data interface{}) {
			x, y, ok := parseXY(data)
			if !ok {
				return
			}
			mu.Lock()
			b := getBoard(s.board)
			s.prevX, s.prevY = x, y
			s.curX, s.curY = x, y
			board := s.board
			changed := false
			switch s.tool {
			case "select":
				if hit := hitTestLocked(b, x, y); hit != nil {
					s.selected = hit.ID
					s.mode = "move"
					s.origEl = copyEl(*hit)
				} else {
					s.selected = 0
					s.mode = ""
				}
			case "hand", "text":
				// handled client-side
			case "eraser":
				s.mode = "erase"
				if hit := hitTestLocked(b, x, y); hit != nil {
					pushUndo(s, undoOp{kind: "del", el: copyEl(*hit)})
					b.remove(hit.ID)
					dbDelete(hit.ID)
					changed = true
				}
			default: // pen and shapes
				el := &Element{Kind: s.tool, X: x, Y: y, Stroke: s.stroke, Fill: s.fill, StrokeW: s.width}
				if s.tool == "pen" {
					el.Points = []Pt{{x, y}}
				}
				dbInsert(board, el)
				b.Els = append(b.Els, el)
				s.cur = el
				s.mode = "draw"
				changed = true
			}
			ov := renderOverlayLocked(s)
			mu.Unlock()
			document.GetDriverById("overlay_layer").FillValue(ov)
			if changed {
				bcast("E", board)
			}
		})

		sink.SetEvent("Move", func(_ *view.None, data interface{}) {
			x, y, ok := parseXY(data)
			if !ok {
				return
			}
			mu.Lock()
			b := getBoard(s.board)
			dx, dy := x-s.prevX, y-s.prevY
			s.prevX, s.prevY = x, y
			s.curX, s.curY = x, y
			s.hasCursor = true
			board := s.board
			kind := "C"
			switch s.mode {
			case "draw":
				if s.cur != nil {
					if s.cur.Kind == "pen" {
						last := s.cur.Points[len(s.cur.Points)-1]
						if math.Hypot(x-last.X, y-last.Y) > 1.5 {
							s.cur.Points = append(s.cur.Points, Pt{x, y})
						}
					} else {
						s.cur.W = x - s.cur.X
						s.cur.H = y - s.cur.Y
					}
					kind = "E"
				}
			case "move":
				if el, _ := b.find(s.selected); el != nil {
					el.X += dx
					el.Y += dy
					for i := range el.Points {
						el.Points[i].X += dx
						el.Points[i].Y += dy
					}
					kind = "E"
				}
			case "erase":
				if hit := hitTestLocked(b, x, y); hit != nil {
					pushUndo(s, undoOp{kind: "del", el: copyEl(*hit)})
					b.remove(hit.ID)
					dbDelete(hit.ID)
					kind = "E"
				}
			}
			mu.Unlock()
			bcast(kind, board)
		})

		sink.SetEvent("Up", func(_ *view.None, data interface{}) {
			mu.Lock()
			b := getBoard(s.board)
			board := s.board
			changed := false
			switch s.mode {
			case "draw":
				if el := s.cur; el != nil {
					// a click without drag produces a degenerate shape: drop it
					if el.Kind != "pen" && math.Abs(el.W) < 3 && math.Abs(el.H) < 3 {
						b.remove(el.ID)
						dbDelete(el.ID)
					} else {
						dbUpdate(el)
						pushUndo(s, undoOp{kind: "add", el: copyEl(*el)})
					}
					changed = true
				}
				s.cur = nil
			case "move":
				if el, _ := b.find(s.selected); el != nil {
					dbUpdate(el)
					pushUndo(s, undoOp{kind: "mut", el: s.origEl})
					changed = true
				}
			}
			s.mode = ""
			ov := renderOverlayLocked(s)
			mu.Unlock()
			document.GetDriverById("overlay_layer").FillValue(ov)
			if changed {
				bcast("E", board)
			}
		})

		sink.SetEvent("Cursor", func(_ *view.None, data interface{}) {
			x, y, ok := parseXY(data)
			if !ok {
				return
			}
			mu.Lock()
			s.curX, s.curY = x, y
			s.hasCursor = true
			board := s.board
			mu.Unlock()
			bcast("C", board)
		})

		sink.SetEvent("Text", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), ",", 3)
			if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
				return
			}
			x, _ := strconv.ParseFloat(parts[0], 64)
			y, _ := strconv.ParseFloat(parts[1], 64)
			text := parts[2]
			if len(text) > 300 {
				text = text[:300]
			}
			mu.Lock()
			b := getBoard(s.board)
			board := s.board
			el := &Element{Kind: "text", X: x, Y: y, Stroke: s.stroke, StrokeW: s.width, Text: text}
			dbInsert(board, el)
			b.Els = append(b.Els, el)
			pushUndo(s, undoOp{kind: "add", el: copyEl(*el)})
			mu.Unlock()
			bcast("E", board)
		})

		sink.SetEvent("Delete", func(_ *view.None, data interface{}) {
			mu.Lock()
			b := getBoard(s.board)
			board := s.board
			changed := false
			if el, _ := b.find(s.selected); el != nil {
				pushUndo(s, undoOp{kind: "del", el: copyEl(*el)})
				b.remove(el.ID)
				dbDelete(el.ID)
				s.selected = 0
				changed = true
			}
			ov := renderOverlayLocked(s)
			mu.Unlock()
			document.GetDriverById("overlay_layer").FillValue(ov)
			if changed {
				bcast("E", board)
			}
		})

		sink.SetEvent("Undo", func(_ *view.None, data interface{}) {
			mu.Lock()
			b := getBoard(s.board)
			board := s.board
			if len(s.undo) == 0 {
				mu.Unlock()
				return
			}
			op := s.undo[len(s.undo)-1]
			s.undo = s.undo[:len(s.undo)-1]
			switch op.kind {
			case "add":
				b.remove(op.el.ID)
				dbDelete(op.el.ID)
				if s.selected == op.el.ID {
					s.selected = 0
				}
			case "del":
				el := copyEl(op.el)
				b.Els = append(b.Els, &el)
				dbInsertWithID(board, &el)
			case "mut":
				if el, _ := b.find(op.el.ID); el != nil {
					*el = copyEl(op.el)
					dbUpdate(el)
				}
			}
			ov := renderOverlayLocked(s)
			mu.Unlock()
			document.GetDriverById("overlay_layer").FillValue(ov)
			bcast("E", board)
		})

		sink.SetEvent("Clear", func(_ *view.None, data interface{}) {
			mu.Lock()
			b := getBoard(s.board)
			board := s.board
			b.Els = nil
			dbClear(board)
			s.selected = 0
			s.undo = nil
			mu.Unlock()
			bcast("E", board)
		})

		switchBoard := func(name string) {
			mu.Lock()
			s.board = name
			s.selected = 0
			s.mode = ""
			s.undo = nil
			tb, els, ov, st := refreshAllLocked()
			mu.Unlock()
			document.GetDriverById("toolbar").FillValue(tb)
			fillEls(els, ov, st)
			bcast("C", "*")
		}

		sink.SetEvent("Board", func(_ *view.None, data interface{}) {
			name := fmt.Sprint(data)
			if !validBoard.MatchString(name) {
				return
			}
			mu.Lock()
			exists := inPalette(dbListBoards(), name)
			mu.Unlock()
			if exists {
				switchBoard(name)
			}
		})

		sink.SetEvent("NewBoard", func(_ *view.None, data interface{}) {
			name := strings.TrimSpace(fmt.Sprint(data))
			if !validBoard.MatchString(name) {
				return
			}
			mu.Lock()
			dbCreateBoard(name)
			mu.Unlock()
			switchBoard(name)
			bcast("T", "*")
		})

		document = view.NewLayout(lid, `
		<div class="board-app">
			<div id="toolbar" class="board-toolbar"></div>
			<svg id="board-svg" viewBox="0 0 1600 1000" xmlns="http://www.w3.org/2000/svg">
				<defs>
					<pattern id="dots" width="24" height="24" patternUnits="userSpaceOnUse">
						<circle cx="1.5" cy="1.5" r="1.2" fill="#d4d4d8"/>
					</pattern>
				</defs>
				<rect x="-20000" y="-20000" width="40000" height="40000" fill="url(#dots)"/>
				<g id="els_layer"></g>
				<g id="overlay_layer"></g>
			</svg>
			<div id="statusbar" class="board-status"></div>
			<span style="display:none">{{mount "board"}}</span>
		</div>`)

		document.Component.SetHandlerEventIn(func(data interface{}) {
			msg := fmt.Sprint(data)
			parts := strings.SplitN(msg, "|", 2)
			mu.Lock()
			if parts[0] == "FIRST_TIME" || len(parts) == 1 {
				tb, els, ov, st := refreshAllLocked()
				tool := s.tool
				mu.Unlock()
				document.GetDriverById("toolbar").FillValue(tb)
				fillEls(els, ov, st)
				document.EvalScript("window.lvTool='" + tool + "'")
				return
			}
			kind, board := parts[0], parts[1]
			if board != "*" && board != s.board {
				mu.Unlock()
				return
			}
			switch kind {
			case "E":
				els := renderElsLocked(getBoard(s.board))
				ov := renderOverlayLocked(s)
				st := statusLocked(s)
				mu.Unlock()
				fillEls(els, ov, st)
			case "C":
				ov := renderOverlayLocked(s)
				st := statusLocked(s)
				mu.Unlock()
				document.GetDriverById("overlay_layer").FillValue(ov)
				document.GetDriverById("statusbar").FillValue(st)
			case "T":
				tb := renderToolbarLocked(s)
				mu.Unlock()
				document.GetDriverById("toolbar").FillValue(tb)
			default:
				mu.Unlock()
			}
		})

		document.Component.SetHandlerEventDestroy(func(id string) {
			mu.Lock()
			board := ""
			if o := sessions[lid]; o != nil {
				board = o.board
			}
			delete(sessions, lid)
			mu.Unlock()
			if board != "" {
				view.SendToAllLayouts("C|" + board)
			}
		})

		return document
	})

	fmt.Println("Board example -> http://localhost:3007")
	log.Fatal(app.Listen(":3007"))
}
