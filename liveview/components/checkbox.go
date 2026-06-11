package components

import (
	"fmt"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// Checkbox is a single check. Events: Change (data is "true"/"false").
type Checkbox struct {
	*view.ComponentDriver[*Checkbox]
	Label   string
	Checked bool
}

func (t *Checkbox) GetDriver() view.LiveDriver {
	return t
}

func (t *Checkbox) Start() {
	t.Commit()
}

func (t *Checkbox) GetTemplate() string {
	return `<label class="lv-check">
	<input type="checkbox" id="{{.IdComponent}}" {{if .Checked}}checked{{end}}
		onchange="send_event(this.id,'Change',this.checked?'true':'false')" />
	<span>{{.Label}}</span>
</label>`
}

// Change keeps Checked in sync when no custom handler is registered.
func (t *Checkbox) Change(data interface{}) {
	t.Checked = fmt.Sprint(data) == "true"
}

func (t *Checkbox) SetChange(fx func(c *Checkbox, data interface{})) *Checkbox {
	t.Events["Change"] = fx
	return t
}
