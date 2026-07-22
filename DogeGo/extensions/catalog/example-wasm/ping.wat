;; Minimal DogeGo wasm extension example.
;; Build: wat2wasm ping.wat -o ping.wasm
(module
  (func (export "ping") (result i32)
    i32.const 42)
  (func (export "dogego_on_enable")))
