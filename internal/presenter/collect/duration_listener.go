package collect

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

func startCollectionDurationListener(enabled bool) (<-chan collectionDurationAction, func()) {
	if !enabled {
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
	actions := make(chan collectionDurationAction, 8)
	stop := make(chan struct{})
	done := make(chan struct{})
	go readCollectionDurationKey(stdinFD, stop, done, actions)
	return actions, func() {
		close(stop)
		<-done
		_ = syscall.SetNonblock(stdinFD, false)
		_ = term.Restore(stdinFD, state)
	}
}

func readCollectionDurationKey(stdinFD int, stop <-chan struct{}, done chan<- struct{}, actions chan<- collectionDurationAction) {
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
		if buffer[0] == 3 {
			process, err := os.FindProcess(os.Getpid())
			if err == nil {
				_ = process.Signal(os.Interrupt)
			}
			continue
		}
		action := mapCollectionDurationKeyAction(buffer[0])
		if action == collectionActionUnknown {
			continue
		}
		select {
		case actions <- action:
		default:
		}
	}
}

func mapCollectionDurationKeyAction(key byte) collectionDurationAction {
	switch key {
	case 'm':
		return collectionActionIncreaseMinute
	case 'M':
		return collectionActionDecreaseMinute
	case 's':
		return collectionActionIncreaseSeconds
	case 'S':
		return collectionActionDecreaseSeconds
	case 'c', 'C':
		return collectionActionStopAndAnalyze
	default:
		return collectionActionUnknown
	}
}

func isRetryableReadError(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EINTR)
}
