package main

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
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

func agoLabel(ts int64) string {
	if ts == 0 {
		return ""
	}
	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return "Editado recién"
	case d < time.Hour:
		return fmt.Sprintf("Editado hace %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("Editado hace %d h", int(d.Hours()))
	default:
		return fmt.Sprintf("Editado hace %d días", int(d.Hours()/24))
	}
}

// ------------------------------------------------------------- sidebar --

func sidebarRow(s *session, p *Page, depth int, caret string) string {
	cls := "sb-row"
	if p.ID == s.page {
		cls += " sb-active"
	}
	return fmt.Sprintf(`<div class="%s" style="padding-left:%dpx" onclick="send_event('sb','Open','%d')">%s<span class="sb-ico">%s</span><span class="sb-title">%s</span><span class="sb-act" title="Subpágina" onclick="event.stopPropagation();send_event('sb','Add','%d')">＋</span><span class="sb-act" title="Borrar" onclick="event.stopPropagation();if(confirm('¿Borrar página y subpáginas?'))send_event('sb','Del','%d')">🗑</span></div>`,
		cls, 6+depth*14, p.ID, caret, p.Icon, esc(p.Title), p.ID, p.ID)
}

func renderSidebarLocked(s *session) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `<div class="sb-search"><input id="nt_search" placeholder="🔎 Buscar…  (Ctrl+K)" value="%s" oninput="send_event('sb','Search',this.value)" onkeydown="if(event.key==='Escape'){this.value='';send_event('sb','Search','')}"/></div>`, esc(s.search))

	if s.search != "" {
		sb.WriteString(`<div class="sb-section">Resultados</div>`)
		hits := searchAll(s.search)
		if len(hits) == 0 {
			sb.WriteString(`<div class="sb-empty">Sin resultados</div>`)
		}
		for _, h := range hits {
			snippet := ""
			if h.Snippet != "" {
				snippet = `<div class="sb-snippet">` + highlight(h.Snippet, s.search) + `</div>`
			}
			fmt.Fprintf(&sb, `<div class="sb-row sb-hit" onclick="send_event('sb','Open','%d')"><span class="sb-ico">%s</span><div class="sb-hitbody"><span class="sb-title">%s</span>%s</div></div>`,
				h.PageID, h.Icon, esc(h.Title), snippet)
		}
		return sb.String()
	}

	if favs := favPages(); len(favs) > 0 {
		sb.WriteString(`<div class="sb-section">⭐ Favoritos</div>`)
		for _, p := range favs {
			sb.WriteString(sidebarRow(s, p, 0, `<span class="sb-caret">·</span>`))
		}
	}

	sb.WriteString(`<div class="sb-section">Páginas</div>`)
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
			sb.WriteString(sidebarRow(s, p, depth, caret))
			if !s.collapsed[p.ID] {
				walk(p.ID, depth+1)
			}
		}
	}
	walk(0, 0)
	return sb.String()
}

func highlight(text, q string) string {
	e := esc(text)
	eq := esc(q)
	idx := strings.Index(strings.ToLower(e), strings.ToLower(eq))
	if idx < 0 {
		return e
	}
	return e[:idx] + "<b>" + e[idx:idx+len(eq)] + "</b>" + e[idx+len(eq):]
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
	sb.WriteString(`</div><div class="nt-topright">`)
	if p := pages[s.page]; p != nil {
		fmt.Fprintf(&sb, `<span class="nt-ago">%s</span>`, agoLabel(p.UpdatedAt))
		star, title := "☆", "Agregar a favoritos"
		if p.Fav {
			star, title = "⭐", "Quitar de favoritos"
		}
		fmt.Fprintf(&sb, `<span class="nt-star" title="%s" onclick="send_event('ed','FavToggle','')">%s</span>`, title, star)
	}
	sb.WriteString(`<div class="nt-presence">`)
	for _, o := range sessions {
		if o.page != s.page {
			continue
		}
		initial := strings.TrimPrefix(o.name, "user-")
		fmt.Fprintf(&sb, `<span class="nt-avatar" style="background:%s" title="%s">%s</span>`, o.color, esc(o.name), esc(initial))
	}
	sb.WriteString(`</div></div>`)
	return sb.String()
}

// ----------------------------------------------------------- page head --

var coverPresets = []string{
	"linear-gradient(135deg,#667eea,#764ba2)",
	"linear-gradient(135deg,#f6d365,#fda085)",
	"linear-gradient(135deg,#84fab0,#8fd3f4)",
	"linear-gradient(135deg,#fa709a,#fee140)",
	"linear-gradient(135deg,#30cfd0,#330867)",
	"linear-gradient(135deg,#a8edea,#fed6e3)",
	"linear-gradient(135deg,#5ee7df,#b490ca)",
	"linear-gradient(135deg,#d299c2,#fef9d7)",
}

var iconPalette = []string{"📄", "👋", "⚙️", "📚", "💡", "🚀", "🎯", "📊", "🗂", "✅", "🧠", "🔥", "🐹", "💬", "📝", "⭐", "🏠", "🌱", "🎨", "🔧"}

func renderCoverLocked(s *session) string {
	p := pages[s.page]
	if p == nil {
		return ""
	}
	if p.Cover == "" {
		return `<div class="nt-coverbar"><span class="nt-coverbtn" onclick="send_event('ed','CoverSet','')">🖼 Agregar portada</span></div>`
	}
	style := "background:" + p.Cover
	if strings.HasPrefix(p.Cover, "http") {
		style = fmt.Sprintf("background:url('%s') center/cover", esc(p.Cover))
	}
	return fmt.Sprintf(`<div class="nt-cover" style="%s">
		<div class="nt-coveracts">
			<span class="nt-coverbtn" onclick="send_event('ed','CoverNext','')">Cambiar</span>
			<span class="nt-coverbtn" onclick="send_event('ed','CoverDel','')">Quitar</span>
		</div>
	</div>`, style)
}

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
	fmt.Fprintf(&sb, `<input id="title_input" class="nt-title" value="%s" placeholder="Sin título" onchange="send_event('ed','Title',this.value)"/>`, esc(p.Title))
	return sb.String()
}

// -------------------------------------------------------------- editor --

type blockTypeDef struct{ code, label, icon, keywords string }

var blockTypes = []blockTypeDef{
	{"p", "Texto", "¶", "texto parrafo paragraph"},
	{"h1", "Título 1", "H1", "titulo heading encabezado h1"},
	{"h2", "Título 2", "H2", "titulo heading encabezado h2"},
	{"h3", "Título 3", "H3", "titulo heading encabezado h3"},
	{"todo", "Lista de tareas", "☑", "todo tarea check checkbox"},
	{"bullet", "Lista", "•", "lista bullet viñeta"},
	{"number", "Lista numerada", "1.", "numerada numeros ordered"},
	{"toggle", "Desplegable", "▸", "toggle desplegable plegar"},
	{"quote", "Cita", "❝", "cita quote"},
	{"code", "Código", "{}", "codigo código code"},
	{"callout", "Callout", "💡", "callout nota aviso"},
	{"divider", "Divisor", "—", "divisor separador divider hr"},
	{"image", "Imagen", "🖼", "imagen image foto url"},
	{"table", "Tabla", "▦", "tabla table grilla"},
}

func validType(t string) bool {
	for _, bt := range blockTypes {
		if bt.code == t {
			return true
		}
	}
	return false
}

func filterTypes(q string) []blockTypeDef {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return blockTypes
	}
	var out []blockTypeDef
	for _, bt := range blockTypes {
		if strings.Contains(strings.ToLower(bt.label), q) || strings.Contains(bt.keywords, q) || strings.Contains(bt.code, q) {
			out = append(out, bt)
		}
	}
	return out
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
	case "toggle":
		sym := "▾"
		if s.toggles[b.ID] {
			sym = "▸"
		}
		return fmt.Sprintf(`<div class="nt-li"><span class="nt-marker nt-togglec" onclick="event.stopPropagation();send_event('ed','FoldBlk','%d')">%s</span><span>%s</span></div>`,
			b.ID, sym, inline(b.Content))
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
		fmt.Fprintf(&sb, `<div class="nt-menu-item" onclick="event.stopPropagation();send_event('ed','SetType','%d|%s')"><span class="nt-mi-ico">%s</span>%s</div>`, id, bt.code, bt.icon, bt.label)
	}
	fmt.Fprintf(&sb, `<div class="nt-menu-item" onclick="event.stopPropagation();send_event('ed','Duplicate','%d')"><span class="nt-mi-ico">⧉</span>Duplicar</div>`, id)
	fmt.Fprintf(&sb, `<div class="nt-menu-item nt-menu-del" onclick="event.stopPropagation();send_event('ed','DelBlock','%d')"><span class="nt-mi-ico">🗑</span>Eliminar</div>`, id)
	sb.WriteString(`</div>`)
	return sb.String()
}

// renderSlashMenu is the floating "/" menu with live filtering.
func renderSlashMenu(id int64, filter string) string {
	matches := filterTypes(filter)
	var sb strings.Builder
	sb.WriteString(`<div class="nt-menu nt-slash">`)
	if len(matches) == 0 {
		sb.WriteString(`<div class="nt-menu-item lv-muted">Sin coincidencias</div>`)
	}
	for i, bt := range matches {
		cls := "nt-menu-item"
		if i == 0 {
			cls += " nt-mi-first"
		}
		fmt.Fprintf(&sb, `<div class="%s" onclick="event.stopPropagation();send_event('ed','SlashSet','%d|%s')"><span class="nt-mi-ico">%s</span>%s</div>`, cls, id, bt.code, bt.icon, bt.label)
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

func renderEditorLocked(s *session) string {
	if pages[s.page] == nil {
		return ""
	}
	var sb strings.Builder
	bs := getBlocks(s.page)
	num := 0
	lastNumIndent := -1
	i := 0
	for i < len(bs) {
		b := bs[i]
		if b.Type == "number" && b.Indent == lastNumIndent {
			num++
		} else if b.Type == "number" {
			num = 1
			lastNumIndent = b.Indent
		} else {
			num = 0
			lastNumIndent = -1
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
		if s.slashFor == b.ID {
			menu = renderSlashMenu(b.ID, s.slashFilter)
		}
		fmt.Fprintf(&sb, `<div class="nt-blk" data-bid="%d" style="padding-left:%dpx" ondragover="lvDragOver(event,this)" ondragleave="lvDragLeave(this)" ondrop="lvDrop(event,this)">
			<div class="nt-gut">
				<span class="nt-plus" title="Agregar bloque abajo" onclick="event.stopPropagation();send_event('ed','AddAfter','%d')">＋</span>
				<span class="nt-dots" title="Menú de bloque" onclick="event.stopPropagation();send_event('ed','Menu','%d')">⋯</span>
				<span class="nt-handle" title="Arrastrar" draggable="true" ondragstart="lvDragStart(event,'%d')">⋮⋮</span>
			</div>
			<div class="nt-body" onclick="send_event('ed','Edit','%d')">%s</div>%s%s
		</div>`, b.ID, b.Indent*26, b.ID, b.ID, b.ID, b.ID, body, badge, menu)

		if b.Type == "toggle" && s.toggles[b.ID] { // collapsed: skip children
			j := i + 1
			for j < len(bs) && bs[j].Indent > b.Indent {
				j++
			}
			i = j
			continue
		}
		i++
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
