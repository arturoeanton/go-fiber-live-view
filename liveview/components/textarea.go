package components

import (
	"fmt"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// TextArea is a multi-line text input. Events: Change, KeyUp.
type TextArea struct {
	*view.ComponentDriver[*TextArea]
	Label       string
	Placeholder string
	Value       string
	Rows        int
}

func (t *TextArea) GetDriver() view.LiveDriver {
	return t
}

func (t *TextArea) Start() {
	if t.Rows == 0 {
		t.Rows = 4
	}
	t.Commit()
}

func (t *TextArea) GetTemplate() string {
	return `<div class="lv-field">
	{{if .Label}}<label class="lv-label" for="{{.IdComponent}}">{{.Label}}</label>{{end}}
	<textarea class="lv-textarea" id="{{.IdComponent}}" rows="{{.Rows}}" placeholder="{{.Placeholder}}"
		onchange="send_event(this.id,'Change',this.value)"
		onkeyup="send_event(this.id,'KeyUp',this.value)">{{.Value}}</textarea>
</div>`
}

func (t *TextArea) Change(data interface{}) {
	t.Value = fmt.Sprint(data)
}

func (t *TextArea) KeyUp(data interface{}) {}

func (t *TextArea) SetChange(fx func(c *TextArea, data interface{})) *TextArea {
	t.Events["Change"] = fx
	return t
}
