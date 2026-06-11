package view

import (
	"sync"
	"sync/atomic"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// Conn wraps a websocket connection with a per-connection write mutex so
// many goroutines (timers, broadcasts, event handlers) can push updates to
// the same browser safely without serializing writes across connections.
type Conn struct {
	ws     *websocket.Conn
	mu     sync.Mutex
	closed atomic.Bool
}

// WriteJSON sends a JSON message to the browser. It is safe for concurrent use.
func (c *Conn) WriteJSON(v interface{}) error {
	if c.closed.Load() {
		return websocket.ErrCloseSent
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(v)
}

// ReadMessage reads the next message from the browser.
func (c *Conn) ReadMessage() (int, []byte, error) {
	return c.ws.ReadMessage()
}

// Close marks the connection as closed and closes the underlying socket.
func (c *Conn) Close() error {
	c.closed.Store(true)
	return c.ws.Close()
}

// IsOpen reports whether the connection is still usable. Long-running
// component loops (e.g. clocks) should stop when this returns false.
func (c *Conn) IsOpen() bool {
	return !c.closed.Load()
}

var upgrader = websocket.FastHTTPUpgrader{
	CheckOrigin: func(ctx *fasthttp.RequestCtx) bool { return true },
}

// NewWebSocketHandler adapts a websocket session handler to a Fiber v3
// handler, upgrading the request with fasthttp/websocket.
func NewWebSocketHandler(handler func(*Conn)) fiber.Handler {
	return func(c fiber.Ctx) error {
		return upgrader.Upgrade(c.RequestCtx(), func(ws *websocket.Conn) {
			conn := &Conn{ws: ws}
			defer conn.Close()
			handler(conn)
		})
	}
}
