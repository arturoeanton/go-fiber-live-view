package main

import (
	"fmt"
	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func main() {
	app := fiber.New()

	home := view.PageControl{
		Title:  "Home",
		Lang:   "en",
		Path:   "/",
		Router: app,
	}

	home.Register(func() view.LiveDriver {
		id := uuid.NewString()

		button1 := view.New("button1", &components.Button{Caption: "Sum 1"})
		text1 := view.New("text1", &components.InputText{})

		text1.Events["KeyUp"] = func(text1 *components.InputText, data interface{}) {
			text1.FillValueById("div_text_result", data.(string))
		}

		button1.Events["Click"] = func(button *components.Button, data interface{}) {
			button.I++
			text := button.GetElementById("text1")
			button.FillValueById("span_result", fmt.Sprint(button.I)+" -> "+text)
			button.EvalScript("console.log(1)")
		}
		layout1 := view.NewLayout("home"+id, `
		{{ mount "text1"}}
		<div id="div_text_result"></div>
		<div>
			{{mount "button1"}}
		</div>
		<div>
			<span id="span_result"></span>
		</div>
		`)
		return layout1

	})

	app.Listen(":3000")
}
