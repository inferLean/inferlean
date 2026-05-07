package interrupt

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

type Source interface {
	Subscribe() (<-chan struct{}, func())
}

type Publisher interface {
	Publish()
}

type Controller interface {
	Source
	Publisher
}

func Subscribe(source Source) (<-chan struct{}, func()) {
	if source == nil {
		return nil, func() {}
	}
	return source.Subscribe()
}

func Publish(publisher Publisher) {
	if publisher == nil {
		return
	}
	publisher.Publish()
}

type Bus struct {
	ctx      context.Context
	cancel   context.CancelFunc
	stop     context.CancelFunc
	stopOnce sync.Once
}

func NewBus(parent context.Context) *Bus {
	if parent == nil {
		parent = context.Background()
	}
	signalCtx, stop := signal.NotifyContext(parent, os.Interrupt)
	ctx, cancel := context.WithCancel(signalCtx)
	bus := &Bus{ctx: ctx, cancel: cancel, stop: stop}
	go bus.stopSignalsAfterInterrupt()
	return bus
}

func (b *Bus) Subscribe() (<-chan struct{}, func()) {
	if b == nil {
		return nil, func() {}
	}
	ch := make(chan struct{}, 1)
	done := make(chan struct{})
	stopped := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(stopped)
		select {
		case <-b.ctx.Done():
			select {
			case ch <- struct{}{}:
			default:
			}
		case <-done:
		}
	}()
	return ch, func() {
		once.Do(func() {
			close(done)
			<-stopped
		})
	}
}

func (b *Bus) Publish() {
	if b == nil {
		return
	}
	b.cancel()
	b.stopSignals()
}

func (b *Bus) Close() {
	if b == nil {
		return
	}
	b.cancel()
	b.stopSignals()
}

func (b *Bus) Context() context.Context {
	if b == nil {
		return context.Background()
	}
	return b.ctx
}

func (b *Bus) stopSignalsAfterInterrupt() {
	<-b.ctx.Done()
	b.stopSignals()
}

func (b *Bus) stopSignals() {
	b.stopOnce.Do(b.stop)
}
