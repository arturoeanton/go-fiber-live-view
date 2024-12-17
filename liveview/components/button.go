package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

type Button struct {
	*view.ComponentDriver[*Button]
	I       int
	Caption string
}

func (t *Button) Start() {
	t.Commit()
}

func (t *Button) GetTemplate() string {
	return `<Button id="{{.IdComponent}}" onclick="send_event(this.id,'Click')" >{{.Caption}}</button>`
}

func (t *Button) GetDriver() view.LiveDriver {
	return t
}

func (t *Button) SetClick(fx func(c *Button, data interface{})) *Button {
	t.Events["Click"] = fx
	return t
}
