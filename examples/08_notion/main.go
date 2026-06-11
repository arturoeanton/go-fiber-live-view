// Notion-style collaborative workspace, rendered from the server.
//
//   - Sidebar with nested pages (create, rename, delete, fold, emoji icons)
//   - Block editor: text, headings, todos, lists, quotes, code, callouts,
//     dividers, images by URL and tables — switch types with the "/" menu,
//     the block menu (⋯) or Notion's markdown shortcuts ("# ", "- ", "[] "…)
//   - Enter splits into a new block, Backspace on an empty block removes it,
//     drag blocks with the ⋮⋮ handle
//   - Block-level live collaboration (like Notion): everyone sees commits in
//     real time, each block shows who is editing it, presence avatars on top
//   - Everything persisted in SQLite (notion.db, pure-Go driver)
package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type session struct {
	lid         string
	name, color string
	page        int64
	editing     int64
	editCell    string
	menuFor     int64
	iconMenu    bool
	collapsed   map[int64]bool
}

var (
	mu       sync.Mutex
	sessions = map[string]*session{}
	userSeq  int

	userColors = []string{"#e03131", "#1971c2", "#2f9e44", "#f08c00", "#9c36b5", "#0c8599"}
)

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// firstRootLocked returns (creating if needed) the workspace's first page.
func firstRootLocked() int64 {
	roots := childPages(0)
	if len(roots) == 0 {
		p := dbAddPage(0, "Sin título")
		return p.ID
	}
	return roots[0].ID
}

var slashCommands = map[string]string{
	"/texto": "p", "/h1": "h1", "/h2": "h2", "/h3": "h3", "/todo": "todo",
	"/lista": "bullet", "/numerada": "number", "/cita": "quote",
	"/codigo": "code", "/código": "code", "/callout": "callout",
	"/divisor": "divider", "/imagen": "image", "/tabla": "table",
}

var mdShortcuts = []struct{ prefix, typ string }{
	{"### ", "h3"}, {"## ", "h2"}, {"# ", "h1"},
	{"[] ", "todo"}, {"[ ] ", "todo"},
	{"- ", "bullet"}, {"* ", "bullet"}, {"1. ", "number"},
	{"> ", "quote"}, {"```", "code"},
}

// applyShortcuts interprets slash commands and Notion markdown prefixes,
// possibly changing the block type, and returns the remaining content.
func applyShortcuts(b *Block, content string) string {
	if b.Type == "code" || b.Type == "image" {
		return content
	}
	if t, ok := slashCommands[strings.ToLower(strings.TrimSpace(content))]; ok {
		b.Type = t
		if t == "table" {
			return tableJSON([][]string{{"", ""}, {"", ""}})
		}
		return ""
	}
	for _, sc := range mdShortcuts {
		if strings.HasPrefix(content, sc.prefix) {
			b.Type = sc.typ
			return strings.TrimPrefix(content, sc.prefix)
		}
	}
	return content
}

func main() {
	if err := openDB("notion.db"); err != nil {
		log.Fatal(err)
	}

	app := fiber.New()
	home := view.PageControl{
		Title:     "GoNotion — go-fiber-live-view",
		Path:      "/",
		Router:    app,
		Css:       notionCss,
		AfterCode: glueJS,
	}

	home.Register(func() view.LiveDriver {
		lid := "notion-" + uuid.NewString()

		mu.Lock()
		userSeq++
		s := &session{
			lid:       lid,
			name:      fmt.Sprintf("user-%d", userSeq),
			color:     userColors[userSeq%len(userColors)],
			page:      firstRootLocked(),
			collapsed: map[int64]bool{},
		}
		sessions[lid] = s
		mu.Unlock()

		sbSink := view.NewWithTemplate("sb", ``)
		edSink := view.NewWithTemplate("ed", ``)

		var document *view.ComponentDriver[*view.Layout]

		fill := func(id, content string) { document.GetDriverById(id).FillValue(content) }

		fillAllLocked := func() (sidebar, topbar, head, editor string) {
			return renderSidebarLocked(s), renderTopbarLocked(s), renderPageHeadLocked(s), renderEditorLocked(s)
		}
		refreshAll := func() {
			mu.Lock()
			sb, tb, hd, ed := fillAllLocked()
			mu.Unlock()
			fill("sidebar_box", sb)
			fill("topbar_box", tb)
			fill("page_head", hd)
			fill("editor_box", ed)
		}
		refreshEditor := func() {
			mu.Lock()
			ed := renderEditorLocked(s)
			mu.Unlock()
			fill("editor_box", ed)
		}
		bcast := func(kind string, arg interface{}) {
			view.SendToAllLayouts(fmt.Sprintf("%s|%v", kind, arg))
		}

		// ------------------------------------------------------ sidebar --

		sbSink.SetEvent("Open", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			if pages[id] == nil {
				mu.Unlock()
				return
			}
			s.page = id
			s.editing, s.menuFor, s.editCell, s.iconMenu = 0, 0, "", false
			mu.Unlock()
			refreshAll()
			bcast("V", "*")
		})

		sbSink.SetEvent("Fold", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			s.collapsed[id] = !s.collapsed[id]
			sb := renderSidebarLocked(s)
			mu.Unlock()
			fill("sidebar_box", sb)
		})

		sbSink.SetEvent("Add", func(_ *view.None, data interface{}) {
			parent := atoi64(fmt.Sprint(data))
			mu.Lock()
			if parent != 0 && pages[parent] == nil {
				mu.Unlock()
				return
			}
			p := dbAddPage(parent, "Sin título")
			s.collapsed[parent] = false
			s.page = p.ID
			s.editing, s.menuFor, s.editCell = 0, 0, ""
			mu.Unlock()
			refreshAll()
			bcast("T", "*")
		})

		sbSink.SetEvent("Del", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			if pages[id] == nil {
				mu.Unlock()
				return
			}
			dbDeletePage(id)
			if pages[s.page] == nil {
				s.page = firstRootLocked()
				s.editing, s.menuFor, s.editCell = 0, 0, ""
			}
			mu.Unlock()
			refreshAll()
			bcast("T", "*")
		})

		// ------------------------------------------------------- editor --

		edSink.SetEvent("Edit", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			b, _ := findBlock(s.page, id)
			if b == nil || b.Type == "table" {
				mu.Unlock()
				return
			}
			s.editing, s.editCell, s.menuFor = id, "", 0
			mu.Unlock()
			refreshEditor()
			bcast("V", s.page) // editing badges live in other viewers' editors
			bcast("P", s.page)
		})

		edSink.SetEvent("Commit", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), "|", 3)
			if len(parts) != 3 {
				return
			}
			id, enter, content := atoi64(parts[0]), parts[1] == "1", parts[2]
			mu.Lock()
			b, idx := findBlock(s.page, id)
			if b == nil || b.Type == "table" {
				mu.Unlock()
				return
			}
			b.Content = applyShortcuts(b, content)
			if len(b.Content) > 10000 {
				b.Content = b.Content[:10000]
			}
			dbUpdateBlock(b)
			s.menuFor = 0
			if enter {
				newType := "p"
				if (b.Type == "bullet" || b.Type == "number" || b.Type == "todo") && strings.TrimSpace(b.Content) != "" {
					newType = b.Type
				}
				nb := dbAddBlock(s.page, newType, "", posAfter(s.page, idx))
				s.editing = nb.ID
			} else {
				s.editing = 0
			}
			s.editCell = ""
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("DelMerge", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			b, idx := findBlock(s.page, id)
			if b == nil {
				mu.Unlock()
				return
			}
			dbDeleteBlock(s.page, id)
			s.editing = 0
			if idx > 0 {
				bs := getBlocks(s.page)
				prev := bs[idx-1]
				if prev.Type != "table" && prev.Type != "divider" {
					s.editing = prev.ID
				}
			}
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("Toggle", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			b, _ := findBlock(s.page, id)
			if b == nil {
				mu.Unlock()
				return
			}
			b.Checked = !b.Checked
			dbUpdateBlock(b)
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("Menu", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			if s.menuFor == id {
				s.menuFor = 0
			} else {
				s.menuFor = id
			}
			mu.Unlock()
			refreshEditor()
		})

		edSink.SetEvent("SetType", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), "|", 2)
			if len(parts) != 2 || !validType(parts[1]) {
				return
			}
			id, typ := atoi64(parts[0]), parts[1]
			mu.Lock()
			b, _ := findBlock(s.page, id)
			if b == nil {
				mu.Unlock()
				return
			}
			b.Type = typ
			switch typ {
			case "table":
				b.Content = tableJSON(parseTable(b.Content))
				s.editing = 0
			case "divider":
				b.Content = ""
				s.editing = 0
			default:
				s.editing = id
			}
			dbUpdateBlock(b)
			s.menuFor = 0
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("DelBlock", func(_ *view.None, data interface{}) {
			id := atoi64(fmt.Sprint(data))
			mu.Lock()
			if b, _ := findBlock(s.page, id); b != nil {
				dbDeleteBlock(s.page, id)
			}
			if s.editing == id {
				s.editing = 0
			}
			s.menuFor = 0
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("AddEnd", func(_ *view.None, data interface{}) {
			mu.Lock()
			nb := dbAddBlock(s.page, "p", "", posAfter(s.page, len(getBlocks(s.page))-1))
			s.editing = nb.ID
			s.menuFor = 0
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("MoveBlk", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), "|", 3)
			if len(parts) != 3 {
				return
			}
			mu.Lock()
			src, _ := findBlock(s.page, atoi64(parts[0]))
			dst, di := findBlock(s.page, atoi64(parts[1]))
			if src == nil || dst == nil || src.ID == dst.ID {
				mu.Unlock()
				return
			}
			bs := getBlocks(s.page)
			if parts[2] == "before" {
				if di == 0 {
					src.Pos = dst.Pos - 1
				} else {
					src.Pos = (bs[di-1].Pos + dst.Pos) / 2
				}
			} else {
				if di >= len(bs)-1 {
					src.Pos = dst.Pos + 1
				} else {
					src.Pos = (dst.Pos + bs[di+1].Pos) / 2
				}
			}
			dbUpdateBlock(src)
			sortBlocks(s.page)
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		// table cells
		edSink.SetEvent("EditCell", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), "|", 3)
			if len(parts) != 3 {
				return
			}
			mu.Lock()
			if b, _ := findBlock(s.page, atoi64(parts[0])); b != nil && b.Type == "table" {
				s.editing = b.ID
				s.editCell = parts[1] + "," + parts[2]
				s.menuFor = 0
			}
			mu.Unlock()
			refreshEditor()
		})

		edSink.SetEvent("CommitCell", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), "|", 3)
			if len(parts) != 3 {
				return
			}
			cell := strings.SplitN(parts[1], ",", 2)
			if len(cell) != 2 {
				return
			}
			r, c := int(atoi64(cell[0])), int(atoi64(cell[1]))
			mu.Lock()
			b, _ := findBlock(s.page, atoi64(parts[0]))
			if b == nil || b.Type != "table" {
				mu.Unlock()
				return
			}
			rows := parseTable(b.Content)
			if r >= 0 && r < len(rows) && c >= 0 && c < len(rows[r]) {
				rows[r][c] = parts[2]
				b.Content = tableJSON(rows)
				dbUpdateBlock(b)
			}
			s.editing, s.editCell = 0, ""
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		edSink.SetEvent("TableOp", func(_ *view.None, data interface{}) {
			parts := strings.SplitN(fmt.Sprint(data), "|", 2)
			if len(parts) != 2 {
				return
			}
			mu.Lock()
			b, _ := findBlock(s.page, atoi64(parts[0]))
			if b == nil || b.Type != "table" {
				mu.Unlock()
				return
			}
			rows := parseTable(b.Content)
			cols := len(rows[0])
			switch parts[1] {
			case "addrow":
				rows = append(rows, make([]string, cols))
			case "addcol":
				for i := range rows {
					rows[i] = append(rows[i], "")
				}
			case "delrow":
				if len(rows) > 1 {
					rows = rows[:len(rows)-1]
				}
			case "delcol":
				if cols > 1 {
					for i := range rows {
						rows[i] = rows[i][:len(rows[i])-1]
					}
				}
			}
			b.Content = tableJSON(rows)
			dbUpdateBlock(b)
			page := s.page
			mu.Unlock()
			bcast("P", page)
		})

		// page head
		edSink.SetEvent("Title", func(_ *view.None, data interface{}) {
			title := strings.TrimSpace(fmt.Sprint(data))
			if len(title) > 120 {
				title = title[:120]
			}
			mu.Lock()
			if p := pages[s.page]; p != nil {
				p.Title = title
				dbUpdatePage(p)
			}
			mu.Unlock()
			bcast("T", "*")
			bcast("P", s.page)
		})

		edSink.SetEvent("IconMenu", func(_ *view.None, data interface{}) {
			mu.Lock()
			s.iconMenu = !s.iconMenu
			hd := renderPageHeadLocked(s)
			mu.Unlock()
			fill("page_head", hd)
		})

		edSink.SetEvent("Icon", func(_ *view.None, data interface{}) {
			icon := fmt.Sprint(data)
			ok := false
			for _, ic := range iconPalette {
				if ic == icon {
					ok = true
				}
			}
			if !ok {
				return
			}
			mu.Lock()
			if p := pages[s.page]; p != nil {
				p.Icon = icon
				dbUpdatePage(p)
			}
			s.iconMenu = false
			mu.Unlock()
			bcast("T", "*")
			bcast("P", s.page)
		})

		// -------------------------------------------------- layout & in --

		document = view.NewLayout(lid, `
		<div class="nt-app">
			<div class="nt-side">
				<div class="nt-logo">🪶 GoNotion <span class="lv-muted" style="font-weight:400;font-size:.75rem">live</span></div>
				<div class="nt-tree" id="sidebar_box"></div>
				<button class="nt-newpage" onclick="send_event('sb','Add','0')">＋ Nueva página</button>
			</div>
			<div class="nt-main">
				<div class="nt-topbar" id="topbar_box"></div>
				<div class="nt-doc"><div class="nt-page">
					<div id="page_head"></div>
					<div id="editor_box"></div>
				</div></div>
			</div>
			<span style="display:none">{{mount "sb"}}{{mount "ed"}}</span>
		</div>`)

		document.Component.SetHandlerEventIn(func(data interface{}) {
			msg := fmt.Sprint(data)
			parts := strings.SplitN(msg, "|", 2)
			if len(parts) != 2 { // FIRST_TIME and friends: full paint
				refreshAll()
				return
			}
			kind, arg := parts[0], parts[1]
			mu.Lock()
			if pages[s.page] == nil {
				s.page = firstRootLocked()
				s.editing, s.menuFor, s.editCell = 0, 0, ""
				sb, tb, hd, ed := fillAllLocked()
				mu.Unlock()
				fill("sidebar_box", sb)
				fill("topbar_box", tb)
				fill("page_head", hd)
				fill("editor_box", ed)
				return
			}
			mine := arg == "*" || atoi64(arg) == s.page
			switch kind {
			case "T":
				sb := renderSidebarLocked(s)
				tb := renderTopbarLocked(s)
				hd := renderPageHeadLocked(s)
				mu.Unlock()
				fill("sidebar_box", sb)
				fill("topbar_box", tb)
				fill("page_head", hd)
			case "P":
				if !mine {
					mu.Unlock()
					return
				}
				ed := renderEditorLocked(s)
				hd := renderPageHeadLocked(s)
				mu.Unlock()
				fill("editor_box", ed)
				fill("page_head", hd)
			case "V":
				if !mine {
					mu.Unlock()
					return
				}
				tb := renderTopbarLocked(s)
				mu.Unlock()
				fill("topbar_box", tb)
			default:
				mu.Unlock()
			}
		})

		document.Component.SetHandlerEventDestroy(func(id string) {
			mu.Lock()
			page := int64(0)
			if o := sessions[lid]; o != nil {
				page = o.page
			}
			delete(sessions, lid)
			mu.Unlock()
			view.SendToAllLayouts(fmt.Sprintf("V|%d", page))
		})

		return document
	})

	fmt.Println("GoNotion example -> http://localhost:3008")
	log.Fatal(app.Listen(":3008"))
}
