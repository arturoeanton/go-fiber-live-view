package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// NavItem is one entry of a NavBar; Code is sent on click.
type NavItem struct {
	Code    string
	Caption string
}

// NavBar is a top navigation bar. Events: ItemClick (data is the item code).
type NavBar struct {
	*view.ComponentDriver[*NavBar]
	Brand string
	Items []NavItem
}

func (t *NavBar) GetDriver() view.LiveDriver {
	return t
}

func (t *NavBar) Start() {
	t.Commit()
}

func (t *NavBar) GetTemplate() string {
	return `<nav class="lv-nav" id="{{.IdComponent}}">
	<span class="lv-nav-brand">{{.Brand}}</span>
	{{range .Items}}<button class="lv-nav-item" onclick="send_event('{{$.IdComponent}}','ItemClick','{{.Code}}')">{{.Caption}}</button>{{end}}
</nav>`
}

func (t *NavBar) SetItemClick(fx func(c *NavBar, data interface{})) *NavBar {
	t.Events["ItemClick"] = fx
	return t
}
