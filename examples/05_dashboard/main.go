// Dashboard: the server pushes simulated metrics every second — progress
// bars, badges and a request table update live without any client code.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func main() {
	app := fiber.New()

	home := view.PageControl{
		Title:  "Dashboard — go-fiber-live-view",
		Path:   "/",
		Router: app,
	}

	paths := []string{"/api/users", "/api/orders", "/api/login", "/healthz", "/api/products"}

	home.Register(func() view.LiveDriver {
		cpu := view.New("cpu", &components.ProgressBar{Value: 30, ShowLabel: true})
		mem := view.New("mem", &components.ProgressBar{Value: 50, ShowLabel: true})
		disk := view.New("disk", &components.ProgressBar{Value: 70, ShowLabel: true})
		status := view.New("status", &components.Badge{Text: "HEALTHY", Variant: "success"})
		view.New("clock", &components.Clock{Format: "15:04:05", Interval: time.Second})

		requests := view.New("requests", &components.Table{
			Headers: []string{"Time", "Path", "Status", "Latency"},
		})

		reqCount := 0

		document := view.NewLayout("dashboard-"+uuid.NewString(), `
		<div class="lv-container">
			<div class="lv-row" style="justify-content:space-between">
				<h1>Live Dashboard</h1>
				<div class="lv-row">{{mount "status"}} {{mount "clock"}}</div>
			</div>
			<div class="lv-row" style="align-items:stretch">
				<div class="lv-card" style="flex:1"><div class="lv-card-header">CPU</div><div class="lv-card-body">{{mount "cpu"}}</div></div>
				<div class="lv-card" style="flex:1"><div class="lv-card-header">Memory</div><div class="lv-card-body">{{mount "mem"}}</div></div>
				<div class="lv-card" style="flex:1"><div class="lv-card-header">Disk</div><div class="lv-card-body">{{mount "disk"}}</div></div>
			</div>
			<div class="lv-card">
				<div class="lv-card-header">Latest requests (<span id="req_count">0</span>)</div>
				<div class="lv-card-body">{{mount "requests"}}</div>
			</div>
		</div>`)

		document.Component.SetHandlerEventTime(time.Second, func() {
			cpuVal := 20 + rand.Intn(80)
			cpu.SetProgress(cpuVal)
			mem.SetProgress(30 + rand.Intn(60))
			disk.SetProgress(60 + rand.Intn(25))

			if cpuVal > 85 {
				status.SetBadge("DEGRADED", "danger")
			} else {
				status.SetBadge("HEALTHY", "success")
			}

			reqCount++
			row := []string{
				time.Now().Format("15:04:05"),
				paths[rand.Intn(len(paths))],
				fmt.Sprint(200),
				fmt.Sprintf("%dms", 5+rand.Intn(120)),
			}
			rows := append([][]string{row}, requests.Rows...)
			if len(rows) > 8 {
				rows = rows[:8]
			}
			requests.SetRows(rows)
			requests.FillValueById("req_count", fmt.Sprint(reqCount))
		})

		return document
	})

	fmt.Println("Dashboard example -> http://localhost:3005")
	log.Fatal(app.Listen(":3005"))
}
