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
	UUID                string
	Html                string
	CloseChan           chan bool
	HandlerEventIn      *func(data interface{})
	HandlerEventTime    *func()
	HandlerEventDestroy *func(id string)
	HandlerFirstTime    *func()
	IntervalEventTime   time.Duration
}

func (t *Layout) GetDriver() LiveDriver {
	return t
}

var (
	MuLayout sync.Mutex         = sync.Mutex{}
	Layaouts map[string]*Layout = make(map[string]*Layout)
)

func DeleteLayout(uid string) {
	MuLayout.Lock()
	defer MuLayout.Unlock()
	if layout, ok := Layaouts[uid]; ok {

		// Cerrar el canal CloseChan de forma segura
		select {
		case <-layout.CloseChan:
			// Ya está cerrado, no hacer nada
		default:
			close(layout.CloseChan)
			fmt.Println("Canal CloseChan cerrado correctamente")
		}

		// Eliminar el layout del mapa
		delete(Layaouts, uid)
		fmt.Println("Layout eliminado:", uid)
	}
}

func SendToAllLayouts(msg interface{}) {
	MuLayout.Lock() // Lectura segura
	layoutsCopy := make([]*Layout, 0, len(Layaouts))
	for _, v := range Layaouts {
		layoutsCopy = append(layoutsCopy, v)
	}
	MuLayout.Unlock() // Liberar bloqueo antes de operar en goroutines

	for _, v := range layoutsCopy {
		(*v.HandlerEventIn)(msg)
	}
}
func SendToLayouts(msg interface{}, uuids ...string) {
	layoutsCopy := make([]*Layout, 0, len(uuids))
	func() {
		MuLayout.Lock()
		defer MuLayout.Unlock()
		for _, uid := range uuids {
			if v, ok := Layaouts[uid]; ok {
				layoutsCopy = append(layoutsCopy, v)
			}
		}
	}()

	for _, v := range layoutsCopy {
		(*v.HandlerEventIn)(msg)
	}
}

func NewLayout(uid string, paramHtml string) *ComponentDriver[*Layout] {
	if Exists(paramHtml) {
		paramHtml, _ = FileToString(paramHtml)
	}
	c := &Layout{UUID: uid, Html: paramHtml, CloseChan: make(chan bool), IntervalEventTime: time.Hour * 24}
	func() {
		MuLayout.Lock()
		defer MuLayout.Unlock()
		Layaouts[uid] = c
	}()

	fmt.Println("NewLayout", uid)
	c.ComponentDriver = NewDriver(uid, c)

	go func() {
		firstTime := true
		tickerFirstTime := time.NewTicker(250 * time.Millisecond)
		tickerEventTime := time.NewTicker(c.IntervalEventTime)

		defer func() {
			tickerFirstTime.Stop()
			tickerEventTime.Stop()
		}()

		for {
			select {
			case <-c.CloseChan:
				fmt.Println("Cerrando Layout:", c.UUID)
				firstTime = true
				return

			case <-tickerFirstTime.C:
				if firstTime {
					firstTime = false
					if c.HandlerFirstTime != nil {
						(*c.HandlerFirstTime)()
					} else {
						SendToAllLayouts("FIRST_TIME")
					}
				}
			case <-tickerEventTime.C:
				if c.HandlerEventTime != nil {
					(*c.HandlerEventTime)()
				}
			}
		}
	}()

	doc, err := html.Parse(strings.NewReader(paramHtml))
	if err != nil {
		fmt.Println(err)
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
	t.HandlerFirstTime = &fx
}
func (t *Layout) SetHandlerEventIn(fx func(data interface{})) {
	t.HandlerEventIn = &fx
}

func (t *Layout) SetHandlerEventTime(IntervalEventTime time.Duration, fx func()) {
	t.IntervalEventTime = IntervalEventTime
	t.HandlerEventTime = &fx
}

func (t *Layout) SetHandlerEventDestroy(fx func(id string)) {
	t.HandlerEventDestroy = &fx
}
func (t *Layout) Start() {
	t.Commit()
}

func (t *Layout) GetTemplate() string {
	return t.Html
}
