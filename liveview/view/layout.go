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
}

func (t *Layout) GetDriver() LiveDriver {
	return t
}

var (
	MuLayout sync.RWMutex = sync.RWMutex{}

	Layaouts map[string]*Layout = make(map[string]*Layout)
)

func DeleteLayout(uid string) {
	MuLayout.Lock()
	defer MuLayout.Unlock()
	if _, ok := Layaouts[uid]; ok {
		delete(Layaouts, uid)
		fmt.Println("Layout eliminado:", uid)
	}
}

func SendToAllLayouts(msg interface{}) {
	MuLayout.RLock() // Lectura concurrente segura
	layoutsCopy := make([]*Layout, 0, len(Layaouts))
	for _, v := range Layaouts {
		layoutsCopy = append(layoutsCopy, v)
	}
	MuLayout.RUnlock() // Liberar el bloqueo antes de operar

	for _, v := range layoutsCopy {

		v.HandlerEventIn(msg)

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
		v.HandlerEventIn(msg)
	}
}

func NewLayout(uid string, paramHtml string) *ComponentDriver[*Layout] {
	quit := make(chan struct{})
	// Verificar si el layout ya existe
	MuLayout.RLock()
	if existingLayout, exists := Layaouts[uid]; exists {
		MuLayout.RUnlock()
		fmt.Println("Layout ya existe:", uid)
		return existingLayout.ComponentDriver
	}
	MuLayout.RUnlock()

	// Si no existe, crear un nuevo layout
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
		HandlerEventIn: func(data interface{}) {

		},
		HandlerEventDestroy: func(id string) {

		},
		HandlerInternalDestroy: func() {
			close(quit)
		},
	}

	// Guardar el nuevo layout en el mapa de layouts
	MuLayout.Lock()
	Layaouts[uid] = c
	MuLayout.Unlock()

	fmt.Println("NewLayout", uid)
	c.ComponentDriver = NewDriver(uid, c)

	// Iniciar la goroutine para eventos
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
			case <-quit:
				return
			case <-tickerFirstTime.C:
				if firstTime {
					firstTime = false
					if c.HandlerFirstTime != nil {
						c.HandlerFirstTime()
					} else {
						SendToAllLayouts("FIRST_TIME")
					}
				}
			case <-tickerEventTime.C:
				c.HandlerEventTime()

			}
		}
	}()

	// Parsear HTML para detectar elementos con ID
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

func (t *Layout) SetHandlerEventTime(IntervalEventTime time.Duration, fx func()) {
	t.IntervalEventTime = IntervalEventTime
	t.HandlerEventTime = fx
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
