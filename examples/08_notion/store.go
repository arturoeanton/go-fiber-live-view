package main

import (
	"database/sql"
	"log"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func openDB(path string) error {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS pages(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parent_id INTEGER NOT NULL DEFAULT 0,
		title TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '📄',
		pos REAL NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS blocks(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		page_id INTEGER NOT NULL,
		type TEXT NOT NULL DEFAULT 'p',
		content TEXT NOT NULL DEFAULT '',
		checked INTEGER NOT NULL DEFAULT 0,
		pos REAL NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_blocks_page ON blocks(page_id);`
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// migrations for databases created by older versions; the error when the
	// column already exists is expected and ignored
	for _, m := range []string{
		`ALTER TABLE pages ADD COLUMN cover TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE pages ADD COLUMN fav INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE pages ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE blocks ADD COLUMN indent INTEGER NOT NULL DEFAULT 0`,
	} {
		db.Exec(m)
	}
	return seedIfEmpty()
}

// ------------------------------------------------------------- in-memory --

type Page struct {
	ID        int64
	Parent    int64
	Title     string
	Icon      string
	Pos       float64
	Cover     string
	Fav       bool
	UpdatedAt int64
}

type Block struct {
	ID      int64
	PageID  int64
	Type    string // p,h1,h2,h3,todo,bullet,number,toggle,quote,code,callout,divider,image,table
	Content string
	Checked bool
	Pos     float64
	Indent  int
}

var (
	pages      = map[int64]*Page{}
	pageBlocks = map[int64][]*Block{}
)

func loadPages() {
	rows, err := db.Query(`SELECT id, parent_id, title, icon, pos, cover, fav, updated_at FROM pages`)
	if err != nil {
		log.Println("db:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		p := &Page{}
		var fav int
		rows.Scan(&p.ID, &p.Parent, &p.Title, &p.Icon, &p.Pos, &p.Cover, &fav, &p.UpdatedAt)
		p.Fav = fav == 1
		pages[p.ID] = p
	}
}

// getBlocks lazily loads and caches a page's blocks. Callers hold mu.
func getBlocks(pageID int64) []*Block {
	if bs, ok := pageBlocks[pageID]; ok {
		return bs
	}
	rows, err := db.Query(`SELECT id, page_id, type, content, checked, pos, indent FROM blocks WHERE page_id=? ORDER BY pos, id`, pageID)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	defer rows.Close()
	var bs []*Block
	for rows.Next() {
		b := &Block{}
		var checked int
		rows.Scan(&b.ID, &b.PageID, &b.Type, &b.Content, &checked, &b.Pos, &b.Indent)
		b.Checked = checked == 1
		bs = append(bs, b)
	}
	pageBlocks[pageID] = bs
	return bs
}

func sortBlocks(pageID int64) {
	bs := pageBlocks[pageID]
	sort.SliceStable(bs, func(i, j int) bool { return bs[i].Pos < bs[j].Pos })
}

func childPages(parent int64) []*Page {
	var out []*Page
	for _, p := range pages {
		if p.Parent == parent {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pos != out[j].Pos {
			return out[i].Pos < out[j].Pos
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func favPages() []*Page {
	var out []*Page
	for _, p := range pages {
		if p.Fav {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out
}

func findBlock(pageID, id int64) (*Block, int) {
	for i, b := range getBlocks(pageID) {
		if b.ID == id {
			return b, i
		}
	}
	return nil, -1
}

// touchPage stamps the page as edited now.
func touchPage(pageID int64) {
	if p := pages[pageID]; p != nil {
		p.UpdatedAt = time.Now().Unix()
		dbUpdatePage(p)
	}
}

// ------------------------------------------------------------ db helpers --

func dbAddPage(parent int64, title string) *Page {
	pos := float64(len(childPages(parent)) + 1)
	now := time.Now().Unix()
	res, err := db.Exec(`INSERT INTO pages(parent_id, title, icon, pos, cover, fav, updated_at) VALUES(?,?,?,?,'',0,?)`, parent, title, "📄", pos, now)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	id, _ := res.LastInsertId()
	p := &Page{ID: id, Parent: parent, Title: title, Icon: "📄", Pos: pos, UpdatedAt: now}
	pages[id] = p
	return p
}

func dbUpdatePage(p *Page) {
	fav := 0
	if p.Fav {
		fav = 1
	}
	if _, err := db.Exec(`UPDATE pages SET title=?, icon=?, parent_id=?, pos=?, cover=?, fav=?, updated_at=? WHERE id=?`,
		p.Title, p.Icon, p.Parent, p.Pos, p.Cover, fav, p.UpdatedAt, p.ID); err != nil {
		log.Println("db:", err)
	}
}

// dbDeletePage removes a page, his blocks and all his descendants.
func dbDeletePage(id int64) {
	for _, c := range childPages(id) {
		dbDeletePage(c.ID)
	}
	db.Exec(`DELETE FROM blocks WHERE page_id=?`, id)
	db.Exec(`DELETE FROM pages WHERE id=?`, id)
	delete(pages, id)
	delete(pageBlocks, id)
}

func dbAddBlock(pageID int64, typ, content string, pos float64, indent int) *Block {
	res, err := db.Exec(`INSERT INTO blocks(page_id, type, content, checked, pos, indent) VALUES(?,?,?,0,?,?)`, pageID, typ, content, pos, indent)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	id, _ := res.LastInsertId()
	b := &Block{ID: id, PageID: pageID, Type: typ, Content: content, Pos: pos, Indent: indent}
	getBlocks(pageID)
	pageBlocks[pageID] = append(pageBlocks[pageID], b)
	sortBlocks(pageID)
	return b
}

func dbUpdateBlock(b *Block) {
	checked := 0
	if b.Checked {
		checked = 1
	}
	if _, err := db.Exec(`UPDATE blocks SET type=?, content=?, checked=?, pos=?, indent=? WHERE id=?`, b.Type, b.Content, checked, b.Pos, b.Indent, b.ID); err != nil {
		log.Println("db:", err)
	}
}

func dbDeleteBlock(pageID, id int64) {
	db.Exec(`DELETE FROM blocks WHERE id=?`, id)
	bs := getBlocks(pageID)
	for i, b := range bs {
		if b.ID == id {
			pageBlocks[pageID] = append(bs[:i], bs[i+1:]...)
			break
		}
	}
}

// posAfter returns a position that places a new block right after the given
// index (fractional ordering keeps drag&drop and inserts O(1)).
func posAfter(pageID int64, idx int) float64 {
	bs := getBlocks(pageID)
	if idx < 0 || len(bs) == 0 {
		if len(bs) == 0 {
			return 1
		}
		return bs[0].Pos - 1
	}
	if idx >= len(bs)-1 {
		return bs[len(bs)-1].Pos + 1
	}
	return (bs[idx].Pos + bs[idx+1].Pos) / 2
}

// -------------------------------------------------------------- search --

type searchHit struct {
	PageID  int64
	Icon    string
	Title   string
	Snippet string // empty for title hits
}

func searchAll(q string) []searchHit {
	var hits []searchHit
	like := "%" + q + "%"
	rows, err := db.Query(`SELECT id FROM pages WHERE title LIKE ? LIMIT 6`, like)
	if err == nil {
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			if p := pages[id]; p != nil {
				hits = append(hits, searchHit{PageID: p.ID, Icon: p.Icon, Title: p.Title})
			}
		}
		rows.Close()
	}
	rows, err = db.Query(`SELECT page_id, content FROM blocks WHERE content LIKE ? AND type NOT IN ('table','image') LIMIT 8`, like)
	if err == nil {
		for rows.Next() {
			var pid int64
			var content string
			rows.Scan(&pid, &content)
			p := pages[pid]
			if p == nil {
				continue
			}
			hits = append(hits, searchHit{PageID: p.ID, Icon: p.Icon, Title: p.Title, Snippet: makeSnippet(content, q)})
		}
		rows.Close()
	}
	return hits
}

func makeSnippet(content, q string) string {
	r := []rune(content)
	low := strings.ToLower(content)
	i := strings.Index(low, strings.ToLower(q))
	if i < 0 {
		if len(r) > 60 {
			return string(r[:60]) + "…"
		}
		return content
	}
	ri := len([]rune(content[:i]))
	from := ri - 25
	if from < 0 {
		from = 0
	}
	to := ri + len([]rune(q)) + 35
	if to > len(r) {
		to = len(r)
	}
	out := string(r[from:to])
	if from > 0 {
		out = "…" + out
	}
	if to < len(r) {
		out += "…"
	}
	return out
}

// ----------------------------------------------------------------- seed --

func seedIfEmpty() error {
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM pages`).Scan(&n); err != nil {
		return err
	}
	loadPages()
	if n > 0 {
		return nil
	}
	home := dbAddPage(0, "Bienvenido")
	home.Icon = "👋"
	home.Cover = coverPresets[0]
	home.Fav = true
	dbUpdatePage(home)
	blocks := []struct {
		typ, content string
		indent       int
	}{
		{"h1", "Bienvenido a tu workspace", 0},
		{"p", "Este es un clon de **Notion** corriendo 100% en Go. Hacé click en cualquier bloque para editarlo.", 0},
		{"h2", "Probá el menú /", 0},
		{"p", "En un bloque vacío escribí `/` y filtrá: /h1, /todo, /tabla, /imagen, /desplegable…", 0},
		{"h2", "Atajos", 0},
		{"bullet", "`# `, `## `, `### ` al inicio crean títulos; `- ` lista, `[] ` todo, `> ` cita", 0},
		{"bullet", "**Enter** crea bloque, **Backspace** en vacío borra, **↑/↓** navegan entre bloques", 0},
		{"bullet", "**Tab / Shift+Tab** indentan, **Ctrl+K** busca en todo el workspace", 0},
		{"toggle", "Un bloque desplegable (click en la flecha)", 0},
		{"todo", "Los hijos se indentan con Tab", 1},
		{"todo", "Y se pliegan con el toggle", 1},
		{"h2", "Bloques", 0},
		{"todo", "Probar los checkboxes", 0},
		{"todo", "Arrastrar bloques con el handle ⋮⋮", 0},
		{"quote", "El estado vive en el servidor. Abrí esta página en dos browsers y editá a la vez.", 0},
		{"callout", "💡 Cada bloque muestra quién lo está editando, en vivo. Y la página tiene portada: probá «Cambiar portada».", 0},
		{"code", "func main() {\n    fmt.Println(\"hola desde Go\")\n}", 0},
		{"divider", "", 0},
		{"table", `[["Lenguaje","Realtime","Tipado"],["Go","✅","estático"],["Elixir","✅","dinámico"],["JS","según","opcional"]]`, 0},
		{"image", "https://go.dev/images/gophers/ladder.svg", 0},
	}
	for i, bl := range blocks {
		dbAddBlock(home.ID, bl.typ, bl.content, float64(i+1), bl.indent)
	}
	sub := dbAddPage(home.ID, "Cómo funciona")
	sub.Icon = "⚙️"
	dbUpdatePage(sub)
	dbAddBlock(sub.ID, "p", "Cada página es un árbol de bloques guardado en **SQLite**. La sincronización es a nivel bloque (last-write-wins), igual que Notion.", 1, 0)
	return nil
}
