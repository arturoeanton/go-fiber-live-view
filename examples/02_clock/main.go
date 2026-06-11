// Clock: the server pushes DOM updates without any user interaction.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func main() {
	app := fiber.New()

	home := view.PageControl{
		Title:  "Clock — go-fiber-live-view",
		Path:   "/",
		Router: app,
	}

	home.Register(func() view.LiveDriver {
		view.New("clock_fast", &components.Clock{Format: "15:04:05.000", Interval: 50 * time.Millisecond})
		view.New("clock_second", &components.Clock{Format: "15:04:05", Interval: time.Second})
		view.New("clock_full", &components.Clock{Format: time.RFC1123, Interval: time.Second})

		return view.NewLayout("clock-"+uuid.NewString(), `
		<div class="lv-container">
			<h1>Server clocks</h1>
			<p class="lv-muted">Three clocks pushed from the server at different rates.</p>
			<div class="lv-card"><div class="lv-card-body lv-row">20 fps: {{mount "clock_fast"}}</div></div>
			<div class="lv-card"><div class="lv-card-body lv-row">1 fps: {{mount "clock_second"}}</div></div>
			<div class="lv-card"><div class="lv-card-body lv-row">Full date: {{mount "clock_full"}}</div></div>
		</div>`)
	})

	fmt.Println("Clock example -> http://localhost:3002")
	log.Fatal(app.Listen(":3002"))
}
