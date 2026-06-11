package main

import (
	"database/sql"
	"encoding/json"
	"log"

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
	CREATE TABLE IF NOT EXISTS boards(name TEXT PRIMARY KEY);
	CREATE TABLE IF NOT EXISTS elements(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		board TEXT NOT NULL,
		kind TEXT NOT NULL,
		x REAL, y REAL, w REAL, h REAL,
		points TEXT,
		stroke TEXT, fill TEXT, stroke_w REAL,
		text TEXT, z INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_elements_board ON elements(board);
	INSERT OR IGNORE INTO boards(name) VALUES('main');`
	_, err = db.Exec(schema)
	return err
}

func dbListBoards() []string {
	rows, err := db.Query(`SELECT name FROM boards ORDER BY name`)
	if err != nil {
		log.Println("db:", err)
		return []string{"main"}
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		names = append(names, n)
	}
	return names
}

func dbCreateBoard(name string) {
	if _, err := db.Exec(`INSERT OR IGNORE INTO boards(name) VALUES(?)`, name); err != nil {
		log.Println("db:", err)
	}
}

func dbLoadElements(board string) []*Element {
	rows, err := db.Query(`SELECT id, kind, x, y, w, h, points, stroke, fill, stroke_w, text
		FROM elements WHERE board = ? ORDER BY z, id`, board)
	if err != nil {
		log.Println("db:", err)
		return nil
	}
	defer rows.Close()
	var els []*Element
	for rows.Next() {
		e := &Element{}
		var points string
		if err := rows.Scan(&e.ID, &e.Kind, &e.X, &e.Y, &e.W, &e.H, &points, &e.Stroke, &e.Fill, &e.StrokeW, &e.Text); err != nil {
			log.Println("db:", err)
			continue
		}
		if points != "" {
			json.Unmarshal([]byte(points), &e.Points)
		}
		els = append(els, e)
	}
	return els
}

func pointsJSON(e *Element) string {
	if len(e.Points) == 0 {
		return ""
	}
	b, _ := json.Marshal(e.Points)
	return string(b)
}

func dbInsert(board string, e *Element) {
	res, err := db.Exec(`INSERT INTO elements(board, kind, x, y, w, h, points, stroke, fill, stroke_w, text, z)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,0)`,
		board, e.Kind, e.X, e.Y, e.W, e.H, pointsJSON(e), e.Stroke, e.Fill, e.StrokeW, e.Text)
	if err != nil {
		log.Println("db:", err)
		return
	}
	e.ID, _ = res.LastInsertId()
}

// dbInsertWithID restores a deleted element keeping its original id (undo).
func dbInsertWithID(board string, e *Element) {
	_, err := db.Exec(`INSERT OR REPLACE INTO elements(id, board, kind, x, y, w, h, points, stroke, fill, stroke_w, text, z)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0)`,
		e.ID, board, e.Kind, e.X, e.Y, e.W, e.H, pointsJSON(e), e.Stroke, e.Fill, e.StrokeW, e.Text)
	if err != nil {
		log.Println("db:", err)
	}
}

func dbUpdate(e *Element) {
	_, err := db.Exec(`UPDATE elements SET x=?, y=?, w=?, h=?, points=?, stroke=?, fill=?, stroke_w=?, text=? WHERE id=?`,
		e.X, e.Y, e.W, e.H, pointsJSON(e), e.Stroke, e.Fill, e.StrokeW, e.Text, e.ID)
	if err != nil {
		log.Println("db:", err)
	}
}

func dbDelete(id int64) {
	if _, err := db.Exec(`DELETE FROM elements WHERE id=?`, id); err != nil {
		log.Println("db:", err)
	}
}

func dbClear(board string) {
	if _, err := db.Exec(`DELETE FROM elements WHERE board=?`, board); err != nil {
		log.Println("db:", err)
	}
}
