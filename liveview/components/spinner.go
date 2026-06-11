package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// Spinner is an animated loading indicator; hide and show it from handlers.
type Spinner struct {
	*view.ComponentDriver[*Spinner]
	Hidden bool
}

func (t *Spinner) GetDriver() view.LiveDriver {
	return t
}

func (t *Spinner) Start() {
	t.Commit()
}

func (t *Spinner) GetTemplate() string {
	return `<span id="{{.IdComponent}}">{{if not .Hidden}}<span class="lv-spinner"></span>{{end}}</span>`
}

// Show makes the spinner visible.
func (t *Spinner) Show() {
	t.Hidden = false
	t.Commit()
}

// Hide makes the spinner invisible.
func (t *Spinner) Hide() {
	t.Hidden = true
	t.Commit()
}
