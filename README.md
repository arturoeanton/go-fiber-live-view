# Go Fiber LiveView

**Phoenix LiveView, pero para Go.** Construí aplicaciones web interactivas en
tiempo real escribiendo solo Go: el estado vive en el servidor, los eventos
viajan por WebSocket y el DOM se actualiza solo. Sin escribir JavaScript y
casi sin escribir HTML gracias a la biblioteca de componentes incluida.

Construido sobre [Fiber v3](https://github.com/gofiber/fiber) (la última
versión del framework web más rápido de Go) y un cliente WebAssembly
compilado desde Go.

```go
home.Register(func() view.LiveDriver {
    count := 0
    view.New("btn", &components.Button{Caption: "+1"}).
        SetClick(func(b *components.Button, data interface{}) {
            count++
            b.FillValueById("result", fmt.Sprint(count))
        })
    return view.NewLayout("home-"+uuid.NewString(), `
        <div class="lv-container">{{mount "btn"}} <span id="result">0</span></div>`)
})
```

## Características

- ⚡ **Tiempo real sin JavaScript**: eventos y render por WebSocket, estilo Phoenix LiveView
- 🧩 **16 componentes listos para usar**: formularios, tablas, modales, tabs… casi no escribís HTML
- 🎨 **Tema CSS incluido**: las páginas se ven bien sin tocar una línea de estilo
- 🚀 **Fiber v3**: la última versión del framework HTTP más rápido de Go
- 📡 **Broadcast integrado**: `SendToAllLayouts` para apps colaborativas (chat, dashboards, todo compartido)
- 📦 **Assets embebidos**: el wasm, el runtime y el CSS van dentro del binario; `go run` y listo
- 🔄 **Una goroutine por sesión**: miles de sesiones concurrentes por nodo

## Performance

El core está optimizado para alta concurrencia:

- Caché de templates parseados (no se re-parsea en cada render)
- Pool de buffers (`sync.Pool`) para el render
- Mutex de escritura **por conexión** (las sesiones no se bloquean entre sí)
- Registro de componentes aislado por conexión (sin carreras entre sesiones)
- Consultas servidor→browser (`GetValue`, etc.) con timeout: nunca quedan goroutines colgadas

## Instalación

```bash
git clone https://github.com/arturoeanton/go-fiber-live-view.git
cd go-fiber-live-view/examples
go run ./01_counter   # http://localhost:3001
```

No hace falta compilar nada más: el cliente WebAssembly ya viene compilado y
embebido en la librería. Solo necesitás **Go 1.24+**. Si modificás el cliente
(`wasm/`), regeneralo con `./build_wasm.sh`.

## Ejemplos

Todos en la carpeta [`examples/`](examples/), cada uno en su puerto — se
pueden correr todos a la vez:

| Comando | Demo | Qué muestra |
|---|---|---|
| `go run ./01_counter` | Contador | Hola mundo: estado en el servidor |
| `go run ./02_clock` | Relojes | Push del servidor sin interacción |
| `go run ./03_gallery` | Galería | **Todos los componentes** con eventos |
| `go run ./04_todo` | Todo colaborativo | Estado compartido entre browsers |
| `go run ./05_dashboard` | Dashboard | Métricas en vivo cada segundo |
| `go run ./06_chat` | 💬 **LiveChat** | Salas, privados, typing, unread, historial |

El chat (`06_chat`) es el ejemplo estrella: salas múltiples con contadores de
no-leídos, lista de usuarios online, mensajes privados (`@nick hola` o click
en un usuario), indicador "está escribiendo…", historial con timestamps,
emojis y mensajes de sistema. Todo renderizado desde Go.

## Componentes incluidos

```go
import "github.com/arturoeanton/go-fiber-live-view/liveview/components"
```

| Componente | Descripción | Eventos |
|---|---|---|
| `Button` | Botón con variantes (primary, secondary, success, danger, ghost) | `Click` |
| `InputText` | Input con label, placeholder y tipo | `Change`, `KeyUp`, `Enter`, `Blur` |
| `TextArea` | Texto multilínea | `Change`, `KeyUp` |
| `Select` | Dropdown con opciones | `Change` |
| `Checkbox` | Casilla con label | `Change` |
| `RadioGroup` | Grupo de radios | `Change` |
| `Table` | Tabla con headers y filas clickeables | `RowClick` |
| `List` | Lista clickeable | `ItemClick` |
| `Tabs` | Pestañas con paneles | `SelectTab` |
| `Modal` | Diálogo controlado desde el servidor (`Show`/`Hide`) | `Close` |
| `Card` | Contenedor con header/footer | — |
| `Alert` | Mensajes info/success/warning/danger, descartables | `Dismiss` |
| `ProgressBar` | Barra de progreso (`SetProgress`) | — |
| `Badge` | Pill de estado (`SetBadge`) | — |
| `NavBar` | Barra de navegación | `ItemClick` |
| `Spinner` | Indicador de carga (`Show`/`Hide`) | — |
| `Clock` | Hora del servidor en vivo, formato e intervalo configurables | — |

Todos usan el tema `liveview.css` embebido (personalizable con variables CSS
`--lv-*`).

## Crear tu propio componente

```go
type Counter struct {
    *view.ComponentDriver[*Counter]
    Count int
}

func (c *Counter) Start()                      { c.Commit() }
func (c *Counter) GetDriver() view.LiveDriver  { return c }
func (c *Counter) GetTemplate() string {
    return `<div id="{{.IdComponent}}">
        <button class="lv-btn" onclick="send_event(this.parentElement.id,'Inc')">+</button>
        <b>{{.Count}}</b>
    </div>`
}

// Los métodos exportados son manejadores de eventos automáticamente.
func (c *Counter) Inc(data interface{}) {
    c.Count++
    c.Commit() // re-renderiza y empuja el HTML al browser
}
```

Registralo con `view.New("mi_contador", &Counter{})` y montalo en el layout
con `{{mount "mi_contador"}}`.

## API esencial

```go
// Página
page := view.PageControl{Title: "Mi App", Path: "/", Router: app}
page.Register(func() view.LiveDriver { ... }) // corre una vez por conexión

// Layout (el HTML de la página); cada elemento con id es manipulable desde Go
doc := view.NewLayout("id-único-por-sesión", `<div id="zona">{{mount "comp"}}</div>`)
doc.GetDriverById("zona").FillValue("<b>html</b>")  // innerHTML
doc.GetDriverById("zona").SetStyle("color:red")

// Tiempo real
doc.Component.SetHandlerEventIn(func(data interface{}) { ... })  // recibir broadcasts
doc.Component.SetHandlerEventTime(time.Second, func() { ... })   // tick periódico
doc.Component.SetHandlerEventDestroy(func(id string) { ... })    // cleanup al desconectar
view.SendToAllLayouts("MSG")            // broadcast a todas las sesiones
view.SendToLayouts("MSG", id1, id2)     // broadcast dirigido
```

## Estructura del proyecto

```
/
├── liveview/           # La librería
│   ├── view/           # Core: drivers, layouts, páginas, websocket
│   ├── components/     # Biblioteca de componentes
│   └── assets/         # wasm + runtime + css (embebidos con go:embed)
├── wasm/               # Código fuente del cliente WebAssembly
├── examples/           # 6 ejemplos listos para correr
└── build_wasm.sh       # Regenera el cliente wasm
```

## Notas de seguridad

Los templates usan `text/template`: el contenido dinámico **no se escapa
automáticamente**. Cuando muestres entrada de usuarios (como hace el chat),
escapala con `html.EscapeString`. No expongas `EvalScript` a datos sin
sanitizar.

## Contribuir

PRs bienvenidos. Áreas con más impacto: tests, diffing de DOM (morphdom),
más componentes, reconexión con estado, documentación.

## Licencia

BSD 3 cláusulas. Ver `LICENSE`.

## Reconocimientos

Inspirado en [Phoenix LiveView](https://github.com/phoenixframework/phoenix_live_view)
del ecosistema Elixir, adaptado a la potencia y simplicidad de Go.
