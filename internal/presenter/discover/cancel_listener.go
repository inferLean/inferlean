package discover

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/interrupt"
	"golang.org/x/term"
)

func startCancelCurrentListener(nonInteractive bool, interrupts interrupt.Publisher) (<-chan struct{}, func()) {
	if nonInteractive {
		return nil, func() {}
	}
	stdinFD := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFD) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil, func() {}
	}
	state, err := term.MakeRaw(stdinFD)
	if err != nil {
		return nil, func() {}
	}
	if err := syscall.SetNonblock(stdinFD, true); err != nil {
		_ = term.Restore(stdinFD, state)
		return nil, func() {}
	}
	cancelCurrent := make(chan struct{}, 1)
	stop := make(chan struct{})
	done := make(chan struct{})
	go readCancelKey(stdinFD, stop, done, cancelCurrent, interrupts)
	var stopOnce sync.Once
	return cancelCurrent, func() {
		stopOnce.Do(func() {
			close(stop)
			<-done
			_ = syscall.SetNonblock(stdinFD, false)
			_ = term.Restore(stdinFD, state)
		})
	}
}

func readCancelKey(stdinFD int, stop <-chan struct{}, done chan<- struct{}, cancelCurrent chan<- struct{}, interrupts interrupt.Publisher) {
	defer close(done)
	buffer := make([]byte, 1)
	for {
		select {
		case <-stop:
			return
		default:
		}
		n, err := syscall.Read(stdinFD, buffer)
		if err != nil {
			if isRetryableReadError(err) {
				time.Sleep(20 * time.Millisecond)
				continue
			}
			return
		}
		if n == 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		switch buffer[0] {
		case 'c', 'C':
			select {
			case cancelCurrent <- struct{}{}:
			default:
			}
		case 3:
			interrupt.Publish(interrupts)
		}
	}
}

func isRetryableReadError(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EINTR)
}
