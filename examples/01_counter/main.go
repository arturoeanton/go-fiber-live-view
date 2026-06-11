// Counter: the "hello world" of LiveView. Two buttons mutate server-side
// state and the DOM updates over websocket — zero JavaScript written.
package main

import (
	"fmt"
	"log"

	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func main() {
	app := fiber.New()

	home := view.PageControl{
		Title:  "Counter — go-fiber-live-view",
		Path:   "/",
		Router: app,
	}

	home.Register(func() view.LiveDriver {
		count := 0

		view.New("btn_dec", &components.Button{Caption: "−", Variant: "secondary"}).
			SetClick(func(this *components.Button, data interface{}) {
				count--
				this.FillValueById("counter_value", fmt.Sprint(count))
			})

		view.New("btn_inc", &components.Button{Caption: "+"}).
			SetClick(func(this *components.Button, data interface{}) {
				count++
				this.FillValueById("counter_value", fmt.Sprint(count))
			})

		return view.NewLayout("counter-"+uuid.NewString(), `
		<div class="lv-container">
			<h1>Counter</h1>
			<p class="lv-muted">State lives on the server. Every click travels over websocket.</p>
			<div class="lv-row">
				{{mount "btn_dec"}}
				<h2 id="counter_value" style="min-width:3rem;text-align:center">0</h2>
				{{mount "btn_inc"}}
			</div>
		</div>`)
	})

	fmt.Println("Counter example -> http://localhost:3001")
	log.Fatal(app.Listen(":3001"))
}
