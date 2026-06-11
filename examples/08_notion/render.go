package main

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// ----------------------------------------------------- inline markdown --

var (
	reLink   = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)
	reCode   = regexp.MustCompile("`([^`]+)`")
	reBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic = regexp.MustCompile(`\*([^*]+)\*`)
	reStrike = regexp.MustCompile(`~~([^~]+)~~`)
)

func esc(s string) string { return html.EscapeString(s) }

// inline renders Notion-style inline markdown over escaped text.
func inline(s string) string {
	s = esc(s)
	s = reLink.ReplaceAllString(s, `<a href="$2" target="_blank">$1</a>`)
	s = reCode.ReplaceAllString(s, `<code>$1</code>`)
	s = reBold.ReplaceAllString(s, `<b>$1</b>`)
	s = reItalic.ReplaceAllString(s, `<i>$1</i>`)
	s = reStrike.ReplaceAllString(s, `<s>$1</s>`)
	return s
}

// ------------------------------------------------------------- sidebar --

func renderSidebarLocked(s *session) string {
	var sb strings.Builder
	var walk func(parent int64, depth int)
	walk = func(parent int64, depth int) {
		for _, p := range childPages(parent) {
			kids := childPages(p.ID)
			caret := `<span class="sb-caret">·</span>`
			if len(kids) > 0 {
				sym := "▾"
				if s.collapsed[p.ID] {
					sym = "▸"
				}
				caret = fmt.Sprintf(`<span class="sb-caret" onclick="event.stopPropagation();send_event('sb','Fold','%d')">%s</span>`, p.ID, sym)
			}
			cls := "sb-row"
			if p.ID == s.page {
				cls += " sb-active"
			}
			fmt.Fprintf(&sb, `<div class="%s" style="padding-left:%dpx" onclick="send_event('sb','Open','%d')">%s<span class="sb-ico">%s</span><span class="sb-title">%s</span><span class="sb-act" title="Subpágina" onclick="event.stopPropagation();send_event('sb','Add','%d')">＋</span><span class="sb-act" title="Borrar" onclick="event.stopPropagation();if(confirm('¿Borrar página y subpáginas?'))send_event('sb','Del','%d')">🗑</span></div>`,
				cls, 6+depth*14, p.ID, caret, p.Icon, esc(p.Title), p.ID, p.ID)
			if !s.collapsed[p.ID] {
				walk(p.ID, depth+1)
			}
		}
	}
	walk(0, 0)
	return sb.String()
}

// ------------------------------------------------------------- topbar --

func renderTopbarLocked(s *session) string {
	var sb strings.Builder
	sb.WriteString(`<div class="nt-crumbs">`)
	var chain []*Page
	for p := pages[s.page]; p != nil; p = pages[p.Parent] {
		chain = append([]*Page{p}, chain...)
	}
	for i, p := range chain {
		if i > 0 {
			sb.WriteString(`<span class="nt-crumb-sep">/</span>`)
		}
		fmt.Fprintf(&sb, `<span class="nt-crumb" onclick="send_event('sb','Open','%d')">%s %s</span>`, p.ID, p.Icon, esc(p.Title))
	}
	sb.WriteString(`</div><div class="nt-presence">`)
	for _, o := range sessions {
		if o.page != s.page {
			continue
		}
		initial := strings.ToUpper(o.name[len(o.name)-1:])
		fmt.Fprintf(&sb, `<span class="nt-avatar" style="background:%s" title="%s">%s</span>`, o.color, esc(o.name), initial)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

var iconPalette = []string{"📄", "👋", "⚙️", "📚", "💡", "🚀", "🎯", "📊", "🗂", "✅", "🧠", "🔥", "🐹", "💬", "📝", "⭐"}

func renderPageHeadLocked(s *session) string {
	p := pages[s.page]
	if p == nil {
		return `<p class="lv-muted">Esta página fue eliminada.</p>`
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="nt-icon" title="Cambiar icono" onclick="send_event('ed','IconMenu','')">%s</div>`, p.Icon)
	if s.iconMenu {
		sb.WriteString(`<div class="nt-iconmenu">`)
		for _, ic := range iconPalette {
			fmt.Fprintf(&sb, `<span onclick="send_event('ed','Icon','%s')">%s</span>`, ic, ic)
		}
		sb.WriteString(`</div>`)
	}
	title := esc(p.Title)
	fmt.Fprintf(&sb, `<input id="title_input" class="nt-title" value="%s" placeholder="Sin título" onchange="send_event('ed','Title',this.value)"/>`, title)
	return sb.String()
}

// -------------------------------------------------------------- editor --

var blockTypes = []struct{ code, label string }{
	{"p", "Texto"}, {"h1", "Título 1"}, {"h2", "Título 2"}, {"h3", "Título 3"},
	{"todo", "Lista de tareas"}, {"bullet", "Lista"}, {"number", "Lista numerada"},
	{"quote", "Cita"}, {"code", "Código"}, {"callout", "Callout"},
	{"divider", "Divisor"}, {"image", "Imagen"}, {"table", "Tabla"},
}

func validType(t string) bool {
	for _, bt := range blockTypes {
		if bt.code == t {
			return true
		}
	}
	return false
}

func parseTable(content string) [][]string {
	var rows [][]string
	if json.Unmarshal([]byte(content), &rows) != nil || len(rows) == 0 {
		rows = [][]string{{"", ""}, {"", ""}}
	}
	return rows
}

func tableJSON(rows [][]string) string {
	b, _ := json.Marshal(rows)
	return string(b)
}

func renderTableLocked(s *session, b *Block) string {
	rows := parseTable(b.Content)
	var sb strings.Builder
	sb.WriteString(`<table class="nt-table">`)
	for r, row := range rows {
		sb.WriteString("<tr>")
		for c, cell := range row {
			tag := "td"
			if r == 0 {
				tag = "th"
			}
			if s.editing == b.ID && s.editCell == fmt.Sprintf("%d,%d", r, c) {
				fmt.Fprintf(&sb, `<%s><input id="blk_edit" data-kind="cell" data-bid="%d" data-cell="%d,%d" class="nt-cellinput" value="%s" oninput="lvDraftSave(this)"/></%s>`,
					tag, b.ID, r, c, esc(cell), tag)
			} else {
				fmt.Fprintf(&sb, `<%s onclick="event.stopPropagation();send_event('ed','EditCell','%d|%d|%d')">%s</%s>`,
					tag, b.ID, r, c, inline(cell), tag)
			}
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString(`</table>`)
	fmt.Fprintf(&sb, `<div class="nt-tablebar">
		<span onclick="event.stopPropagation();send_event('ed','TableOp','%d|addrow')">＋ fila</span>
		<span onclick="event.stopPropagation();send_event('ed','TableOp','%d|addcol')">＋ columna</span>
		<span onclick="event.stopPropagation();send_event('ed','TableOp','%d|delrow')">− fila</span>
		<span onclick="event.stopPropagation();send_event('ed','TableOp','%d|delcol')">− columna</span>
	</div>`, b.ID, b.ID, b.ID, b.ID)
	return sb.String()
}

func renderBlockViewLocked(s *session, b *Block, num int) string {
	switch b.Type {
	case "h1":
		return `<h1>` + inline(b.Content) + `</h1>`
	case "h2":
		return `<h2>` + inline(b.Content) + `</h2>`
	case "h3":
		return `<h3>` + inline(b.Content) + `</h3>`
	case "todo":
		cls := ""
		if b.Checked {
			cls = " nt-done"
		}
		chk := ""
		if b.Checked {
			chk = "checked"
		}
		return fmt.Sprintf(`<div class="nt-todo"><input type="checkbox" %s onclick="event.stopPropagation();send_event('ed','Toggle','%d')"/><span class="%s">%s</span></div>`,
			chk, b.ID, cls, inline(b.Content))
	case "bullet":
		return `<div class="nt-li"><span class="nt-marker">•</span><span>` + inline(b.Content) + `</span></div>`
	case "number":
		return fmt.Sprintf(`<div class="nt-li"><span class="nt-marker">%d.</span><span>%s</span></div>`, num, inline(b.Content))
	case "quote":
		return `<blockquote>` + inline(b.Content) + `</blockquote>`
	case "code":
		return `<pre class="nt-code"><code>` + esc(b.Content) + `</code></pre>`
	case "callout":
		return `<div class="nt-callout">` + inline(b.Content) + `</div>`
	case "divider":
		return `<hr class="nt-hr"/>`
	case "image":
		url := strings.TrimSpace(b.Content)
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			return fmt.Sprintf(`<img class="nt-img" src="%s" alt=""/>`, esc(url))
		}
		return `<div class="nt-imgempty">🖼 Click y pegá la URL de una imagen</div>`
	case "table":
		return renderTableLocked(s, b)
	}
	if strings.TrimSpace(b.Content) == "" {
		return `<p class="nt-p nt-empty">Escribí algo, o "/" para comandos…</p>`
	}
	return `<p class="nt-p">` + inline(b.Content) + `</p>`
}

func renderBlockEditLocked(b *Block) string {
	rows := 1
	if b.Type == "code" {
		rows = strings.Count(b.Content, "\n") + 2
	}
	placeholder := `Escribí algo, o &quot;/&quot; para comandos…`
	if b.Type == "image" {
		placeholder = "https://…  (URL de la imagen)"
	}
	return fmt.Sprintf(`<textarea id="blk_edit" data-kind="block" data-btype="%s" data-bid="%d" class="nt-input nt-input-%s" rows="%d" placeholder="%s" oninput="lvDraftSave(this)">%s</textarea>`,
		b.Type, b.ID, b.Type, rows, placeholder, esc(b.Content))
}

func renderTypeMenu(id int64) string {
	var sb strings.Builder
	sb.WriteString(`<div class="nt-menu">`)
	for _, bt := range blockTypes {
		fmt.Fprintf(&sb, `<div class="nt-menu-item" onclick="event.stopPropagation();send_event('ed','SetType','%d|%s')">%s</div>`, id, bt.code, bt.label)
	}
	fmt.Fprintf(&sb, `<div class="nt-menu-item nt-menu-del" onclick="event.stopPropagation();send_event('ed','DelBlock','%d')">🗑 Eliminar</div>`, id)
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderEditorLocked(s *session) string {
	if pages[s.page] == nil {
		return ""
	}
	var sb strings.Builder
	num := 0
	for _, b := range getBlocks(s.page) {
		if b.Type == "number" {
			num++
		} else {
			num = 0
		}
		var body string
		if s.editing == b.ID && b.Type != "table" {
			body = renderBlockEditLocked(b)
		} else {
			body = renderBlockViewLocked(s, b, num)
		}
		badge := ""
		for _, o := range sessions {
			if o.lid != s.lid && o.page == s.page && o.editing == b.ID {
				badge = fmt.Sprintf(`<span class="nt-editing" style="background:%s">✏️ %s</span>`, o.color, esc(o.name))
				break
			}
		}
		menu := ""
		if s.menuFor == b.ID {
			menu = renderTypeMenu(b.ID)
		}
		fmt.Fprintf(&sb, `<div class="nt-blk" data-bid="%d" ondragover="lvDragOver(event,this)" ondragleave="lvDragLeave(this)" ondrop="lvDrop(event,this)">
			<div class="nt-gut">
				<span class="nt-plus" title="Menú de bloque" onclick="event.stopPropagation();send_event('ed','Menu','%d')">⋯</span>
				<span class="nt-handle" title="Arrastrar" draggable="true" ondragstart="lvDragStart(event,'%d')">⋮⋮</span>
			</div>
			<div class="nt-body" onclick="send_event('ed','Edit','%d')">%s</div>%s%s
		</div>`, b.ID, b.ID, b.ID, b.ID, body, badge, menu)
	}
	sb.WriteString(`<div class="nt-addblk" onclick="send_event('ed','AddEnd','')">＋ Agregar bloque</div>`)

	if kids := childPages(s.page); len(kids) > 0 {
		sb.WriteString(`<div class="nt-subpages"><div class="nt-sub-h">Subpáginas</div>`)
		for _, p := range kids {
			fmt.Fprintf(&sb, `<div class="nt-sublink" onclick="send_event('sb','Open','%d')">%s %s</div>`, p.ID, p.Icon, esc(p.Title))
		}
		sb.WriteString(`</div>`)
	}
	return sb.String()
}
