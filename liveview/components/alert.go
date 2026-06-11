package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// Alert shows a message box. Variants: "info" (default), "success",
// "warning", "danger". If Dismissible, the user can close it.
type Alert struct {
	*view.ComponentDriver[*Alert]
	Variant     string
	Message     string
	Dismissible bool
	Hidden      bool
}

func (t *Alert) GetDriver() view.LiveDriver {
	return t
}

func (t *Alert) Start() {
	if t.Variant == "" {
		t.Variant = "info"
	}
	t.Commit()
}

func (t *Alert) GetTemplate() string {
	return `<div id="{{.IdComponent}}">
{{if not .Hidden}}
	<div class="lv-alert lv-alert-{{.Variant}}">
		<span>{{.Message}}</span>
		{{if .Dismissible}}<button class="lv-alert-close" onclick="send_event('{{.IdComponent}}','Dismiss')">&times;</button>{{end}}
	</div>
{{end}}
</div>`
}

// Notify updates message and variant and shows the alert.
func (t *Alert) Notify(variant string, message string) {
	t.Variant = variant
	t.Message = message
	t.Hidden = false
	t.Commit()
}

// Dismiss handles the default close event.
func (t *Alert) Dismiss(data interface{}) {
	t.Hidden = true
	t.Commit()
}
