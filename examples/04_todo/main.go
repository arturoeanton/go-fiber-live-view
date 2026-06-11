// Todo: a collaborative task list. Open it in two browsers — every change
// is broadcast to all connected sessions in real time.
package main

import (
	"fmt"
	"html"
	"log"
	"strings"
	"sync"

	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type Task struct {
	ID   string
	Name string
	Done bool
}

var (
	muStore sync.Mutex
	tasks   []*Task
)

func renderTasks() (string, int, int) {
	muStore.Lock()
	defer muStore.Unlock()
	var sb strings.Builder
	sb.WriteString(`<ul class="lv-list">`)
	pending := 0
	for _, t := range tasks {
		checked, style := "", ""
		if t.Done {
			checked = "checked"
			style = "text-decoration:line-through;color:var(--lv-text-muted)"
		} else {
			pending++
		}
		sb.WriteString(fmt.Sprintf(`<li class="lv-list-item" style="display:flex;justify-content:space-between;align-items:center">
			<label class="lv-check"><input type="checkbox" %s onchange="send_event('tasks_list','Toggle','%s')"/>
			<span style="%s">%s</span></label>
			<button class="lv-btn lv-btn-danger" style="padding:.15rem .5rem" onclick="send_event('tasks_list','Delete','%s')">&times;</button>
		</li>`, checked, t.ID, style, html.EscapeString(t.Name), t.ID))
	}
	if len(tasks) == 0 {
		sb.WriteString(`<li class="lv-list-item lv-muted">No tasks yet — add one above.</li>`)
	}
	sb.WriteString(`</ul>`)
	return sb.String(), pending, len(tasks)
}

func main() {
	app := fiber.New()

	home := view.PageControl{
		Title:  "Todo — go-fiber-live-view",
		Path:   "/",
		Router: app,
	}

	home.Register(func() view.LiveDriver {
		taskList := view.NewWithTemplate("tasks_list", `<div id="tasks_list"></div>`)

		refresh := func() {
			html, pending, total := renderTasks()
			taskList.FillValue(html)
			taskList.FillValueById("todo_stats", fmt.Sprintf("%d pending of %d", pending, total))
		}

		addTask := func(name string) {
			name = strings.TrimSpace(name)
			if name == "" {
				return
			}
			muStore.Lock()
			tasks = append(tasks, &Task{ID: uuid.NewString(), Name: name})
			muStore.Unlock()
			view.SendToAllLayouts("UPDATE")
		}

		taskList.SetEvent("Toggle", func(this *view.None, data interface{}) {
			id := fmt.Sprint(data)
			muStore.Lock()
			for _, t := range tasks {
				if t.ID == id {
					t.Done = !t.Done
				}
			}
			muStore.Unlock()
			view.SendToAllLayouts("UPDATE")
		})

		taskList.SetEvent("Delete", func(this *view.None, data interface{}) {
			id := fmt.Sprint(data)
			muStore.Lock()
			for i, t := range tasks {
				if t.ID == id {
					tasks = append(tasks[:i], tasks[i+1:]...)
					break
				}
			}
			muStore.Unlock()
			view.SendToAllLayouts("UPDATE")
		})

		input := view.New("input_new", &components.InputText{Placeholder: "What needs to be done? (Enter to add)"})
		input.SetEnter(func(this *components.InputText, data interface{}) {
			addTask(fmt.Sprint(data))
			this.SetValue("")
		})

		view.New("btn_add", &components.Button{Caption: "Add"}).
			SetClick(func(this *components.Button, data interface{}) {
				addTask(this.GetElementById("input_new"))
				input.SetValue("")
			})

		document := view.NewLayout("todo-"+uuid.NewString(), `
		<div class="lv-container" style="max-width:640px">
			<h1>Shared Todo</h1>
			<p class="lv-muted">Open this page in two browsers: changes sync instantly. <span class="lv-badge lv-badge-secondary" id="todo_stats"></span></p>
			<div class="lv-row">
				<div style="flex:1">{{mount "input_new"}}</div>
				{{mount "btn_add"}}
			</div>
			{{mount "tasks_list"}}
		</div>`)

		document.Component.SetHandlerEventIn(func(data interface{}) { refresh() })
		document.Component.SetHandlerFirstTime(func() { refresh() })
		return document
	})

	fmt.Println("Todo example -> http://localhost:3004")
	log.Fatal(app.Listen(":3004"))
}
