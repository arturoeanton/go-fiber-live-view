// Chat: the full power of go-fiber-live-view in one app.
//
//   - Multiple rooms with live unread counters
//   - Online user list with private messages (click a user or use "@nick hi")
//   - Typing indicator, message history, timestamps and emoji shortcodes
//   - System messages for join / leave / room switching
//
// Everything renders on the server; the browser only runs the liveview
// wasm client. Open several browsers and chat with yourself.
package main

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arturoeanton/go-fiber-live-view/liveview/components"
	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------- model --

type ChatMsg struct {
	From    string
	To      string // set on private messages
	Text    string
	At      time.Time
	System  bool
	Private bool
}

type Session struct {
	LayoutID    string
	Nick        string
	Room        string
	Target      string // non-empty: next messages go private to this nick
	TypingUntil time.Time
	Unread      map[string]int
}

type Hub struct {
	mu       sync.Mutex
	rooms    map[string][]ChatMsg
	privates []ChatMsg
	sessions map[string]*Session
}

var hub = &Hub{
	rooms:    map[string][]ChatMsg{"general": {}, "random": {}},
	sessions: map[string]*Session{},
}

var validName = regexp.MustCompile(`^[a-zA-Z0-9_\-]{2,16}$`)

const maxHistory = 200

func (h *Hub) join(layoutID, nick string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !validName.MatchString(nick) {
		return fmt.Errorf("nickname must be 2-16 chars: letters, numbers, _ or -")
	}
	for _, s := range h.sessions {
		if strings.EqualFold(s.Nick, nick) {
			return fmt.Errorf("nickname %q is already taken", nick)
		}
	}
	h.sessions[layoutID] = &Session{LayoutID: layoutID, Nick: nick, Room: "general", Unread: map[string]int{}}
	h.postLocked("general", ChatMsg{System: true, Text: nick + " joined the room", At: time.Now()})
	return nil
}

func (h *Hub) leave(layoutID string) {
	h.mu.Lock()
	s := h.sessions[layoutID]
	if s != nil {
		delete(h.sessions, layoutID)
		h.postLocked(s.Room, ChatMsg{System: true, Text: s.Nick + " left", At: time.Now()})
	}
	h.mu.Unlock()
	if s != nil {
		view.SendToAllLayouts("R")
	}
}

// postLocked appends a message to a room and bumps unread counters of the
// sessions that are looking at another room. Callers must hold h.mu.
func (h *Hub) postLocked(room string, m ChatMsg) {
	msgs := append(h.rooms[room], m)
	if len(msgs) > maxHistory {
		msgs = msgs[len(msgs)-maxHistory:]
	}
	h.rooms[room] = msgs
	if !m.System {
		for _, s := range h.sessions {
			if s.Room != room {
				s.Unread[room]++
			}
		}
	}
}

func (h *Hub) send(layoutID, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	h.mu.Lock()
	s := h.sessions[layoutID]
	if s == nil {
		h.mu.Unlock()
		return
	}
	target := s.Target
	// "@nick message" forces a private message
	if strings.HasPrefix(text, "@") {
		if i := strings.Index(text, " "); i > 1 {
			target = text[1:i]
			text = strings.TrimSpace(text[i:])
		}
	}
	if target != "" {
		h.privates = append(h.privates, ChatMsg{From: s.Nick, To: target, Text: text, At: time.Now(), Private: true})
		if len(h.privates) > maxHistory*2 {
			h.privates = h.privates[len(h.privates)-maxHistory*2:]
		}
	} else {
		h.postLocked(s.Room, ChatMsg{From: s.Nick, Text: text, At: time.Now()})
	}
	h.mu.Unlock()
	view.SendToAllLayouts("R")
}

func (h *Hub) switchRoom(layoutID, room string) {
	h.mu.Lock()
	s := h.sessions[layoutID]
	if s == nil || s.Room == room {
		h.mu.Unlock()
		return
	}
	if _, ok := h.rooms[room]; !ok {
		h.mu.Unlock()
		return
	}
	s.Room = room
	s.Target = ""
	s.Unread[room] = 0
	h.mu.Unlock()
	view.SendToAllLayouts("R")
}

func (h *Hub) createRoom(layoutID, room string) {
	if !validName.MatchString(room) {
		return
	}
	h.mu.Lock()
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = []ChatMsg{}
	}
	h.mu.Unlock()
	h.switchRoom(layoutID, room)
}

func (h *Hub) typing(layoutID string) {
	h.mu.Lock()
	if s := h.sessions[layoutID]; s != nil {
		s.TypingUntil = time.Now().Add(2 * time.Second)
	}
	h.mu.Unlock()
	view.SendToAllLayouts("T")
	time.AfterFunc(2100*time.Millisecond, func() { view.SendToAllLayouts("T") })
}

func (h *Hub) setTarget(layoutID, nick string) {
	h.mu.Lock()
	if s := h.sessions[layoutID]; s != nil && !strings.EqualFold(nick, s.Nick) {
		s.Target = nick
	}
	h.mu.Unlock()
	view.SendToLayouts("R", layoutID)
}

// ------------------------------------------------------------ rendering --

var emojis = strings.NewReplacer(
	":)", "😄", ":(", "😢", ":D", "😁", ";)", "😉",
	":+1:", "👍", "<3", "❤️", ":fire:", "🔥", ":tada:", "🎉", ":rocket:", "🚀",
)

func esc(s string) string { return html.EscapeString(s) }

func renderMessagesLocked(h *Hub, s *Session) string {
	msgs := make([]ChatMsg, 0, maxHistory)
	msgs = append(msgs, h.rooms[s.Room]...)
	for _, p := range h.privates {
		if strings.EqualFold(p.From, s.Nick) || strings.EqualFold(p.To, s.Nick) {
			msgs = append(msgs, p)
		}
	}
	sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].At.Before(msgs[j].At) })

	var sb strings.Builder
	for _, m := range msgs {
		ts := m.At.Format("15:04")
		switch {
		case m.System:
			sb.WriteString(`<div class="msg-system">` + esc(m.Text) + ` <span class="msg-time">` + ts + `</span></div>`)
		default:
			cls := "msg"
			if strings.EqualFold(m.From, s.Nick) {
				cls += " msg-own"
			}
			if m.Private {
				cls += " msg-private"
			}
			head := esc(m.From)
			if m.Private {
				head = "🔒 " + esc(m.From) + " → " + esc(m.To)
			}
			text := emojis.Replace(esc(m.Text))
			sb.WriteString(`<div class="` + cls + `"><div class="msg-head">` + head +
				` <span class="msg-time">` + ts + `</span></div><div class="msg-body">` + text + `</div></div>`)
		}
	}
	if len(msgs) == 0 {
		sb.WriteString(`<div class="msg-system">No messages yet in #` + esc(s.Room) + ` — say hi! 👋</div>`)
	}
	return sb.String()
}

func renderRoomsLocked(h *Hub, s *Session) string {
	names := make([]string, 0, len(h.rooms))
	for name := range h.rooms {
		names = append(names, name)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, name := range names {
		cls := "side-item"
		if name == s.Room {
			cls += " side-active"
		}
		badge := ""
		if n := s.Unread[name]; n > 0 && name != s.Room {
			badge = `<span class="lv-badge lv-badge-danger">` + fmt.Sprint(n) + `</span>`
		}
		sb.WriteString(`<div class="` + cls + `" onclick="send_event('rooms_box','Switch','` + esc(name) + `')"># ` + esc(name) + ` ` + badge + `</div>`)
	}
	return sb.String()
}

func renderUsersLocked(h *Hub, s *Session) string {
	type user struct {
		nick   string
		room   string
		typing bool
	}
	users := make([]user, 0, len(h.sessions))
	for _, o := range h.sessions {
		users = append(users, user{o.Nick, o.Room, time.Now().Before(o.TypingUntil)})
	}
	sort.Slice(users, func(i, j int) bool { return strings.ToLower(users[i].nick) < strings.ToLower(users[j].nick) })

	var sb strings.Builder
	for _, u := range users {
		cls := "side-item"
		label := esc(u.nick)
		if strings.EqualFold(u.nick, s.Nick) {
			label += ` <span class="lv-muted">(you)</span>`
		} else {
			cls += " side-user"
		}
		if u.room == s.Room {
			label = `<span class="dot dot-here"></span>` + label
		} else {
			label = `<span class="dot"></span>` + label
		}
		if u.typing {
			label += ` ✍️`
		}
		sb.WriteString(`<div class="` + cls + `" onclick="send_event('users_box','Target','` + esc(u.nick) + `')">` + label + `</div>`)
	}
	return sb.String()
}

func renderTypingLocked(h *Hub, s *Session) string {
	now := time.Now()
	var who []string
	for _, o := range h.sessions {
		if o.Room == s.Room && !strings.EqualFold(o.Nick, s.Nick) && now.Before(o.TypingUntil) {
			who = append(who, esc(o.Nick))
		}
	}
	if len(who) == 0 {
		return "&nbsp;"
	}
	if len(who) == 1 {
		return who[0] + " is typing…"
	}
	return strings.Join(who, ", ") + " are typing…"
}

func renderHeaderLocked(h *Hub, s *Session) string {
	online := 0
	for _, o := range h.sessions {
		if o.Room == s.Room {
			online++
		}
	}
	return `# ` + esc(s.Room) + ` <span class="lv-badge lv-badge-success">` + fmt.Sprint(online) + ` online</span>`
}

func renderTargetLocked(s *Session) string {
	if s.Target == "" {
		return "&nbsp;"
	}
	return `🔒 private to <b>@` + esc(s.Target) + `</b> — <a href="#" onclick="send_event('users_box','Target','` + esc(s.Nick) + `');return false">cancel</a>`
}

// ------------------------------------------------------------------ app --

const chatCss = `
html, body, #content { height: 100%; }
.chat-app { display: none; height: 100vh; }
.chat-side { width: 230px; background: #0f172a; color: #e2e8f0; display: flex; flex-direction: column; padding: 1rem .75rem; gap: .5rem; overflow-y: auto; }
.chat-side h3 { font-size: .75rem; text-transform: uppercase; letter-spacing: .1em; color: #64748b; margin: .75rem 0 .25rem; }
.side-item { padding: .35rem .6rem; border-radius: 6px; cursor: pointer; font-size: .9rem; display: flex; align-items: center; gap: .4rem; }
.side-item:hover { background: rgba(255,255,255,.08); }
.side-active { background: #4f46e5; color: #fff; font-weight: 600; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: #475569; display: inline-block; }
.dot-here { background: #22c55e; }
.chat-main { flex: 1; display: flex; flex-direction: column; background: #f1f5f9; min-width: 0; }
.chat-header { padding: .9rem 1.25rem; background: #fff; border-bottom: 1px solid var(--lv-border); font-weight: 700; font-size: 1.05rem; display:flex; align-items:center; gap:.5rem; }
.chat-msgs { flex: 1; overflow-y: auto; padding: 1rem 1.25rem; display: flex; flex-direction: column; gap: .5rem; }
.msg { background: #fff; border: 1px solid var(--lv-border); border-radius: 10px; padding: .5rem .75rem; max-width: 70%; align-self: flex-start; box-shadow: var(--lv-shadow); }
.msg-own { align-self: flex-end; background: #eef2ff; border-color: #c7d2fe; }
.msg-private { background: #fefce8; border-color: #fde68a; }
.msg-head { font-size: .75rem; font-weight: 700; color: #4f46e5; }
.msg-own .msg-head { color: #6366f1; }
.msg-time { color: var(--lv-text-muted); font-weight: 400; margin-left: .35rem; }
.msg-body { font-size: .92rem; word-wrap: break-word; }
.msg-system { text-align: center; font-size: .8rem; color: var(--lv-text-muted); }
.chat-typing { padding: 0 1.25rem; font-size: .8rem; color: var(--lv-text-muted); height: 1.25rem; font-style: italic; }
.chat-input { display: flex; gap: .5rem; padding: .75rem 1.25rem; background: #fff; border-top: 1px solid var(--lv-border); align-items: center; }
.chat-input .lv-field { flex: 1; margin: 0; }
.chat-target { padding: .15rem 1.25rem; font-size: .8rem; background: #fefce8; }
.join-wrap { height: 100vh; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #4f46e5, #0ea5e9); }
.join-card { background: #fff; border-radius: 14px; padding: 2rem; width: min(380px, 90vw); box-shadow: 0 25px 60px rgba(0,0,0,.35); }
.join-card h1 { margin: 0 0 .25rem; font-size: 1.4rem; }
.side-newroom input { width: 100%; box-sizing: border-box; background: rgba(255,255,255,.07); border: 1px solid rgba(255,255,255,.15); color: #e2e8f0; border-radius: 6px; padding: .35rem .6rem; font-size: .85rem; }
`

func main() {
	app := fiber.New()

	home := view.PageControl{
		Title:  "LiveChat — go-fiber-live-view",
		Path:   "/",
		Router: app,
		Css:    chatCss,
	}

	home.Register(func() view.LiveDriver {
		lid := "chat-" + uuid.NewString()

		messagesBox := view.NewWithTemplate("messages_box", `<div id="messages_box" class="chat-msgs"></div>`)
		roomsBox := view.NewWithTemplate("rooms_box", `<div id="rooms_box"></div>`)
		usersBox := view.NewWithTemplate("users_box", `<div id="users_box"></div>`)
		typingBox := view.NewWithTemplate("typing_box", `<div id="typing_box" class="chat-typing"></div>`)
		headerBox := view.NewWithTemplate("header_box", `<span id="header_box"></span>`)
		targetBox := view.NewWithTemplate("target_box", `<div id="target_box" class="chat-target">&nbsp;</div>`)
		joinError := view.NewWithTemplate("join_error", `<div id="join_error"></div>`)

		refresh := func() {
			hub.mu.Lock()
			s := hub.sessions[lid]
			if s == nil {
				hub.mu.Unlock()
				return
			}
			msgs := renderMessagesLocked(hub, s)
			rooms := renderRoomsLocked(hub, s)
			users := renderUsersLocked(hub, s)
			typing := renderTypingLocked(hub, s)
			header := renderHeaderLocked(hub, s)
			target := renderTargetLocked(s)
			hub.mu.Unlock()

			messagesBox.FillValue(msgs)
			roomsBox.FillValue(rooms)
			usersBox.FillValue(users)
			typingBox.FillValue(typing)
			headerBox.FillValue(header)
			targetBox.FillValue(target)
			messagesBox.EvalScript(`var d=document.getElementById('messages_box'); if(d){d.scrollTop=d.scrollHeight;}`)
		}

		refreshTyping := func() {
			hub.mu.Lock()
			s := hub.sessions[lid]
			if s == nil {
				hub.mu.Unlock()
				return
			}
			typing := renderTypingLocked(hub, s)
			users := renderUsersLocked(hub, s)
			hub.mu.Unlock()
			typingBox.FillValue(typing)
			usersBox.FillValue(users)
		}

		inputNick := view.New("input_nick", &components.InputText{Placeholder: "Your nickname…"})
		inputMsg := view.New("input_msg", &components.InputText{Placeholder: "Write a message… (@nick for private)"})

		var document *view.ComponentDriver[*view.Layout]

		doJoin := func(nick string) {
			if err := hub.join(lid, strings.TrimSpace(nick)); err != nil {
				joinError.FillValue(`<div class="lv-alert lv-alert-danger">` + esc(err.Error()) + `</div>`)
				return
			}
			document.GetDriverById("join_box").SetStyle("display:none")
			document.GetDriverById("chat_app").SetStyle("display:flex")
			inputMsg.EvalScript(`var i=document.getElementById('input_msg'); if(i){i.focus();}`)
			view.SendToAllLayouts("R")
		}

		inputNick.SetEnter(func(this *components.InputText, data interface{}) { doJoin(fmt.Sprint(data)) })
		view.New("btn_join", &components.Button{Caption: "Join chat"}).
			SetClick(func(this *components.Button, data interface{}) {
				doJoin(this.GetElementById("input_nick"))
			})

		submit := func(text string) {
			hub.send(lid, text)
			inputMsg.SetValue("")
		}
		inputMsg.SetEnter(func(this *components.InputText, data interface{}) { submit(fmt.Sprint(data)) })
		inputMsg.SetKeyUp(func(this *components.InputText, data interface{}) { hub.typing(lid) })
		view.New("btn_send", &components.Button{Caption: "Send"}).
			SetClick(func(this *components.Button, data interface{}) {
				submit(this.GetElementById("input_msg"))
			})

		roomsBox.SetEvent("Switch", func(this *view.None, data interface{}) {
			hub.switchRoom(lid, fmt.Sprint(data))
		})
		usersBox.SetEvent("Target", func(this *view.None, data interface{}) {
			hub.setTarget(lid, fmt.Sprint(data))
		})

		view.New("input_room", &components.InputText{Placeholder: "+ new room (Enter)"}).
			SetEnter(func(this *components.InputText, data interface{}) {
				hub.createRoom(lid, strings.TrimSpace(fmt.Sprint(data)))
				this.SetValue("")
			})

		document = view.NewLayout(lid, `
		<div id="join_box">
			<div class="join-wrap">
				<div class="join-card">
					<h1>💬 LiveChat</h1>
					<p class="lv-muted">Realtime chat rendered 100% from Go.</p>
					{{mount "join_error"}}
					{{mount "input_nick"}}
					{{mount "btn_join"}}
				</div>
			</div>
		</div>
		<div id="chat_app" class="chat-app">
			<div class="chat-side">
				<h3>Rooms</h3>
				{{mount "rooms_box"}}
				<div class="side-newroom">{{mount "input_room"}}</div>
				<h3>Online</h3>
				{{mount "users_box"}}
			</div>
			<div class="chat-main">
				<div class="chat-header">{{mount "header_box"}}</div>
				{{mount "messages_box"}}
				{{mount "typing_box"}}
				{{mount "target_box"}}
				<div class="chat-input">
					{{mount "input_msg"}}
					{{mount "btn_send"}}
				</div>
			</div>
		</div>`)

		document.Component.SetHandlerEventIn(func(data interface{}) {
			if fmt.Sprint(data) == "T" {
				refreshTyping()
				return
			}
			refresh()
		})
		document.Component.SetHandlerEventDestroy(func(id string) {
			hub.leave(lid)
		})

		return document
	})

	fmt.Println("LiveChat example -> http://localhost:3006")
	log.Fatal(app.Listen(":3006"))
}
