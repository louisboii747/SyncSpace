package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type windowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

func restoreWindowState(ctx context.Context) {
	path, err := windowStatePath()
	if err != nil {
		return
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var state windowState
	if json.Unmarshal(contents, &state) != nil {
		return
	}
	screens, err := wailsruntime.ScreenGetAll(ctx)
	if err != nil || len(screens) == 0 {
		return
	}
	primary := screens[0]
	for _, screen := range screens {
		if screen.IsPrimary {
			primary = screen
			break
		}
	}
	state = normaliseWindowState(state, primary.Size.Width, primary.Size.Height)
	wailsruntime.WindowSetSize(ctx, state.Width, state.Height)
	if state.X >= 0 && state.Y >= 0 {
		wailsruntime.WindowSetPosition(ctx, state.X, state.Y)
	} else {
		wailsruntime.WindowCenter(ctx)
	}
	if state.Maximised {
		wailsruntime.WindowMaximise(ctx)
	}
}

func saveWindowState(ctx context.Context) {
	if ctx == nil {
		return
	}
	width, height := wailsruntime.WindowGetSize(ctx)
	x, y := wailsruntime.WindowGetPosition(ctx)
	state := windowState{X: x, Y: y, Width: width, Height: height, Maximised: wailsruntime.WindowIsMaximised(ctx)}
	path, err := windowStatePath()
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	contents, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, append(contents, '\n'), 0o600)
	}
}

func windowStatePath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "SyncSpace", "window.json"), nil
}

func normaliseWindowState(state windowState, screenWidth, screenHeight int) windowState {
	if state.Width < 900 {
		state.Width = 900
	}
	if state.Height < 600 {
		state.Height = 600
	}
	if screenWidth > 0 && state.Width > screenWidth {
		state.Width = screenWidth
	}
	if screenHeight > 0 && state.Height > screenHeight {
		state.Height = screenHeight
	}
	if state.X < 0 || state.Y < 0 || state.X+100 > screenWidth || state.Y+100 > screenHeight {
		state.X, state.Y = -1, -1
	}
	return state
}
