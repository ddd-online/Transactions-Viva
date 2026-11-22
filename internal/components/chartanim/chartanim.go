// Package chartanim owns the one-shot reveal shared by the app's charts.
//
// A chart paints its still form when no clock is available or the session asks
// for reduced motion. Otherwise the first paint after the data key changes
// starts a short eased pass; every paint during the pass schedules the next
// frame, and the last paint stops asking. This is a temporary reason to draw,
// never a heartbeat.
package chartanim

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// Duration is the reveal used by statistics and analysis charts.
const Duration = 420 * time.Millisecond

// State is one chart's reveal progress. Keep it in the page model: it survives
// paints but belongs to one chart.
type State struct {
	key     string
	start   time.Duration
	started bool
}

// Progress returns 0..1 for key. A new key restarts the pass; a clock-less
// canvas, reduced motion and a zero duration paint the finished chart.
func (s *State) Progress(canvas *gfx.Canvas, key string, duration time.Duration) float32 {
	if canvas == nil || duration <= 0 || !canvas.HasClock() || canvas.ReduceMotion() {
		return 1
	}
	now := canvas.Now()
	if !s.started || s.key != key {
		s.key, s.start, s.started = key, now, true
	}
	elapsed := now - s.start
	if elapsed <= 0 {
		canvas.RequestFrame()
		return 0
	}
	if elapsed >= duration {
		return 1
	}
	canvas.RequestFrame()
	t := float32(elapsed) / float32(duration)
	// Ease-out cubic: most of the reveal happens early, then the curve settles.
	inverse := 1 - t
	return 1 - inverse*inverse*inverse
}
