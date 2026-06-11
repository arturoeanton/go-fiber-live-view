package components

import (
	"fmt"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// InputText is a text input with optional label and placeholder.
// Type can be "text" (default), "password", "email", "number", etc.
// Events: Change, KeyUp, Enter, Blur.
type InputText struct {
	*view.ComponentDriver[*InputText]
	Label       string
	Placeholder string
	Value       string
	Type        string
}

func (t *InputText) GetDriver() view.LiveDriver {
	return t
}

func (t *InputText) Start() {
	t.Commit()
}

func (t *InputText) GetTemplate() string {
	return `<div class="lv-field">
	{{if .Label}}<label class="lv-label" for="{{.IdComponent}}">{{.Label}}</label>{{end}}
	<input class="lv-input" type="{{if .Type}}{{.Type}}{{else}}text{{end}}" id="{{.IdComponent}}"
		placeholder="{{.Placeholder}}" value="{{.Value}}"
		onchange="send_event(this.id,'Change',this.value)"
		onkeyup="send_event(this.id,'KeyUp',this.value)"
		onblur="send_event(this.id,'Blur',this.value)"
		onkeypress="if(event.key==='Enter'){send_event(this.id,'Enter',this.value)}" />
</div>`
}

// Change keeps Value in sync when no custom handler is registered.
func (t *InputText) Change(data interface{}) {
	t.Value = fmt.Sprint(data)
}

func (t *InputText) KeyUp(data interface{}) {}

func (t *InputText) Blur(data interface{}) {}

func (t *InputText) Enter(data interface{}) {}

func (t *InputText) SetChange(fx func(c *InputText, data interface{})) *InputText {
	t.Events["Change"] = fx
	return t
}

func (t *InputText) SetKeyUp(fx func(c *InputText, data interface{})) *InputText {
	t.Events["KeyUp"] = fx
	return t
}

// SetEnter registers a handler fired when the user presses Enter.
func (t *InputText) SetEnter(fx func(c *InputText, data interface{})) *InputText {
	t.Events["Enter"] = fx
	return t
}
