package components

import (
	"fmt"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// Option is a value/label pair used by Select and RadioGroup.
type Option struct {
	Value string
	Label string
}

// Select is a dropdown. Events: Change (data is the selected value).
type Select struct {
	*view.ComponentDriver[*Select]
	Label    string
	Options  []Option
	Selected string
}

func (t *Select) GetDriver() view.LiveDriver {
	return t
}

func (t *Select) Start() {
	t.Commit()
}

func (t *Select) GetTemplate() string {
	return `<div class="lv-field">
	{{if .Label}}<label class="lv-label" for="{{.IdComponent}}">{{.Label}}</label>{{end}}
	<select class="lv-select" id="{{.IdComponent}}" onchange="send_event(this.id,'Change',this.value)">
		{{range .Options}}<option value="{{.Value}}" {{if eq $.Selected .Value}}selected{{end}}>{{.Label}}</option>{{end}}
	</select>
</div>`
}

// Change keeps Selected in sync when no custom handler is registered.
func (t *Select) Change(data interface{}) {
	t.Selected = fmt.Sprint(data)
}

func (t *Select) SetChange(fx func(c *Select, data interface{})) *Select {
	t.Events["Change"] = fx
	return t
}

// SetOptions replaces the options and re-renders.
func (t *Select) SetOptions(options []Option) *Select {
	t.Options = options
	t.Commit()
	return t
}
