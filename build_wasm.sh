cd wasm/
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ../liveview/assets/
GOOS=js GOARCH=wasm go build -o  ../liveview/assets/json.wasm
cd -