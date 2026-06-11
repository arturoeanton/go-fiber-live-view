package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// Modal is a dialog overlay. Open it with Show() and close it with Hide();
// the close button and a click outside the dialog also close it.
// Content accepts raw HTML.
type Modal struct {
	*view.ComponentDriver[*Modal]
	Title   string
	Content string
	Open    bool
}

func (t *Modal) GetDriver() view.LiveDriver {
	return t
}

func (t *Modal) Start() {
	t.Commit()
}

func (t *Modal) GetTemplate() string {
	return `<div id="{{.IdComponent}}">
{{if .Open}}
	<div class="lv-modal-overlay" onclick="if(event.target===this){send_event('{{.IdComponent}}','Close')}">
		<div class="lv-modal">
			<div class="lv-modal-header">
				<span>{{.Title}}</span>
				<button class="lv-modal-close" onclick="send_event('{{.IdComponent}}','Close')">&times;</button>
			</div>
			<div class="lv-modal-body">{{.Content}}</div>
		</div>
	</div>
{{end}}
</div>`
}

// Show opens the modal.
func (t *Modal) Show() {
	t.Open = true
	t.Commit()
}

// Hide closes the modal.
func (t *Modal) Hide() {
	t.Open = false
	t.Commit()
}

// Close handles the default close event.
func (t *Modal) Close(data interface{}) {
	t.Hide()
}

func (t *Modal) SetClose(fx func(c *Modal, data interface{})) *Modal {
	t.Events["Close"] = fx
	return t
}
