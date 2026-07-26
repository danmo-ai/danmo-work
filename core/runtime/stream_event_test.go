package runtime

import (
	"context"
	"testing"
)

func TestSubscribeBufferHoldsBurstsWithoutDrop(t *testing.T) {
	m := NewStreamEventManager(nil)
	ch := m.Subscribe("s1")
	defer m.Unsubscribe("s1", ch)

	const n = 200
	for i := 0; i < n; i++ {
		m.Publish(context.Background(), "s1", "t1", "agent.message", map[string]any{"i": i})
	}

	got := 0
	for {
		select {
		case <-ch:
			got++
		default:
			if got != n {
				t.Fatalf("dropped events: got %d want %d", got, n)
			}
			return
		}
	}
}

func TestPublishDoesNotBlockWhenSubscriberSlow(t *testing.T) {
	m := NewStreamEventManager(nil)
	ch := m.Subscribe("s1")
	defer m.Unsubscribe("s1", ch)

	// Fill the buffer, then publish one more (must not block).
	for i := 0; i < 300; i++ {
		m.Publish(context.Background(), "s1", "t1", "agent.message", map[string]any{"i": i})
	}
}
