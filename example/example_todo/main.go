package main

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"strconv"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/google/uuid"
)

type Task struct {
	Name  *string `json:"name,omitempty"`
	State *int    `json:"state,omitempty"`
}

type Todo struct {
	*view.ComponentDriver[*Todo]
	ActualTime string
	code       string
	Tasks      map[string]Task
}

func (t *Todo) GetDriver() view.LiveDriver {
	return t
}

func (t *Todo) Start() {
	tasksString, _ := view.FileToString("tasks.json")
	t.Tasks = make(map[string]Task)
	json.Unmarshal([]byte(tasksString), &(t.Tasks))
	t.Commit()
}

func (t *Todo) GetTemplate() string {
	if t.code == "" {
		t.code, _ = view.FileToString("todo.html")
	}
	return t.code
}

func (t *Todo) Add(data interface{}) {
	name := t.GetElementById("new_name")
	stateStr := t.GetElementById("new_state")
	state, _ := strconv.Atoi(stateStr)
	id := uuid.NewString()
	task := Task{
		Name:  &name,
		State: &state,
	}
	t.Tasks[id] = task
	content, _ := json.Marshal(t.Tasks)
	view.StringToFile("tasks.json", string(content))
	t.Commit()
}

func (t *Todo) RemoveTask(data interface{}) {
	id := data.(string)
	delete(t.Tasks, id)
	content, _ := json.Marshal(t.Tasks)
	view.StringToFile("tasks.json", string(content))
	t.Commit()
}

func (t *Todo) Change(data interface{}) {
	id := data.(string)
	name := t.GetElementById("name_" + id)
	stateStr := t.GetElementById("state_" + id)
	state, _ := strconv.Atoi(stateStr)
	task := Task{
		Name:  &name,
		State: &state,
	}
	t.Tasks[id] = task
	content, _ := json.Marshal(t.Tasks)
	view.StringToFile("tasks.json", string(content))
	t.Commit()
}

func main() {
	app := fiber.New()
	home := view.PageControl{
		Title:    "Todo",
		HeadCode: "head.html",
		Lang:     "en",
		Path:     "/",
		Router:   app,
		//	Debug:    true,
	}
	home.Register(func() view.LiveDriver {
		view.New("todo", &Todo{})
		return view.NewLayout("layout1", `<div> {{mount "todo"}} </div>`)
	})

	app.Listen(":3000")

}
