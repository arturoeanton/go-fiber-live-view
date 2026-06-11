package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// ProgressBar shows progress as a horizontal bar. Max defaults to 100.
type ProgressBar struct {
	*view.ComponentDriver[*ProgressBar]
	Value     int
	Max       int
	ShowLabel bool
}

func (t *ProgressBar) GetDriver() view.LiveDriver {
	return t
}

func (t *ProgressBar) Start() {
	if t.Max == 0 {
		t.Max = 100
	}
	t.Commit()
}

func (t *ProgressBar) GetTemplate() string {
	return `<div id="{{.IdComponent}}">
	<div class="lv-progress"><div class="lv-progress-bar" style="width: {{.Percent}}%"></div></div>
	{{if .ShowLabel}}<span class="lv-progress-label">{{.Value}} / {{.Max}}</span>{{end}}
</div>`
}

// Percent returns the progress as 0-100, used by the template.
func (t *ProgressBar) Percent() int {
	if t.Max <= 0 {
		return 0
	}
	p := t.Value * 100 / t.Max
	if p > 100 {
		p = 100
	}
	if p < 0 {
		p = 0
	}
	return p
}

// SetProgress updates the value and re-renders.
func (t *ProgressBar) SetProgress(value int) *ProgressBar {
	t.Value = value
	t.Commit()
	return t
}
