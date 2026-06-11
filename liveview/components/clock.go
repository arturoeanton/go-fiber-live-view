package components

import (
	"time"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
)

// Clock renders the server time, updated in real time. The update loop
// stops when the connection closes.
type Clock struct {
	*view.ComponentDriver[*Clock]
	ActualTime string
	Format     string
	Interval   time.Duration
}

func (t *Clock) GetDriver() view.LiveDriver {
	return t
}

func (t *Clock) Start() {
	if t.Format == "" {
		t.Format = "15:04:05.0"
	}
	if t.Interval == 0 {
		t.Interval = 100 * time.Millisecond
	}
	go func() {
		ticker := time.NewTicker(t.Interval)
		defer ticker.Stop()
		for range ticker.C {
			if t.Conn == nil || !t.Conn.IsOpen() {
				return
			}
			t.ActualTime = time.Now().Format(t.Format)
			t.Commit()
		}
	}()
}

func (t *Clock) GetTemplate() string {
	return `<span id="{{.IdComponent}}" class="lv-badge lv-badge-secondary">{{.ActualTime}}</span>`
}
