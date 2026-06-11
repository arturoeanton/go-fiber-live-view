package components

import (
	"fmt"
	"strconv"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// Table renders headers and rows. Events: RowClick (data is the row index
// as string; use RowIndex to parse it).
type Table struct {
	*view.ComponentDriver[*Table]
	Headers []string
	Rows    [][]string
}

func (t *Table) GetDriver() view.LiveDriver {
	return t
}

func (t *Table) Start() {
	t.Commit()
}

func (t *Table) GetTemplate() string {
	return `<table class="lv-table" id="{{.IdComponent}}">
	<thead><tr>{{range .Headers}}<th>{{.}}</th>{{end}}</tr></thead>
	<tbody>
	{{range $i, $row := .Rows}}<tr onclick="send_event('{{$.IdComponent}}','RowClick','{{$i}}')">{{range $row}}<td>{{.}}</td>{{end}}</tr>
	{{end}}
	</tbody>
</table>`
}

// RowIndex parses the data of a RowClick event into a row index.
func RowIndex(data interface{}) int {
	i, _ := strconv.Atoi(fmt.Sprint(data))
	return i
}

// AddRow appends a row and re-renders.
func (t *Table) AddRow(row []string) *Table {
	t.Rows = append(t.Rows, row)
	t.Commit()
	return t
}

// SetRows replaces all rows and re-renders.
func (t *Table) SetRows(rows [][]string) *Table {
	t.Rows = rows
	t.Commit()
	return t
}

func (t *Table) SetRowClick(fx func(c *Table, data interface{})) *Table {
	t.Events["RowClick"] = fx
	return t
}
