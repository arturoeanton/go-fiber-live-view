// Package assets embeds the static files served by liveview pages:
// the Go wasm runtime (wasm_exec.js), the compiled client (json.wasm)
// and the default component theme (liveview.css).
package assets

import "embed"

//go:embed json.wasm wasm_exec.js liveview.css
var FS embed.FS
