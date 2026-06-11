package main

import (
	"database/sql"
	"log"
	"sort"

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
	return seedIfEmpty()
}

// ------------------------------------------------------------- in-memory --

type Page struct {
	ID     int64
	Parent int64
	Title  string
	Icon   string
	Pos    float64
}

type Block struct {
	ID      int64
	PageID  int64
	Type    string // p,h1,h2,h3,todo,bullet,number,quote,code,callout,divider,image,table
	Content string
	Checked bool
	Pos     float64
}

var (
	pages       = map[int64]*Page{}
	pageBlocks  = map[int64][]*Block{} // sorted by pos, lazy-loaded
	blocksDirty = map[int64]bool{}
)

func loadPages() {
	rows, err := db.Query(`SELECT id, parent_id, title, icon, pos FROM pages`)
	if err != nil {
		log.Println("db:", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		p := &Page{}
		rows.Scan(&p.ID, &p.Parent, &p.Title, &p.Icon, &p.Pos)
		pages[p.ID] = p
	}
}

// getBlocks lazily loads and caches a page's blocks. Callers hold mu.
func getBlocks(pageID int64) []*Block {
	if bs, ok := pageBlocks[pageID]; ok && !blocksDirty[pageID] {
		return bs
	}
	rows, err := db.Query(`SELECT id, page_id, type, content, checked, pos FROM blocks WHERE page_id=? ORDER BY pos, id`, pageID)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	defer rows.Close()
	var bs []*Block
	for rows.Next() {
		b := &Block{}
		var checked int
		rows.Scan(&b.ID, &b.PageID, &b.Type, &b.Content, &checked, &b.Pos)
		b.Checked = checked == 1
		bs = append(bs, b)
	}
	pageBlocks[pageID] = bs
	blocksDirty[pageID] = false
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

func findBlock(pageID, id int64) (*Block, int) {
	for i, b := range getBlocks(pageID) {
		if b.ID == id {
			return b, i
		}
	}
	return nil, -1
}

// ------------------------------------------------------------ db helpers --

func dbAddPage(parent int64, title string) *Page {
	pos := float64(len(childPages(parent)) + 1)
	res, err := db.Exec(`INSERT INTO pages(parent_id, title, icon, pos) VALUES(?,?,?,?)`, parent, title, "📄", pos)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	id, _ := res.LastInsertId()
	p := &Page{ID: id, Parent: parent, Title: title, Icon: "📄", Pos: pos}
	pages[id] = p
	return p
}

func dbUpdatePage(p *Page) {
	if _, err := db.Exec(`UPDATE pages SET title=?, icon=?, parent_id=?, pos=? WHERE id=?`, p.Title, p.Icon, p.Parent, p.Pos, p.ID); err != nil {
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

func dbAddBlock(pageID int64, typ, content string, pos float64) *Block {
	res, err := db.Exec(`INSERT INTO blocks(page_id, type, content, checked, pos) VALUES(?,?,?,0,?)`, pageID, typ, content, pos)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	id, _ := res.LastInsertId()
	b := &Block{ID: id, PageID: pageID, Type: typ, Content: content, Pos: pos}
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
	if _, err := db.Exec(`UPDATE blocks SET type=?, content=?, checked=?, pos=? WHERE id=?`, b.Type, b.Content, checked, b.Pos, b.ID); err != nil {
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
	dbUpdatePage(home)
	blocks := []struct {
		typ, content string
	}{
		{"h1", "Bienvenido a tu workspace"},
		{"p", "Este es un clon de **Notion** corriendo 100% en Go. Hacé click en cualquier bloque para editarlo."},
		{"h2", "Atajos de teclado"},
		{"bullet", "`# `, `## `, `### ` al inicio crean títulos"},
		{"bullet", "`- ` crea una lista, `[] ` un todo, `> ` una cita"},
		{"bullet", "Escribí `/` para ver los comandos: /h1, /todo, /tabla, /imagen…"},
		{"bullet", "**Enter** crea un bloque nuevo, **Backspace** en un bloque vacío lo borra"},
		{"h2", "Bloques"},
		{"todo", "Probar los checkboxes"},
		{"todo", "Arrastrar bloques con el handle ⋮⋮"},
		{"quote", "El estado vive en el servidor. Abrí esta página en dos browsers y editá a la vez."},
		{"callout", "💡 Cada bloque muestra quién lo está editando, en vivo."},
		{"code", "func main() {\n    fmt.Println(\"hola desde Go\")\n}"},
		{"divider", ""},
		{"table", `[["Lenguaje","Realtime","Tipado"],["Go","✅","estático"],["Elixir","✅","dinámico"],["JS","según","opcional"]]`},
		{"image", "https://go.dev/images/gophers/ladder.svg"},
	}
	for i, bl := range blocks {
		dbAddBlock(home.ID, bl.typ, bl.content, float64(i+1))
	}
	sub := dbAddPage(home.ID, "Cómo funciona")
	sub.Icon = "⚙️"
	dbUpdatePage(sub)
	dbAddBlock(sub.ID, "p", "Cada página es un árbol de bloques guardado en **SQLite**. La sincronización es a nivel bloque (last-write-wins), igual que Notion.", 1)
	return nil
}
