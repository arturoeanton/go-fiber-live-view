package view

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"text/template"

	"github.com/arturoeanton/go-fiber-live-view/liveview/assets"
	"github.com/gofiber/fiber/v3"
)

// PageControl registers a LiveView page (HTML shell + websocket endpoint)
// on a Fiber router.
type PageControl struct {
	Path      string
	Title     string
	HeadCode  string
	Lang      string
	Css       string
	LiveJs    string
	AfterCode string
	Router    fiber.Router
	Debug     bool
}

var (
	muRegister       sync.Mutex
	assetsRegistered sync.Map // fiber.Router -> struct{}

	templateBase string = `<!DOCTYPE html>
<html lang="{{.Lang}}">
	<head>
		<title>{{.Title}}</title>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1"/>
		<link rel="stylesheet" href="/assets/liveview.css"/>
		{{.HeadCode}}
		<style>
			{{.Css}}
		</style>
		<script src="/assets/wasm_exec.js"></script>
	</head>
	<body>
		<div id="content"></div>
		<script>
		const go = new Go();
		WebAssembly.instantiateStreaming(fetch("/assets/json.wasm"), go.importObject).then((result) => {
			go.run(result.instance);
		});
		</script>
		{{.AfterCode}}
	</body>
</html>
`
)

// Register registers the page route and his websocket endpoint. The fx
// factory runs once per browser connection and returns the page layout.
func (pc *PageControl) Register(fx func() LiveDriver) {
	if Exists(pc.AfterCode) {
		pc.AfterCode, _ = FileToString(pc.AfterCode)
	}
	if Exists(pc.HeadCode) {
		pc.HeadCode, _ = FileToString(pc.HeadCode)
	}
	if pc.Lang == "" {
		pc.Lang = "en"
	}
	if Exists("live.js") {
		pc.LiveJs, _ = FileToString("live.js")
	}

	// Static assets (wasm runtime, client and css) are embedded in the
	// library binary, so pages work from any working directory.
	if _, done := assetsRegistered.LoadOrStore(pc.Router, struct{}{}); !done {
		pc.Router.Get("/assets/:file", func(c fiber.Ctx) error {
			name := c.Params("file")
			content, err := assets.FS.ReadFile(name)
			if err != nil {
				return c.SendStatus(fiber.StatusNotFound)
			}
			switch {
			case strings.HasSuffix(name, ".wasm"):
				c.Set("Content-Type", "application/wasm")
			case strings.HasSuffix(name, ".js"):
				c.Set("Content-Type", "application/javascript")
			case strings.HasSuffix(name, ".css"):
				c.Set("Content-Type", "text/css")
			}
			c.Set("Cache-Control", "public, max-age=86400")
			return c.Send(content)
		})
	}

	pageTemplate := template.Must(template.New("page_control").Parse(templateBase))
	pc.Router.Get(pc.Path, func(c fiber.Ctx) error {
		buf := new(bytes.Buffer)
		if err := pageTemplate.Execute(buf, pc); err != nil {
			return err
		}
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(buf.String())
	})

	pc.Router.Get(pc.Path+"ws_goliveview", NewWebSocketHandler(func(conn *Conn) {
		// Build this connection's component tree. The global registry is
		// reset per connection (under lock) so concurrent sessions never
		// share or overwrite each other's components.
		muRegister.Lock()
		componentsDrivers = make(map[string]LiveDriver)
		content := fx()
		pageComponents := componentsDrivers
		muRegister.Unlock()

		defer func() {
			unsubscribeConn(conn)
			DeleteLayout(content.GetIDComponet())
			func() {
				defer HandleRecoverPass()
				layout := content.GetComponet().(*Layout)
				layout.HandlerEventDestroy(content.GetIDComponet())
				layout.HandlerInternalDestroy()
			}()
			if pc.Debug {
				log.Println("liveview: session closed:", content.GetIDComponet())
			}
		}()

		for _, v := range pageComponents {
			content.Mount(v.GetComponet())
		}
		content.SetID("content")

		drivers := make(map[string]LiveDriver)
		channelIn := make(map[string]chan interface{})

		go func() {
			defer HandleRecover()
			content.StartDriver(conn, &drivers, &channelIn)
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				if pc.Debug {
					log.Println("liveview: bad message:", err)
				}
				continue
			}

			mtype, _ := data["type"].(string)
			switch mtype {
			case "data":
				id, _ := data["id"].(string)
				event, _ := data["event"].(string)
				mu.Lock()
				driver := drivers[id]
				mu.Unlock()
				if driver != nil {
					// ExecuteEvent dispatches in his own goroutine, the
					// read-loop is never blocked by slow handlers.
					driver.ExecuteEvent(event, data["data"])
				}
			case "get":
				idRet, _ := data["id_ret"].(string)
				muChannel.Lock()
				ch := channelIn[idRet]
				muChannel.Unlock()
				if ch != nil {
					select {
					case ch <- data["data"]:
					default:
					}
				}
			case "crdt":
				handleCrdt(conn, msg)
			}
		}
		conn.Close()
	}))
}
