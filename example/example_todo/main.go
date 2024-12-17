package main

import (
	"encoding/json"
	"fmt"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"strconv"
)

var (
	todos = make(map[string]*Todo)
	tasks = make(map[string]Task)
)

type Task struct {
	Name  *string `json:"name,omitempty"`
	State *int    `json:"state,omitempty"`
}

type Todo struct {
	*view.ComponentDriver[*Todo]
	ParentId   string
	ActualTime string
	code       string
	Tasks      *map[string]Task
}

type Message struct {
	ID     string `json:"id"`
	Event  string `json:"event"`
	Source string `json:"source"`
}

func (t *Todo) GetDriver() view.LiveDriver {
	return t
}

func (t *Todo) Start() {
	tasksString, _ := view.FileToString("tasks.json")
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
	(*t.Tasks)[id] = task
	content, _ := json.Marshal(t.Tasks)
	view.StringToFile("tasks.json", string(content))

	//t.Commit()

	msg := Message{
		ID:     t.ParentId,
		Event:  "UPDATE",
		Source: "ADD",
	}
	msgJson, _ := json.Marshal(msg)
	view.SendToAllLayouts(string(msgJson))
}

func (t *Todo) RemoveTask(data interface{}) {
	id := data.(string)
	if _, ok := (*t.Tasks)[id]; !ok {
		return
	}

	delete(*t.Tasks, id)
	content, _ := json.Marshal(t.Tasks)
	view.StringToFile("tasks.json", string(content))
	//t.Commit()
	msg := Message{
		ID:     t.ParentId,
		Event:  "UPDATE",
		Source: "REMOVE",
	}
	msgJson, _ := json.Marshal(msg)
	view.SendToAllLayouts(string(msgJson))
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
	(*t.Tasks)[id] = task
	content, _ := json.Marshal(t.Tasks)
	view.StringToFile("tasks.json", string(content))
	//t.Commit()

	msg := Message{
		ID:     t.ParentId,
		Event:  "UPDATE",
		Source: "CHANGE",
	}
	msgJson, _ := json.Marshal(msg)
	view.SendToAllLayouts(string(msgJson))
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
		idLayout := uuid.NewString()
		document := view.NewLayout(idLayout, `<div> {{mount "todo"}} </div>`)

		todo := &Todo{
			ParentId: idLayout,
			Tasks:    &tasks,
		}
		todos[idLayout] = todo
		view.New("todo", todo)

		document.Component.SetHandlerEventIn(func(data interface{}) {
			msgJson := data.(string)

			if msgJson == "FIRST_TIME" {
				return
			}

			var msg Message
			err := json.Unmarshal([]byte(msgJson), &msg)
			if err != nil {
				return
			}

			if msg.Event == "UPDATE" {
				for _, t := range todos {

					fmt.Println(idLayout, "event", msgJson)

					t.Commit()

				}
			}
		})

		return document
	})

	app.Listen(":3000")

}
