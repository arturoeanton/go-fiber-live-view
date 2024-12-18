package view

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"net/http"
	"sync"
	"text/template"
)

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
	muDriver     sync.Mutex
	muChannelIn  sync.Mutex
	templateBase string = `
<html lang="{{.Lang}}">
	<head>
		<title>{{.Title}}</title>
		{{.HeadCode}}
		<style>
			{{.Css}}
		</style>
		<meta charset="utf-8"/>
        <script src="assets/wasm_exec.js"></script>
	</head>
    <body>
		<div id="content"> 
		</div>
		<script>
		const go = new Go();
		WebAssembly.instantiateStreaming(fetch("assets/json.wasm"), go.importObject).then((result) => {
			go.run(result.instance);
		});
		</script>
		{{.AfterCode}}
    </body>
</html>
`
)

// Register this method to register in router of Echo page and websocket
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

	pc.Router.Get("/assets/:file", func(c *fiber.Ctx) error {
		file := "../../liveview/assets/" + c.Params("file")

		if Exists(file) {
			if c.Params("file") == "json.wasm" {
				c.Set("Content-Type", "application/wasm")
			}
			if c.Params("file") == "wasm_exec.js" {
				c.Set("Content-Type", "application/javascript")
			}

			content, _ := FileToString(file)
			return c.SendString(content)
		}
		return c.SendStatus(http.StatusNotFound)
	})

	pc.Router.Get(pc.Path, func(c *fiber.Ctx) error {
		t := template.Must(template.New("page_control").Parse(templateBase))
		buf := new(bytes.Buffer)
		_ = t.Execute(buf, pc)
		c.Set("Content-Type", "text/html; charset=utf-8")
		e := c.SendString(buf.String())
		if e != nil {
			fmt.Println(e)
		}
		return nil
	})

	pc.Router.Get(pc.Path+"ws_goliveview", websocket.New(func(conn *websocket.Conn) {

		content := fx()

		// Cleanup y lógica de cierre
		defer func() {
			// Eliminar el layout del mapa global
			func() {
				id := content.GetIDComponet()
				DeleteLayout(id)
			}()

			// Ejecutar el handler de destrucción si existe
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Println("Layout has not HandlerEventDestroy method defined", r)
					}
				}()

				//Destroy component
				(content.GetComponet().(*Layout)).HandlerEventDestroy(content.GetIDComponet())
				(content.GetComponet().(*Layout)).HandlerInternalDestroy()
			}()

			fmt.Println("Delete Layout:", content.GetIDComponet())
		}()

		// Montar componentes
		for _, v := range componentsDrivers {
			content.Mount(v.GetComponet())
		}
		content.SetID("content")

		// Canales

		drivers := make(map[string]LiveDriver)
		channelIn := make(map[string](chan interface{}))

		// Iniciar driver en goroutine
		go func() {
			defer HandleRecover()
			muChannelIn.Lock()
			defer muChannelIn.Unlock()
			muDriver.Lock()
			defer muDriver.Unlock()
			content.StartDriver(conn, &drivers, &channelIn)
		}()

		// Leer mensajes del cliente
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("Error leyendo mensaje:", err)
				break
			}

			var data map[string]interface{}
			if err := json.Unmarshal(msg, &data); err != nil {
				fmt.Println("Error al deserializar JSON:", err)
				continue
			}

			// Procesar mensajes
			if mtype, ok := data["type"]; ok {
				param := data["data"]
				if mtype == "data" {
					func() {
						muDriver.Lock()
						defer muDriver.Unlock()
						drivers[data["id"].(string)].ExecuteEvent(data["event"].(string), param)
					}()
				}
				if mtype == "get" {
					func() {
						muChannelIn.Lock()
						defer muChannelIn.Unlock()
						channelIn[data["id_ret"].(string)] <- param
					}()

				}
			}
		}

	}))
}
