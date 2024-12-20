package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

type InputText struct {
	*view.ComponentDriver[*InputText]
}

func (t *InputText) GetDriver() view.LiveDriver {
	return t
}

func (t *InputText) Start() {
	t.Commit()
}

func (t *InputText) GetTemplate() string {
	return `<input type="text" 
	onkeypress="send_event(this.id,'KeyPress',this.value)"
	onchange="send_event(this.id,'Change',this.value)"
	onkeyup="send_event(this.id,'KeyUp',this.value)"
	id="{{.IdComponent}}"   />`
}

func (t *InputText) KeyPress(data interface{}) {

}

func (t *InputText) KeyUp(data interface{}) {}

func (t *InputText) Change(data interface{}) {}

func (t *InputText) SetKeyUp(fx func(c *InputText, data interface{})) *InputText {
	t.Events["KeyUp"] = fx
	return t
}
