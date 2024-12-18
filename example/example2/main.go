package main

import (
	"fmt"
	"github.com/google/uuid"
	"strings"
	"sync"
	"time"

	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v2"
)

var (
	userMutex = &sync.Mutex{}                   // Mutex para proteger acceso concurrente
	bUser     = view.NewBiMap[string, string]() // Mapa bidireccional para gestionar usuarios y sus IDs
)

func main() {
	// Crear aplicación Fiber
	app := fiber.New()

	// Página principal
	home := view.PageControl{
		Title:  "Public and Private Chat",
		Lang:   "en",
		Path:   "/",
		Router: app,
	}

	// Agregar un usuario general (todos)
	bUser.Set("*", "Todos")

	// Registrar página LiveView
	home.Register(func() view.LiveDriver {
		// Crear Layout Principal
		id := uuid.NewString()
		document := view.NewLayout("layout"+id, `
			<div>Nickname: {{ mount "text_nickname" }} <span id="span_text_nickname"></span></div>
			<hr/>
			<div id="div_general_chat"></div>
			<hr/>
			<div>Message: {{ mount "text_msg" }} to {{ mount "select_to" }} {{ mount "button_send" }}</div>
			<hr/>
			<div id="div_status"></div>
		`)

		// Componentes
		view.New("text_nickname", &components.InputText{}).
			SetEvent("Change", func(this *components.InputText, data interface{}) {
				userMutex.Lock()
				defer userMutex.Unlock()

				// Verificar duplicación de nicknames
				if _, exists := bUser.GetByValue(this.GetValue()); exists {
					this.SetValue("")
					return
				}

				// Asignar nickname al usuario
				bUser.Set(document.Component.UUID, this.GetValue())

				// Actualizar UI
				spanNickname := document.GetDriverById("span_text_nickname")
				spanNickname.FillValue(fmt.Sprint(data))
				view.SendToAllLayouts("NEW_USER")
			})

		view.New("text_msg", &components.InputText{})
		view.NewWithTemplate("select_to", `
			<select onchange="send_event(this.id,'Change',this.value)" id="{{.IdComponent}}">
				{{range $index, $element := .GetDriver.Data}}
					<option value="{{$index}}">{{$element}}</option>
				{{end}}
			</select>`)

		view.New("button_send", &components.Button{Caption: "Send"}).
			SetClick(func(this *components.Button, data interface{}) {
				userMutex.Lock()
				defer userMutex.Unlock()

				if nickname, exists := bUser.Get(document.Component.UUID); exists {
					textMsg := document.GetDriverById("text_msg").GetValue()
					idTo := document.GetDriverById("select_to").GetValue()

					if textMsg == "" {
						return
					}

					// Enviar mensaje público o privado
					if idTo == "*" {
						view.SendToAllLayouts("MSG|" + fmt.Sprintf("%s [Public]: %s", nickname, textMsg))
					} else if userTo, exists := bUser.Get(idTo); exists {
						view.SendToLayouts("MSG|"+fmt.Sprintf("%s to %s [Private]: %s", nickname, userTo, textMsg), idTo, document.Component.UUID)
					}
				}
			})

		// Eventos del Componente Principal
		document.Component.SetHandlerEventIn(func(data interface{}) {
			msg := data.(string)
			if strings.HasPrefix(msg, "MSG|") {
				msg = strings.TrimPrefix(msg, "MSG|")
				chatBox := document.GetDriverById("div_general_chat")
				chatBox.FillValue(fmt.Sprint(chatBox.GetHTML(), msg, "<br/>"))
			}
			if msg == "NEW_USER" {
				// Actualizar lista de usuarios en el selector
				selectTo := document.GetDriverById("select_to")
				currentValue := selectTo.GetValue()
				selectTo.SetData(bUser.GetAll())
				selectTo.Commit()
				selectTo.SetValue(currentValue)
			}
		})

		// Estado de conexión (ping cada 5 segundos)
		document.Component.SetHandlerEventTime(time.Second*5, func() {
			statusDiv := document.GetDriverById("div_status")
			statusDiv.FillValue("online")
		})

		// Evento al destruir un layout
		document.Component.SetHandlerEventDestroy(func(id string) {
			userMutex.Lock()
			defer userMutex.Unlock()
			bUser.Delete(id)
			view.SendToAllLayouts("NEW_USER")
		})

		return document
	})

	// Escuchar en el puerto 3000
	fmt.Println("Server running on http://127.0.0.1:3000")
	app.Listen(":3000")
}
