package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/louisboii747/syncspace/backend/internal/frontend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var buildVersion = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println("SyncSpace", buildVersion)
		return
	}

	host := newDesktopHost()
	err := wails.Run(&options.App{
		Title:            "SyncSpace",
		Width:            1180,
		Height:           760,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 8, G: 17, B: 31, A: 255},
		AssetServer:      &assetserver.Options{Assets: frontend.Assets()},
		OnStartup:        host.startup,
		OnBeforeClose:    host.beforeClose,
		OnShutdown:       host.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "7e4f31e0-f072-4e8b-a565-9dc588ca674f",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if host.ctx != nil {
					wailsruntime.WindowUnminimise(host.ctx)
					wailsruntime.WindowShow(host.ctx)
				}
			},
		},
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Windows: &windows.Options{
			Theme:                windows.SystemDefault,
			IsZoomControlEnabled: false,
			DisablePinchZoom:     true,
		},
		Linux: &linux.Options{ProgramName: "syncspace"},
		Bind:  []interface{}{host},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "SyncSpace:", err)
		os.Exit(1)
	}
}

type DesktopHost struct {
	ctx     context.Context
	mu      sync.RWMutex
	backend *backendRuntime
	err     error
}

func newDesktopHost() *DesktopHost { return &DesktopHost{} }

func (h *DesktopHost) startup(ctx context.Context) {
	h.ctx = ctx
	restoreWindowState(ctx)
	go func() {
		backend, err := startBackend(ctx)
		h.mu.Lock()
		h.backend, h.err = backend, err
		h.mu.Unlock()
	}()
}

func (h *DesktopHost) shutdown(context.Context) {
	h.mu.RLock()
	backend := h.backend
	h.mu.RUnlock()
	if backend != nil {
		backend.stop()
	}
}

func (h *DesktopHost) beforeClose(ctx context.Context) bool {
	saveWindowState(ctx)
	return false
}

// StartupStatus lets the loading UI distinguish backend startup failure from
// temporary network unavailability without exposing internal error types.
func (h *DesktopHost) StartupStatus() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.err != nil {
		return map[string]any{"ready": false, "error": h.err.Error()}
	}
	return map[string]any{"ready": h.backend != nil}
}
