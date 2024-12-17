package main

import (
	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v2"
)

func main() {

	app := fiber.New()

	home := view.PageControl{
		Title: "Example1",
		Path:  "/",
		App:   app,
	}

	home.Register(func() view.LiveDriver {
		view.New("clock1", &components.Clock{})
		return view.NewLayout("layout1", `
		<div id="d2">{{mount "clock1"}}</div>
		`)
	})

	app.Listen(":3000")
}
