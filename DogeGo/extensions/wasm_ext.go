// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const wasmMaxModuleBytes = 16 << 20

// WasmExtension runs entry.wasm with export-per-RPC method (sandboxed; no WASI by default).
type WasmExtension struct {
	manifest Manifest
	extDir   string
	wasmPath string
	mu       sync.Mutex
	runtime  wazero.Runtime
	module   api.Module
	compiled wazero.CompiledModule
	alive    bool
}

// NewWasmExtension loads a wasm module from an on-disk install dir.
func NewWasmExtension(dir string, man Manifest) (*WasmExtension, error) {
	wasmPath, err := resolveWasmModulePath(dir, man.Entry.Wasm)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, err
	}
	if len(raw) > wasmMaxModuleBytes {
		return nil, fmt.Errorf("wasm module exceeds size limit")
	}
	return &WasmExtension{
		manifest: man,
		extDir:   dir,
		wasmPath: wasmPath,
	}, nil
}

func resolveWasmModulePath(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("entry.wasm required")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid wasm path")
	}
	clean := filepath.Join(dir, name)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs, absDir+string(os.PathSeparator)) && abs != absDir {
		return "", fmt.Errorf("wasm path escape")
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("wasm module %q: %w", name, err)
	}
	return abs, nil
}

func (w *WasmExtension) Manifest() Manifest { return w.manifest }

func (w *WasmExtension) OnEnable(ctx context.Context, host Host) error {
	if host == nil {
		return fmt.Errorf("wasm: host required")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.alive {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	raw, err := os.ReadFile(w.wasmPath)
	if err != nil {
		return err
	}
	rt := wazero.NewRuntime(ctx)
	hostBuilder := rt.NewHostModuleBuilder("dogego")
	hostBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, p, n uint32) {
			if host != nil && n > 0 && n < 4096 {
				b, ok := m.Memory().Read(p, n)
				if ok {
					host.Log(string(b))
				}
			}
		}).
		WithParameterNames("ptr", "len").
		Export("log")
	if _, err := hostBuilder.Instantiate(ctx); err != nil {
		_ = rt.Close(ctx)
		return fmt.Errorf("wasm host module: %w", err)
	}
	compiled, err := rt.CompileModule(ctx, raw)
	if err != nil {
		_ = rt.Close(ctx)
		return fmt.Errorf("wasm compile: %w", err)
	}
	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(w.manifest.ID))
	if err != nil {
		_ = rt.Close(ctx)
		return fmt.Errorf("wasm instantiate: %w", err)
	}
	w.runtime = rt
	w.compiled = compiled
	w.module = mod
	w.alive = true
	host.Log(fmt.Sprintf("wasm %s loaded", w.manifest.ID))
	if fn := mod.ExportedFunction("dogego_on_enable"); fn != nil {
		if _, err := fn.Call(ctx); err != nil {
			_ = w.closeLocked(ctx)
			return fmt.Errorf("dogego_on_enable: %w", err)
		}
	}
	return nil
}

func (w *WasmExtension) OnDisable() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	ctx := context.Background()
	if w.module != nil {
		if fn := w.module.ExportedFunction("dogego_on_disable"); fn != nil {
			_, _ = fn.Call(ctx)
		}
	}
	return w.closeLocked(ctx)
}

func (w *WasmExtension) closeLocked(ctx context.Context) error {
	if !w.alive && w.runtime == nil {
		return nil
	}
	if w.module != nil {
		_ = w.module.Close(ctx)
		w.module = nil
	}
	if w.compiled != nil {
		_ = w.compiled.Close(ctx)
		w.compiled = nil
	}
	if w.runtime != nil {
		_ = w.runtime.Close(ctx)
		w.runtime = nil
	}
	w.alive = false
	return nil
}

func (w *WasmExtension) HandleRPC(method string, params []json.RawMessage, host Host) (interface{}, error) {
	if !w.manifest.HasPermission("rpc_register") {
		return nil, fmt.Errorf("extension lacks rpc_register permission")
	}
	switch method {
	case "info":
		w.mu.Lock()
		alive := w.alive
		w.mu.Unlock()
		return map[string]interface{}{
			"id":      w.manifest.ID,
			"runtime": "wasm",
			"version": w.manifest.Version,
			"wasm":    w.manifest.Entry.Wasm,
			"alive":   alive,
		}, nil
	}
	w.mu.Lock()
	mod := w.module
	alive := w.alive
	w.mu.Unlock()
	if !alive || mod == nil {
		return nil, fmt.Errorf("wasm not running")
	}
	fn := mod.ExportedFunction(method)
	if fn == nil {
		fn = mod.ExportedFunction("dogego_rpc")
	}
	if fn == nil {
		return nil, fmt.Errorf("wasm export %q not found", method)
	}
	ctx := context.Background()
	def := fn.Definition()
	if len(def.ParamTypes()) == 0 {
		res, err := fn.Call(ctx)
		if err != nil {
			return nil, err
		}
		return wasmScalarResult(method, res), nil
	}
	// Single i32 param: pass JSON param length as convention for future guests.
	if len(def.ParamTypes()) == 1 && len(params) > 0 {
		var s string
		_ = json.Unmarshal(params[0], &s)
		ptr, alloc, err := writeGuestString(ctx, mod, s)
		if err != nil {
			return nil, err
		}
		defer freeGuest(ctx, mod, alloc)
		res, err := fn.Call(ctx, uint64(ptr), uint64(len(s)))
		if err != nil {
			return nil, err
		}
		return wasmScalarResult(method, res), nil
	}
	res, err := fn.Call(ctx)
	if err != nil {
		return nil, err
	}
	return wasmScalarResult(method, res), nil
}

func wasmScalarResult(method string, res []uint64) interface{} {
	if len(res) == 0 {
		if method == "ping" {
			return "pong"
		}
		return nil
	}
	v := uint32(res[0])
	if method == "ping" {
		if v == 0 {
			return "pong"
		}
		return map[string]interface{}{"pong": v}
	}
	return v
}

func writeGuestString(ctx context.Context, mod api.Module, s string) (ptr, alloc uint32, err error) {
	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		return 0, 0, fmt.Errorf("wasm guest missing malloc for string args")
	}
	n := len(s)
	if n == 0 {
		return 0, 0, nil
	}
	out, err := malloc.Call(ctx, uint64(n))
	if err != nil || len(out) == 0 {
		return 0, 0, fmt.Errorf("malloc failed")
	}
	ptr = uint32(out[0])
	if !mod.Memory().Write(ptr, []byte(s)) {
		return 0, 0, fmt.Errorf("guest memory write failed")
	}
	return ptr, ptr, nil
}

func freeGuest(ctx context.Context, mod api.Module, ptr uint32) {
	if ptr == 0 {
		return
	}
	if free := mod.ExportedFunction("free"); free != nil {
		_, _ = free.Call(ctx, uint64(ptr))
	}
}

// RPCMethods implements RPCProvider.
func (w *WasmExtension) RPCMethods() []RPCMethod {
	return w.manifest.AdvertisedRPCMethods()
}
