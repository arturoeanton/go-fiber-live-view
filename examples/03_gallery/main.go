// Gallery: every built-in component wired together. Interacting with any
// component reports the event in the alert at the top — all server-side,
// without writing HTML for the widgets themselves.
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
		Title:  "Component Gallery — go-fiber-live-view",
		Path:   "/",
		Router: app,
	}

	home.Register(func() view.LiveDriver {
		notify := view.New("gallery_alert", &components.Alert{
			Variant: "info", Message: "Interact with any component…", Dismissible: true,
		})
		report := func(msg string) { notify.Notify("success", msg) }

		view.New("nav", &components.NavBar{
			Brand: "go-fiber-live-view",
			Items: []components.NavItem{{Code: "home", Caption: "Home"}, {Code: "docs", Caption: "Docs"}, {Code: "about", Caption: "About"}},
		}).SetItemClick(func(this *components.NavBar, data interface{}) {
			report(fmt.Sprintf("NavBar: clicked %q", data))
		})

		view.New("btn_primary", &components.Button{Caption: "Primary"}).
			SetClick(func(this *components.Button, data interface{}) { report("Button: primary clicked") })
		view.New("btn_secondary", &components.Button{Caption: "Secondary", Variant: "secondary"}).
			SetClick(func(this *components.Button, data interface{}) { report("Button: secondary clicked") })
		view.New("btn_success", &components.Button{Caption: "Success", Variant: "success"}).
			SetClick(func(this *components.Button, data interface{}) { report("Button: success clicked") })
		view.New("btn_danger", &components.Button{Caption: "Danger", Variant: "danger"}).
			SetClick(func(this *components.Button, data interface{}) { report("Button: danger clicked") })
		view.New("btn_ghost", &components.Button{Caption: "Ghost", Variant: "ghost"}).
			SetClick(func(this *components.Button, data interface{}) { report("Button: ghost clicked") })

		view.New("input_name", &components.InputText{Label: "Name", Placeholder: "Type and press Enter…"}).
			SetEnter(func(this *components.InputText, data interface{}) {
				report(fmt.Sprintf("InputText: Enter with %q", data))
			})
		view.New("input_pass", &components.InputText{Label: "Password", Type: "password", Placeholder: "secret"})
		view.New("textarea_bio", &components.TextArea{Label: "Bio", Placeholder: "Tell us something…", Rows: 3}).
			SetChange(func(this *components.TextArea, data interface{}) {
				report(fmt.Sprintf("TextArea: %d chars", len(fmt.Sprint(data))))
			})

		view.New("select_lang", &components.Select{
			Label: "Language", Selected: "go",
			Options: []components.Option{{Value: "go", Label: "Go"}, {Value: "elixir", Label: "Elixir"}, {Value: "rust", Label: "Rust"}},
		}).SetChange(func(this *components.Select, data interface{}) {
			this.Selected = fmt.Sprint(data)
			report(fmt.Sprintf("Select: %q", data))
		})

		view.New("check_news", &components.Checkbox{Label: "Subscribe to newsletter"}).
			SetChange(func(this *components.Checkbox, data interface{}) {
				this.Checked = fmt.Sprint(data) == "true"
				report(fmt.Sprintf("Checkbox: %v", data))
			})

		view.New("radio_size", &components.RadioGroup{
			Label: "Size", Selected: "m",
			Options: []components.Option{{Value: "s", Label: "Small"}, {Value: "m", Label: "Medium"}, {Value: "l", Label: "Large"}},
		}).SetChange(func(this *components.RadioGroup, data interface{}) {
			this.Selected = fmt.Sprint(data)
			report(fmt.Sprintf("RadioGroup: %q", data))
		})

		progress := view.New("progress_demo", &components.ProgressBar{Value: 40, ShowLabel: true})
		view.New("btn_more", &components.Button{Caption: "+10", Variant: "ghost"}).
			SetClick(func(this *components.Button, data interface{}) {
				if progress.Value < 100 {
					progress.SetProgress(progress.Value + 10)
				}
			})

		view.New("badge_new", &components.Badge{Text: "NEW"})
		view.New("badge_ok", &components.Badge{Text: "stable", Variant: "success"})
		view.New("badge_beta", &components.Badge{Text: "beta", Variant: "warning"})
		view.New("clock_badge", &components.Clock{})
		view.New("spinner_demo", &components.Spinner{})

		view.New("table_people", &components.Table{
			Headers: []string{"Name", "Role", "Country"},
			Rows: [][]string{
				{"Ada Lovelace", "Engineer", "UK"},
				{"Alan Turing", "Scientist", "UK"},
				{"Grace Hopper", "Admiral", "USA"},
			},
		}).SetRowClick(func(this *components.Table, data interface{}) {
			report(fmt.Sprintf("Table: clicked row %d (%s)", components.RowIndex(data), this.Rows[components.RowIndex(data)][0]))
		})

		view.New("list_demo", &components.List{Items: []string{"First item", "Second item", "Third item"}}).
			SetItemClick(func(this *components.List, data interface{}) {
				report(fmt.Sprintf("List: clicked item %d", components.RowIndex(data)))
			})

		view.New("tabs_demo", &components.Tabs{Tabs: []components.Tab{
			{Title: "Overview", Content: "<p>Components render from Go structs — no HTML required.</p>"},
			{Title: "Realtime", Content: "<p>Every change is pushed over websocket, like Phoenix LiveView.</p>"},
			{Title: "Go", Content: "<p>One goroutine per session, thousands of sessions per node.</p>"},
		}})

		modal := view.New("modal_demo", &components.Modal{
			Title:   "Hello from the server",
			Content: "<p>This modal was opened by Go code, not JavaScript.</p>",
		})
		view.New("btn_modal", &components.Button{Caption: "Open modal", Variant: "secondary"}).
			SetClick(func(this *components.Button, data interface{}) { modal.Show() })

		return view.NewLayout("gallery-"+uuid.NewString(), `
		{{mount "nav"}}
		<div class="lv-container">
			<h1>Component Gallery</h1>
			{{mount "gallery_alert"}}

			<div class="lv-card"><div class="lv-card-header">Buttons</div><div class="lv-card-body lv-row">
				{{mount "btn_primary"}} {{mount "btn_secondary"}} {{mount "btn_success"}} {{mount "btn_danger"}} {{mount "btn_ghost"}}
			</div></div>

			<div class="lv-card"><div class="lv-card-header">Form fields</div><div class="lv-card-body">
				{{mount "input_name"}} {{mount "input_pass"}} {{mount "textarea_bio"}}
				{{mount "select_lang"}} {{mount "check_news"}} {{mount "radio_size"}}
			</div></div>

			<div class="lv-card"><div class="lv-card-header">Feedback</div><div class="lv-card-body">
				<div class="lv-row">{{mount "progress_demo"}} {{mount "btn_more"}}</div>
				<div class="lv-row" style="margin-top:1rem">
					{{mount "badge_new"}} {{mount "badge_ok"}} {{mount "badge_beta"}} {{mount "spinner_demo"}} {{mount "clock_badge"}}
				</div>
			</div></div>

			<div class="lv-card"><div class="lv-card-header">Data</div><div class="lv-card-body">
				{{mount "table_people"}}
				<br/>
				{{mount "list_demo"}}
			</div></div>

			<div class="lv-card"><div class="lv-card-header">Tabs &amp; Modal</div><div class="lv-card-body">
				{{mount "tabs_demo"}}
				{{mount "btn_modal"}}
				{{mount "modal_demo"}}
			</div></div>
		</div>`)
	})

	fmt.Println("Gallery example -> http://localhost:3003")
	log.Fatal(app.Listen(":3003"))
}
