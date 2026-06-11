package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// Badge is a small status pill. Variants: "" (primary), "secondary",
// "success", "danger", "warning".
type Badge struct {
	*view.ComponentDriver[*Badge]
	Text    string
	Variant string
}

func (t *Badge) GetDriver() view.LiveDriver {
	return t
}

func (t *Badge) Start() {
	t.Commit()
}

func (t *Badge) GetTemplate() string {
	return `<span class="lv-badge{{if .Variant}} lv-badge-{{.Variant}}{{end}}" id="{{.IdComponent}}">{{.Text}}</span>`
}

// SetBadge updates text and variant and re-renders.
func (t *Badge) SetBadge(text string, variant string) *Badge {
	t.Text = text
	t.Variant = variant
	t.Commit()
	return t
}
