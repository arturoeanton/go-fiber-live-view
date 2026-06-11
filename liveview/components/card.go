package components

import "github.com/arturoeanton/go-fiber-live-view/liveview/view"

// Card is a content container with optional header and footer.
// Body and Footer accept raw HTML.
type Card struct {
	*view.ComponentDriver[*Card]
	Title  string
	Body   string
	Footer string
}

func (t *Card) GetDriver() view.LiveDriver {
	return t
}

func (t *Card) Start() {
	t.Commit()
}

func (t *Card) GetTemplate() string {
	return `<div class="lv-card" id="{{.IdComponent}}">
	{{if .Title}}<div class="lv-card-header">{{.Title}}</div>{{end}}
	<div class="lv-card-body">{{.Body}}</div>
	{{if .Footer}}<div class="lv-card-footer">{{.Footer}}</div>{{end}}
</div>`
}

// SetBody replaces the body HTML and re-renders.
func (t *Card) SetBody(body string) *Card {
	t.Body = body
	t.Commit()
	return t
}
