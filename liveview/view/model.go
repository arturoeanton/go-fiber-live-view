package view

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
)

var (
	componentsDrivers map[string]LiveDriver = make(map[string]LiveDriver)
	mu                sync.Mutex
	muChannel         sync.Mutex

	// templateCache avoids re-parsing the component template on every Commit.
	templateCache sync.Map // template source -> *template.Template
	bufPool       = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}

	// getTimeout bounds how long a server->browser query (GetValue, GetHTML,
	// ...) waits before giving up, so handlers never leak goroutines when the
	// browser disconnects mid-request.
	getTimeout = 5 * time.Second
)

// Component it is interface for implement one component
type Component interface {
	// GetTemplate return html template for render with component in the {{.}}
	GetTemplate() string
	// Start it will invoke in the mount time
	Start()
	GetDriver() LiveDriver
}

type LiveDriver interface {
	GetID() string
	SetID(string)
	StartDriver(*Conn, *map[string]LiveDriver, *map[string]chan interface{})
	GetIDComponet() string
	ExecuteEvent(name string, data interface{})

	GetComponet() Component
	Mount(component Component) LiveDriver
	MountWithStart(conn *Conn, id string, componentDriver LiveDriver) LiveDriver

	Commit()
	Remove(string)
	AddNode(string, string)
	FillValue(string)
	SetHTML(string)
	SetText(string)
	SetPropertie(string, interface{})
	SetValue(interface{})
	EvalScript(string)
	SetStyle(string)

	FillValueById(id string, value string)

	GetPropertie(string) string
	GetDriverById(id string) LiveDriver
	GetText() string
	GetHTML() string
	GetStyle(string) string
	GetValue() string
	GetElementById(string) string

	SetData(interface{})
}

func (cw *ComponentDriver[T]) SetData(data interface{}) {
	cw.Data = data
}

func (cw *ComponentDriver[T]) GetData() interface{} {
	return cw.Data
}

// ComponentDriver this is the driver for component, with this struct we can execute our methods in the web
type ComponentDriver[T Component] struct {
	Component         T
	id                string
	IdComponent       string
	Conn              *Conn
	componentsDrivers map[string]LiveDriver
	DriversPage       *map[string]LiveDriver
	channelIn         *map[string]chan interface{}
	// Events has rewrite of our implementings of  events, examples click, change, keyup, keydown, etc
	Events map[string]func(c T, data interface{})
	Data   interface{}

	evMu    sync.Mutex
	evQueue []eventCall
	evBusy  bool
}

type eventCall struct {
	name string
	data interface{}
}

func (cw *ComponentDriver[T]) SetEvent(name string, fx func(c T, data interface{})) {
	cw.Events[name] = fx
}

func (cw *ComponentDriver[T]) GetIDComponet() string {
	return cw.IdComponent
}

// Commit renders the component template and pushes the resulting HTML to the
// browser. Templates are parsed once and cached.
func (cw *ComponentDriver[T]) Commit() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in Commit:", r)
		}
	}()
	t := cw.template()
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if err := t.Execute(buf, cw.Component); err != nil {
		log.Println("liveview: error rendering", cw.IdComponent, ":", err)
		return
	}
	cw.FillValueById(cw.GetID(), buf.String())
}

func (cw *ComponentDriver[T]) template() *template.Template {
	src := cw.Component.GetTemplate()
	if t, ok := templateCache.Load(src); ok {
		return t.(*template.Template)
	}
	t := template.Must(template.New("component").Funcs(FuncMapTemplate).Parse(src))
	templateCache.Store(src, t)
	return t
}

func (cw *ComponentDriver[T]) StartDriver(conn *Conn, drivers *map[string]LiveDriver, channelIn *map[string]chan interface{}) {
	defer HandleRecover()
	cw.Conn = conn
	cw.DriversPage = drivers
	cw.channelIn = channelIn
	mu.Lock()
	(*drivers)[cw.GetIDComponet()] = cw
	mu.Unlock()
	cw.Component.Start()
	var wg sync.WaitGroup
	for _, c := range cw.componentsDrivers {
		wg.Add(1)
		go func(c LiveDriver) {
			defer HandleRecover()
			defer wg.Done()
			c.StartDriver(conn, drivers, channelIn)
		}(c)
	}
	wg.Wait()
}

// GetComponet return component of driver
func (cw *ComponentDriver[T]) GetComponet() Component {
	return cw.Component
}

// GetDriverById return driver of component by id
func (cw *ComponentDriver[T]) GetDriverById(id string) LiveDriver {
	if c, ok := cw.componentsDrivers["mount_span_"+id]; ok {
		return c
	}
	if c, ok := cw.componentsDrivers[id]; ok {
		return c
	}
	c := &None{}
	New(id, c)
	c.ComponentDriver.Conn = cw.Conn
	c.ComponentDriver.channelIn = cw.channelIn
	return c
}

// GetID return id of driver
func (cw *ComponentDriver[T]) GetID() string {
	return cw.id
}

// SetID set id of driver
func (cw *ComponentDriver[T]) SetID(id string) {
	cw.id = id
}

// Mount mount component in other component
func (cw *ComponentDriver[T]) Mount(component Component) LiveDriver {
	componentDriver := component.GetDriver()
	id := "mount_span_" + componentDriver.GetIDComponet()
	componentDriver.SetID(id)
	cw.componentsDrivers[id] = componentDriver
	return cw
}

// MountWithStart mount component in other component and start his driver
func (cw *ComponentDriver[T]) MountWithStart(conn *Conn, id string, componentDriver LiveDriver) LiveDriver {
	componentDriver.SetID(id)
	cw.Conn = conn
	cw.componentsDrivers[id] = componentDriver
	componentDriver.StartDriver(conn, cw.DriversPage, cw.channelIn)
	return cw
}

func Join(ids ...string) {
	for _, id := range ids {
		New(id, &None{})
	}
}

func New[T Component](id string, c T) T {
	NewDriver(id, c)
	componentDriver := c.GetDriver()
	idMount := "mount_span_" + componentDriver.GetIDComponet()
	componentDriver.SetID(idMount)
	mu.Lock()
	componentsDrivers[idMount] = componentDriver
	mu.Unlock()
	return c
}

func NewWithTemplate(id string, template string) *None {
	return New(id, &None{Template: template})
}

// Create Driver with component
func NewDriver[T Component](id string, c T) *ComponentDriver[T] {
	driver := newDriver(c)
	driver.IdComponent = id
	ps := reflect.ValueOf(c)
	field := ps.Elem().FieldByName("Id")
	if field.CanSet() {
		field.SetString(id)
	}
	field = ps.Elem().FieldByName("Driver")

	if field.CanSet() {
		field.Set(reflect.ValueOf(driver))
	} else {
		field = ps.Elem().FieldByName("ComponentDriver")
		if field.CanSet() {
			field.Set(reflect.ValueOf(driver))
		}
	}
	return driver
}

func newDriver[T Component](c T) *ComponentDriver[T] {
	driver := &ComponentDriver[T]{Component: c}
	driver.componentsDrivers = make(map[string]LiveDriver)
	driver.Events = make(map[string]func(T, interface{}))
	return driver
}

// ExecuteEvent runs an event handler asynchronously. Events of the same
// driver are processed strictly in arrival order (one FIFO mailbox per
// driver, like a LiveView process), so high-frequency sequences such as
// pointer down/move/up never interleave. The page read-loop is never
// blocked, and handlers can still call GetValue & friends safely.
func (cw *ComponentDriver[T]) ExecuteEvent(name string, data interface{}) {
	if cw == nil {
		return
	}
	cw.evMu.Lock()
	cw.evQueue = append(cw.evQueue, eventCall{name: name, data: data})
	if cw.evBusy {
		cw.evMu.Unlock()
		return
	}
	cw.evBusy = true
	cw.evMu.Unlock()
	go func() {
		for {
			cw.evMu.Lock()
			if len(cw.evQueue) == 0 {
				cw.evBusy = false
				cw.evMu.Unlock()
				return
			}
			ev := cw.evQueue[0]
			cw.evQueue = cw.evQueue[1:]
			cw.evMu.Unlock()
			cw.runEvent(ev.name, ev.data)
		}
	}()
}

func (cw *ComponentDriver[T]) runEvent(name string, data interface{}) {
	defer HandleRecover()
	if data == nil {
		data = make(map[string]interface{})
	}
	if cw.Events != nil {
		if fx, ok := cw.Events[name]; ok {
			fx(cw.Component, data)
			return
		}
	}
	func() {
		defer HandleRecoverPass()
		in := []reflect.Value{reflect.ValueOf(data)}
		reflect.ValueOf(cw.Component).MethodByName(name).Call(in)
	}()
}

// Remove removes the DOM node with the given id
func (cw *ComponentDriver[T]) Remove(id string) {
	cw.writeJSON(map[string]interface{}{"type": "remove", "id": id})
}

// AddNode add node to id
func (cw *ComponentDriver[T]) AddNode(id string, value string) {
	cw.writeJSON(map[string]interface{}{"type": "addNode", "id": id, "value": value})
}

// FillValueById sets innerHTML of the element with the given id
func (cw *ComponentDriver[T]) FillValueById(id string, value string) {
	cw.writeJSON(map[string]interface{}{"type": "fill", "id": id, "value": value})
}

// FillValue is same SetHTML
func (cw *ComponentDriver[T]) FillValue(value string) {
	cw.writeJSON(map[string]interface{}{"type": "fill", "id": cw.GetIDComponet(), "value": value})
}

// SetHTML is same FillValue :p haha, execute  document.getElementById("$id").innerHTML = $value
func (cw *ComponentDriver[T]) SetHTML(value string) {
	cw.writeJSON(map[string]interface{}{"type": "fill", "id": cw.GetIDComponet(), "value": value})
}

// SetText execute document.getElementById("$id").innerText = $value
func (cw *ComponentDriver[T]) SetText(value string) {
	cw.writeJSON(map[string]interface{}{"type": "text", "id": cw.GetIDComponet(), "value": value})
}

// SetPropertie execute  document.getElementById("$id")[$propertie] = $value
func (cw *ComponentDriver[T]) SetPropertie(propertie string, value interface{}) {
	cw.writeJSON(map[string]interface{}{"type": "propertie", "id": cw.GetIDComponet(), "propertie": propertie, "value": value})
}

// SetValue execute document.getElementById("$id").value = $value|
func (cw *ComponentDriver[T]) SetValue(value interface{}) {
	cw.writeJSON(map[string]interface{}{"type": "set", "id": cw.GetIDComponet(), "value": value})
}

// EvalScript execute eval($code);
func (cw *ComponentDriver[T]) EvalScript(code string) {
	cw.writeJSON(map[string]interface{}{"type": "script", "value": code})
}

// SetStyle execute  document.getElementById("$id").style.cssText = $style
func (cw *ComponentDriver[T]) SetStyle(style string) {
	cw.writeJSON(map[string]interface{}{"type": "style", "id": cw.GetIDComponet(), "value": style})
}

func (cw *ComponentDriver[T]) writeJSON(msg map[string]interface{}) {
	if cw.Conn == nil {
		return
	}
	if err := cw.Conn.WriteJSON(msg); err != nil && cw.Conn.IsOpen() {
		log.Println("liveview: write error:", err)
	}
}

// GetElementById same as GetValue
func (cw *ComponentDriver[T]) GetElementById(id string) string {
	return cw.get(id, "value", "")
}

// GetValue return document.getElementById("$id").value
func (cw *ComponentDriver[T]) GetValue() string {
	return cw.get(cw.GetIDComponet(), "value", "")
}

// GetStyle  return document.getElementById("$id").style["$propertie"]
func (cw *ComponentDriver[T]) GetStyle(propertie string) string {
	return cw.get(cw.GetIDComponet(), "style", propertie)
}

// GetHTML  return document.getElementById("$id").innerHTML
func (cw *ComponentDriver[T]) GetHTML() string {
	return cw.get(cw.GetIDComponet(), "html", "")
}

// GetText  return document.getElementById("$id").innerText
func (cw *ComponentDriver[T]) GetText() string {
	return cw.get(cw.GetIDComponet(), "text", "")
}

// GetPropertie return document.getElementById("$id")[$propertie]
func (cw *ComponentDriver[T]) GetPropertie(name string) string {
	return cw.get(cw.GetIDComponet(), "propertie", name)
}

// get asks the browser for a value and waits (bounded) for the response.
func (cw *ComponentDriver[T]) get(id string, subType string, value string) string {
	if cw.Conn == nil || cw.channelIn == nil {
		return ""
	}
	uid := uuid.NewString()
	ch := make(chan interface{}, 1)
	muChannel.Lock()
	(*cw.channelIn)[uid] = ch
	muChannel.Unlock()
	defer func() {
		muChannel.Lock()
		delete(*cw.channelIn, uid)
		muChannel.Unlock()
	}()
	if err := cw.Conn.WriteJSON(map[string]interface{}{"type": "get", "id": id, "value": value, "id_ret": uid, "sub_type": subType}); err != nil {
		return ""
	}
	select {
	case data := <-ch:
		if data != nil {
			return fmt.Sprint(data)
		}
		return ""
	case <-time.After(getTimeout):
		return ""
	}
}
