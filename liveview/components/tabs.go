package components

import (
	"fmt"
	"strconv"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// Tab is a single tab with a title and raw HTML content.
type Tab struct {
	Title   string
	Content string
}

// Tabs renders a tab bar with switchable content panels.
// Events: SelectTab (data is the tab index).
type Tabs struct {
	*view.ComponentDriver[*Tabs]
	Tabs   []Tab
	Active int
}

func (t *Tabs) GetDriver() view.LiveDriver {
	return t
}

func (t *Tabs) Start() {
	t.Commit()
}

func (t *Tabs) GetTemplate() string {
	return `<div id="{{.IdComponent}}">
	<div class="lv-tabs-nav">
	{{range $i, $tab := .Tabs}}<button class="lv-tab{{if eqInt $i $.Active}} lv-tab-active{{end}}" onclick="send_event('{{$.IdComponent}}','SelectTab','{{$i}}')">{{$tab.Title}}</button>{{end}}
	</div>
	<div class="lv-tabs-content">
	{{range $i, $tab := .Tabs}}{{if eqInt $i $.Active}}{{$tab.Content}}{{end}}{{end}}
	</div>
</div>`
}

// SelectTab handles the default tab switch event.
func (t *Tabs) SelectTab(data interface{}) {
	i, _ := strconv.Atoi(fmt.Sprint(data))
	if i >= 0 && i < len(t.Tabs) {
		t.Active = i
		t.Commit()
	}
}

func (t *Tabs) SetSelectTab(fx func(c *Tabs, data interface{})) *Tabs {
	t.Events["SelectTab"] = fx
	return t
}
