// Pad: real character-by-character co-editing (Google-Docs style) with the
// framework's transparent CRDT support.
//
// The whole collaborative machinery is three lines of Go and one attribute:
//
//	body := view.NewSharedText("pad-main", "...")   // shared CRDT document
//	body.OnChange(func(s string) { ... })           // observe changes
//	<textarea live-text="pad-main"></textarea>      // bind it in the layout
//
// The wasm client replicates the document (same RGA package as the server),
// applies local keystrokes with zero latency, merges remote ones and keeps
// the cursor in place. Open two browsers and type at the same time — even
// in the same word.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arturoeanton/go-fiber-live-view/liveview/view"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const saveFile = "pad.txt"

func main() {
	initial := "Escribí acá… abrí esta página en dos browsers y tipeen a la vez, incluso en la misma palabra.\n\nCada tecla es una operación CRDT (RGA): se aplica local sin latencia, viaja al servidor y se mergea en todas las réplicas."
	if data, err := os.ReadFile(saveFile); err == nil && len(data) > 0 {
		initial = string(data)
	}

	title := view.NewSharedText("pad-title", "Documento compartido")
	body := view.NewSharedText("pad-main", initial)

	// Debounced persistence: the pad survives server restarts.
	var saveMu sync.Mutex
	var saveTimer *time.Timer
	scheduleSave := func() {
		saveMu.Lock()
		defer saveMu.Unlock()
		if saveTimer != nil {
			saveTimer.Stop()
		}
		saveTimer = time.AfterFunc(time.Second, func() {
			os.WriteFile(saveFile, []byte(body.Text()), 0644)
		})
	}

	body.OnChange(func(s string) {
		scheduleSave()
		view.SendToAllLayouts("STATS")
	})
	title.OnChange(func(s string) {
		view.SendToAllLayouts("STATS")
	})

	app := fiber.New()
	home := view.PageControl{
		Title:  "Pad — go-fiber-live-view",
		Path:   "/",
		Router: app,
		Css: `
		html, body, #content { height: 100%; margin: 0; }
		.pad-app { max-width: 760px; margin: 0 auto; padding: 40px 24px; height: 100vh; display: flex; flex-direction: column; box-sizing: border-box; }
		.pad-title { font-size: 2rem; font-weight: 800; border: none; outline: none; width: 100%; color: #0f172a; padding: 0 0 10px; }
		.pad-meta { display: flex; gap: 14px; color: var(--lv-text-muted); font-size: .8rem; padding-bottom: 12px; border-bottom: 1px solid var(--lv-border); margin-bottom: 14px; }
		.pad-body { flex: 1; border: none; outline: none; resize: none; font-size: 1.02rem; line-height: 1.75; font-family: inherit; color: #1e293b; }
		`,
	}

	home.Register(func() view.LiveDriver {
		document := view.NewLayout("pad-"+uuid.NewString(), `
		<div class="pad-app">
			<input class="pad-title" live-text="pad-title" />
			<div class="pad-meta">
				<span>🟢 <span id="pad_viewers">1</span> conectados</span>
				<span id="pad_stats"></span>
				<span>guardado automático en pad.txt</span>
			</div>
			<textarea class="pad-body" live-text="pad-main" placeholder="Escribí acá…"></textarea>
		</div>`)

		refresh := func() {
			text := body.Text()
			words := len(strings.Fields(text))
			lines := strings.Count(text, "\n") + 1
			view.MuLayout.RLock()
			viewers := len(view.Layouts)
			view.MuLayout.RUnlock()
			document.GetDriverById("pad_stats").FillValue(
				fmt.Sprintf("%d palabras · %d caracteres · %d líneas", words, len([]rune(text)), lines))
			document.GetDriverById("pad_viewers").FillValue(fmt.Sprint(viewers))
		}

		document.Component.SetHandlerEventIn(func(data interface{}) { refresh() })
		document.Component.SetHandlerEventTime(2*time.Second, refresh)
		return document
	})

	fmt.Println("Pad example -> http://localhost:3009")
	log.Fatal(app.Listen(":3009"))
}
