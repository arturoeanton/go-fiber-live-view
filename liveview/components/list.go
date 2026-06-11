package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// List renders a clickable list of items. Events: ItemClick (data is the
// item index as string; use RowIndex to parse it).
type List struct {
	*view.ComponentDriver[*List]
	Items []string
}

func (t *List) GetDriver() view.LiveDriver {
	return t
}

func (t *List) Start() {
	t.Commit()
}

func (t *List) GetTemplate() string {
	return `<ul class="lv-list" id="{{.IdComponent}}">
	{{range $i, $item := .Items}}<li class="lv-list-item" onclick="send_event('{{$.IdComponent}}','ItemClick','{{$i}}')">{{$item}}</li>{{end}}
</ul>`
}

// AddItem appends an item and re-renders.
func (t *List) AddItem(item string) *List {
	t.Items = append(t.Items, item)
	t.Commit()
	return t
}

// SetItems replaces all items and re-renders.
func (t *List) SetItems(items []string) *List {
	t.Items = items
	t.Commit()
	return t
}

func (t *List) SetItemClick(fx func(c *List, data interface{})) *List {
	t.Events["ItemClick"] = fx
	return t
}
