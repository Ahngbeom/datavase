package proto_test

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
)

// The goroutine that draws must not learn how slow the terminal is. If it
// could block here, a wedged client would stop dv reading rows from the
// database — and MySQL will not take another statement until the result set
// is drained, so cancellation and schema browsing would stop with it.
func TestPutNeverBlocks(t *testing.T) {
	q := proto.NewFrameQueue()
	defer q.Close()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			q.Put(screen.Frame{Cells: []screen.Cell{{X: i % 40, Y: 0, Main: 'x', Width: 1}}})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Put was still going after two seconds with nobody taking")
	}
}

// What piles up must be one frame, not a queue of them.
func TestWaitingFramesAreMerged(t *testing.T) {
	q := proto.NewFrameQueue()
	defer q.Close()

	q.Put(screen.Frame{Cells: []screen.Cell{
		{X: 1, Y: 0, Main: 'a', Width: 1},
		{X: 5, Y: 2, Main: 'b', Width: 1},
	}})
	q.Put(screen.Frame{Cells: []screen.Cell{
		{X: 1, Y: 0, Main: 'z', Width: 1},
	}})

	got, ok := q.Take(context.Background())
	if !ok {
		t.Fatal("Take returned nothing")
	}
	if len(got.Cells) != 2 {
		t.Fatalf("took %d cells, want 2: the cell only the first frame touched must survive", len(got.Cells))
	}

	byPos := map[[2]int]rune{}
	for _, c := range got.Cells {
		byPos[[2]int{c.X, c.Y}] = c.Main
	}
	if byPos[[2]int{1, 0}] != 'z' {
		t.Errorf("at 1,0 = %q, want 'z': the later value wins", byPos[[2]int{1, 0}])
	}
	if byPos[[2]int{5, 2}] != 'b' {
		t.Errorf("at 5,2 = %q, want 'b': it was never overwritten", byPos[[2]int{5, 2}])
	}

	if _, ok := q.Take(withTimeout(t, 50*time.Millisecond)); ok {
		t.Error("a second frame was waiting; the two should have been merged into one")
	}
}

// Take must return when the connection ends, or the writer goroutine outlives
// the client it was writing to.
func TestTakeReturnsOnClose(t *testing.T) {
	q := proto.NewFrameQueue()

	done := make(chan bool, 1)
	go func() {
		_, ok := q.Take(context.Background())
		done <- ok
	}()

	q.Close()

	select {
	case ok := <-done:
		if ok {
			t.Error("Take reported a frame after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Take still blocked two seconds after Close")
	}
}

func withTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}
