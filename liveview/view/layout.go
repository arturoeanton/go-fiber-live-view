package view

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Layout struct {
	*ComponentDriver[*Layout]
	UUID                   string
	Html                   string
	HandlerEventIn         func(data interface{})
	HandlerEventTime       func()
	HandlerEventDestroy    func(id string)
	HandlerInternalDestroy func()
	HandlerFirstTime       func()
	IntervalEventTime      time.Duration

	tickerEventTime *time.Ticker
}

func (t *Layout) GetDriver() LiveDriver {
	return t
}

var (
	MuLayout sync.RWMutex = sync.RWMutex{}

	// Layouts holds every live session layout, keyed by his uuid.
	Layouts map[string]*Layout = make(map[string]*Layout)

	// Layaouts is a deprecated alias of Layouts, kept for backward compatibility.
	Layaouts = Layouts
)

func DeleteLayout(uid string) {
	MuLayout.Lock()
	defer MuLayout.Unlock()
	delete(Layouts, uid)
}

// SendToAllLayouts delivers msg to the HandlerEventIn of every live session.
func SendToAllLayouts(msg interface{}) {
	MuLayout.RLock()
	layoutsCopy := make([]*Layout, 0, len(Layouts))
	for _, v := range Layouts {
		layoutsCopy = append(layoutsCopy, v)
	}
	MuLayout.RUnlock()

	for _, v := range layoutsCopy {
		v.sendEventIn(msg)
	}
}

// SendToLayouts delivers msg to the sessions with the given uuids.
func SendToLayouts(msg interface{}, uuids ...string) {
	layoutsCopy := make([]*Layout, 0, len(uuids))
	MuLayout.RLock()
	for _, uid := range uuids {
		if v, ok := Layouts[uid]; ok {
			layoutsCopy = append(layoutsCopy, v)
		}
	}
	MuLayout.RUnlock()

	for _, v := range layoutsCopy {
		v.sendEventIn(msg)
	}
}

func (t *Layout) sendEventIn(msg interface{}) {
	defer HandleRecover()
	if t.HandlerEventIn != nil {
		t.HandlerEventIn(msg)
	}
}

func NewLayout(uid string, paramHtml string) *ComponentDriver[*Layout] {
	quit := make(chan struct{})
	MuLayout.RLock()
	if existingLayout, exists := Layouts[uid]; exists {
		MuLayout.RUnlock()
		return existingLayout.ComponentDriver
	}
	MuLayout.RUnlock()

	if Exists(paramHtml) {
		paramHtml, _ = FileToString(paramHtml)
	}

	c := &Layout{
		UUID:              uid,
		Html:              paramHtml,
		IntervalEventTime: time.Hour * 24,
		HandlerFirstTime: func() {
			SendToLayouts("FIRST_TIME", uid)
		},
		HandlerEventIn:      func(data interface{}) {},
		HandlerEventDestroy: func(id string) {},
		HandlerInternalDestroy: func() {
			defer HandleRecoverPass()
			close(quit)
		},
	}
	c.tickerEventTime = time.NewTicker(c.IntervalEventTime)

	MuLayout.Lock()
	Layouts[uid] = c
	MuLayout.Unlock()

	c.ComponentDriver = NewDriver(uid, c)

	go func() {
		tickerFirstTime := time.NewTicker(250 * time.Millisecond)
		defer func() {
			tickerFirstTime.Stop()
			c.tickerEventTime.Stop()
		}()

		firstTime := true
		for {
			select {
			case <-quit:
				return
			case <-tickerFirstTime.C:
				if firstTime {
					firstTime = false
					tickerFirstTime.Stop()
					if c.HandlerFirstTime != nil {
						func() {
							defer HandleRecover()
							c.HandlerFirstTime()
						}()
					} else {
						SendToAllLayouts("FIRST_TIME")
					}
				}
			case <-c.tickerEventTime.C:
				if c.HandlerEventTime != nil {
					func() {
						defer HandleRecover()
						c.HandlerEventTime()
					}()
				}
			}
		}
	}()

	// Every element with an id inside the layout gets a None driver, so
	// handlers can target it with GetDriverById without extra boilerplate.
	doc, err := html.Parse(strings.NewReader(paramHtml))
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return c.ComponentDriver
	}
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == "id" {
					Join(a.Val)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return c.ComponentDriver
}

func (t *Layout) SetHandlerFirstTime(fx func()) {
	t.HandlerFirstTime = fx
}

func (t *Layout) SetHandlerEventIn(fx func(data interface{})) {
	t.HandlerEventIn = fx
}

// SetHandlerEventTime makes fx run every IntervalEventTime while the session
// is alive. It can be called at any moment; the interval is applied at once.
func (t *Layout) SetHandlerEventTime(IntervalEventTime time.Duration, fx func()) {
	t.IntervalEventTime = IntervalEventTime
	t.HandlerEventTime = fx
	t.tickerEventTime.Reset(IntervalEventTime)
}

func (t *Layout) SetHandlerEventDestroy(fx func(id string)) {
	t.HandlerEventDestroy = fx
}

func (t *Layout) Start() {
	t.Commit()
}

func (t *Layout) GetTemplate() string {
	return t.Html
}
