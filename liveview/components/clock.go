package components

import (
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"time"
)

type Clock struct {
	*view.ComponentDriver[*Clock]
	ActualTime string
}

func (t *Clock) GetDriver() view.LiveDriver {
	return t
}
func (t *Clock) Start() {
	go func() {
		for {
			t.ActualTime = time.Now().Format(time.RFC3339Nano)
			t.Commit()
			time.Sleep((time.Second * 1) / 60)
		}
	}()
}

func (t *Clock) GetTemplate() string {
	return `
		<div  id="{{.IdComponent}}"  >
			<span>Time: {{ .ActualTime }}</span>
		</div>
	`
}
