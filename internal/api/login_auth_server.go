package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func startAuthorizationServer(expectedState string) (<-chan string, <-chan error, func(), error) {
	listener, err := net.Listen("tcp", callbackAddress)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("start callback listener: %w", err)
	}
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotState := r.URL.Query().Get("state"); gotState != expectedState {
			errCh <- fmt.Errorf("state mismatch")
			return
		}
		if loginErr := r.URL.Query().Get("error"); loginErr != "" {
			errCh <- fmt.Errorf("login error: %s", loginErr)
			return
		}
		resultCh <- strings.TrimSpace(r.URL.Query().Get("code"))
		_, _ = io.WriteString(w, "InferLean login complete. You can close this tab.\n")
	})}
	go func() { _ = server.Serve(listener) }()
	closeFn := func() {
		_ = listener.Close()
		shutdownServer(server)
	}
	return resultCh, errCh, closeFn, nil
}

func shutdownServer(server *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func waitForAuthorizationCode(ctx context.Context, resultCh <-chan string, errCh <-chan error) (string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return "", fmt.Errorf("timed out waiting for login callback")
	case err := <-errCh:
		return "", err
	case code := <-resultCh:
		if code == "" {
			return "", fmt.Errorf("authorization code missing")
		}
		return code, nil
	}
}
