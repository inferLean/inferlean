package interrupt

import (
	"context"
	"testing"
	"time"
)

func TestBusPublishNotifiesSubscribers(t *testing.T) {
	bus := NewBus(context.Background())
	defer bus.Close()

	first, unsubscribeFirst := bus.Subscribe()
	defer unsubscribeFirst()
	second, unsubscribeSecond := bus.Subscribe()
	defer unsubscribeSecond()

	bus.Publish()
	expectInterrupt(t, first)
	expectInterrupt(t, second)
}

func TestBusUnsubscribeRemovesSubscriber(t *testing.T) {
	bus := NewBus(context.Background())
	defer bus.Close()

	ch, unsubscribe := bus.Subscribe()
	unsubscribe()

	bus.Publish()
	expectNoInterrupt(t, ch)
}

func TestBusCloseNotifiesSubscribers(t *testing.T) {
	bus := NewBus(context.Background())

	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	bus.Close()
	expectInterrupt(t, ch)
}

func expectInterrupt(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected interrupt notification")
	}
}

func expectNoInterrupt(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("expected no interrupt notification")
	case <-time.After(20 * time.Millisecond):
	}
}
