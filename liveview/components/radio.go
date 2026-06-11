package components

import (
	"fmt"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// RadioGroup is a group of radio buttons. Events: Change (data is the
// selected value).
type RadioGroup struct {
	*view.ComponentDriver[*RadioGroup]
	Label    string
	Options  []Option
	Selected string
}

func (t *RadioGroup) GetDriver() view.LiveDriver {
	return t
}

func (t *RadioGroup) Start() {
	t.Commit()
}

func (t *RadioGroup) GetTemplate() string {
	return `<div class="lv-field" id="{{.IdComponent}}">
	{{if .Label}}<span class="lv-label">{{.Label}}</span>{{end}}
	<div>
	{{range .Options}}
		<label class="lv-radio-item">
			<input type="radio" name="{{$.IdComponent}}" value="{{.Value}}" {{if eq $.Selected .Value}}checked{{end}}
				onchange="send_event('{{$.IdComponent}}','Change',this.value)" />
			<span>{{.Label}}</span>
		</label>
	{{end}}
	</div>
</div>`
}

// Change keeps Selected in sync when no custom handler is registered.
func (t *RadioGroup) Change(data interface{}) {
	t.Selected = fmt.Sprint(data)
}

func (t *RadioGroup) SetChange(fx func(c *RadioGroup, data interface{})) *RadioGroup {
	t.Events["Change"] = fx
	return t
}
