package chartanim

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestProgressRunsOncePerKey(t *testing.T) {
	var now time.Duration
	var canvas gfx.Canvas
	canvas.SetClock(func() time.Duration { return now })
	var state State

	if got := state.Progress(&canvas, "chart-a", 100*time.Millisecond); got != 0 {
		t.Fatalf("first paint = %v, want 0", got)
	}
	now = 50 * time.Millisecond
	if got := state.Progress(&canvas, "chart-a", 100*time.Millisecond); got <= 0 || got >= 1 {
		t.Fatalf("mid reveal = %v, want 0..1", got)
	}
	now = 100 * time.Millisecond
	if got := state.Progress(&canvas, "chart-a", 100*time.Millisecond); got != 1 {
		t.Fatalf("finished reveal = %v, want 1", got)
	}

	now = 200 * time.Millisecond
	if got := state.Progress(&canvas, "chart-b", 100*time.Millisecond); got != 0 {
		t.Fatalf("new key = %v, want a fresh 0", got)
	}
}

func TestProgressPaintsStillWithoutClockOrMotion(t *testing.T) {
	var canvas gfx.Canvas
	var state State
	if got := state.Progress(&canvas, "chart", time.Second); got != 1 {
		t.Fatalf("clock-less progress = %v, want 1", got)
	}

	var now time.Duration
	canvas.SetClock(func() time.Duration { return now })
	canvas.SetReduceMotion(true)
	if got := state.Progress(&canvas, "chart", time.Second); got != 1 {
		t.Fatalf("reduced-motion progress = %v, want 1", got)
	}
}
