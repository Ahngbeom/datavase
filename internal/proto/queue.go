package proto

import (
	"context"
	"sync"

	"github.com/Ahngbeom/datavase/internal/screen"
)

// FrameQueue holds at most one frame for a writer that has fallen behind.
//
// Put never blocks, which is the entire point: it is called from the
// goroutine that draws the interface, and that goroutine is also the one
// reading rows off the connection. A slow, suspended or dead terminal that
// could stall it would stall the statement too — and because MySQL will not
// accept another statement until the result set is drained, cancellation and
// schema browsing would stall with it.
//
// When a frame is already waiting the two are merged rather than one being
// dropped. See screen.Frame.Merge for why that is sound, and for what
// dropping would cost.
type FrameQueue struct {
	mu      sync.Mutex
	frame   screen.Frame
	waiting bool
	ready   chan struct{}
	done    chan struct{}
	once    sync.Once
}

func NewFrameQueue() *FrameQueue {
	return &FrameQueue{
		ready: make(chan struct{}, 1),
		done:  make(chan struct{}),
	}
}

// Put leaves f for the next Take, merging it with anything already waiting.
func (q *FrameQueue) Put(f screen.Frame) {
	q.mu.Lock()
	if q.waiting {
		q.frame = q.frame.Merge(f)
	} else {
		q.frame = f
		q.waiting = true
	}
	q.mu.Unlock()

	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// Take blocks for a frame. It reports false when the queue is closed or ctx
// ends, which is how the writer goroutine learns to stop.
func (q *FrameQueue) Take(ctx context.Context) (screen.Frame, bool) {
	for {
		q.mu.Lock()
		if q.waiting {
			f := q.frame
			q.frame = screen.Frame{}
			q.waiting = false
			q.mu.Unlock()
			return f, true
		}
		q.mu.Unlock()

		select {
		case <-q.ready:
		case <-q.done:
			return screen.Frame{}, false
		case <-ctx.Done():
			return screen.Frame{}, false
		}
	}
}

// Close releases anything blocked in Take. It is safe to call twice.
func (q *FrameQueue) Close() {
	q.once.Do(func() { close(q.done) })
}
