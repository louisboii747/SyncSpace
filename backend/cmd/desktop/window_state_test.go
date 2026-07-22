package main

import "testing"

func TestNormaliseWindowState(t *testing.T) {
	state := normaliseWindowState(windowState{X: 2500, Y: 1800, Width: 320, Height: 200}, 1920, 1080)
	if state.X != -1 || state.Y != -1 {
		t.Fatalf("off-screen position was retained: %+v", state)
	}
	if state.Width != 900 || state.Height != 600 {
		t.Fatalf("minimum size was not enforced: %+v", state)
	}

	state = normaliseWindowState(windowState{X: 100, Y: 80, Width: 2600, Height: 1400}, 1920, 1080)
	if state.Width != 1920 || state.Height != 1080 || state.X != 100 || state.Y != 80 {
		t.Fatalf("screen bounds were not applied: %+v", state)
	}
}
