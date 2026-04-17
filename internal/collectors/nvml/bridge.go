package nvml

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type Bridge struct {
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
	lastRaw  string
}

type BridgeResult struct {
	Available bool
	Reason    string
	Endpoint  string
	Bridge    *Bridge
}

func StartBridge() BridgeResult {
	if err := CheckAvailability(); err != nil {
		return BridgeResult{Available: false, Reason: err.Error()}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return BridgeResult{Available: false, Reason: err.Error()}
	}
	bridge := &Bridge{listener: listener}
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", bridge.handleMetrics)
	bridge.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		_ = bridge.server.Serve(listener)
	}()
	return BridgeResult{
		Available: true,
		Endpoint:  fmt.Sprintf("http://%s/metrics", listener.Addr().String()),
		Bridge:    bridge,
	}
}

func (b *Bridge) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	metrics, raw, err := BuildPromMetrics(ctx)
	if err != nil {
		http.Error(w, "nvml bridge unavailable", http.StatusServiceUnavailable)
		return
	}
	b.setLastRaw(raw)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(metrics + "\n"))
}

func (b *Bridge) setLastRaw(raw string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastRaw = raw
}

func (b *Bridge) LastRaw() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastRaw
}

func (b *Bridge) Stop(ctx context.Context) error {
	if b == nil || b.server == nil {
		return nil
	}
	return b.server.Shutdown(ctx)
}
