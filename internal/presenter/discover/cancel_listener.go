package discover

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

func startCancelCurrentListener(noInteractive bool) (<-chan struct{}, func()) {
	if noInteractive {
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
	go readCancelKey(stdinFD, stop, done, cancelCurrent)
	return cancelCurrent, func() {
		close(stop)
		<-done
		_ = syscall.SetNonblock(stdinFD, false)
		_ = term.Restore(stdinFD, state)
	}
}

func readCancelKey(stdinFD int, stop <-chan struct{}, done chan<- struct{}, cancelCurrent chan<- struct{}) {
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
			process, err := os.FindProcess(os.Getpid())
			if err == nil {
				_ = process.Signal(os.Interrupt)
			}
		}
	}
}

func isRetryableReadError(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EINTR)
}
