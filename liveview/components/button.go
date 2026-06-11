package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// Button is a clickable button. Variants: "" (primary), "secondary",
// "success", "danger", "ghost".
type Button struct {
	*view.ComponentDriver[*Button]
	Caption  string
	Variant  string
	Disabled bool
	I        int
}

func (t *Button) Start() {
	t.Commit()
}

func (t *Button) GetTemplate() string {
	return `<button id="{{.IdComponent}}" class="lv-btn{{if .Variant}} lv-btn-{{.Variant}}{{end}}" {{if .Disabled}}disabled{{end}} onclick="send_event(this.id,'Click')">{{.Caption}}</button>`
}

func (t *Button) GetDriver() view.LiveDriver {
	return t
}

// SetClick registers the click handler.
func (t *Button) SetClick(fx func(c *Button, data interface{})) *Button {
	t.Events["Click"] = fx
	return t
}
