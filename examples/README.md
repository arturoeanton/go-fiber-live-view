# Examples

Every example is a standalone `main.go`. From this folder, run any of them
with `go run` — no extra setup, assets are embedded in the library:

```sh
cd examples

go run ./01_counter     # http://localhost:3001
go run ./02_clock       # http://localhost:3002
go run ./03_gallery     # http://localhost:3003
go run ./04_todo        # http://localhost:3004
go run ./05_dashboard   # http://localhost:3005
go run ./06_chat        # http://localhost:3006
go run ./07_board       # http://localhost:3007
go run ./08_notion      # http://localhost:3008
```

Each example uses its own port, so you can run all of them at the same time.

| Example | What it shows |
|---|---|
| **01_counter** | The "hello world": server-side state, button events, DOM updates over websocket. |
| **02_clock** | Server push without user interaction — three clocks at different rates. |
| **03_gallery** | Every built-in component (buttons, inputs, select, checkbox, radio, table, list, tabs, modal, alert, progress, badge, nav, spinner, clock) wired with events. |
| **04_todo** | Collaborative todo list: state shared between sessions, broadcast with `SendToAllLayouts`. Open two browsers and watch them sync. |
| **05_dashboard** | Periodic server-side updates with `SetHandlerEventTime`: live metrics, progress bars and a request table. |
| **06_chat** | 💬 **The showcase.** Multi-room chat with online users, private messages (`@nick hi` or click a user), typing indicator, unread counters, message history, emojis and system messages. |
| **07_board** | 🎨 **Excalidraw-style whiteboard.** Pen, shapes, arrows, text and eraser; select/move, undo, zoom/pan, SVG export; live collaboration with remote cursors; multiple boards persisted in SQLite (`board.db`). |
| **08_notion** | 🪶 **Notion-style workspace.** Nested pages, block editor (headings, todos, lists, quotes, code, callouts, images, tables) with markdown shortcuts and "/" commands, drag & drop, block-level live collaboration with editing badges and presence avatars. SQLite (`notion.db`). |

## Tips

- Components are created with `view.New("id", &components.X{...})` and placed
  in the layout with `{{mount "id"}}`.
- Any element with an `id` in the layout can be manipulated from Go with
  `document.GetDriverById("the_id").FillValue(...)` / `.SetStyle(...)`.
- `view.SendToAllLayouts(msg)` / `view.SendToLayouts(msg, ids...)` broadcast
  to live sessions; handle them with `SetHandlerEventIn`.
