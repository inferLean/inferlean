package events

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/auth"
	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
)

const (
	pendingEventsDir = "events/pending"
	fileMode         = 0o600
	dirMode          = 0o700
	httpTimeout      = 10 * time.Second
)

type Emitter struct {
	dir    string
	client *http.Client
}

func NewEmitter() (*Emitter, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewEmitterAt(filepath.Join(home, config.DefaultDirName, pendingEventsDir), nil), nil
}

func NewEmitterAt(dir string, client *http.Client) *Emitter {
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}
	return &Emitter{dir: dir, client: client}
}

func NewWorkflowEvent(installationID, runID, command, stage, outcome string, metadata map[string]string) contracts.TelemetryEvent {
	return contracts.TelemetryEvent{
		EventID:        newID("evt"),
		EventType:      contracts.TelemetryEventTypeWorkflow,
		InstallationID: installationID,
		RunID:          runID,
		Command:        command,
		Stage:          stage,
		Outcome:        outcome,
		OccurredAt:     time.Now().UTC(),
		Metadata:       metadata,
	}
}

func NewCrashEvent(installationID, runID, command, stage, message string, metadata map[string]string) contracts.TelemetryEvent {
	return contracts.TelemetryEvent{
		EventID:        newID("crash"),
		EventType:      contracts.TelemetryEventTypeCrash,
		InstallationID: installationID,
		RunID:          runID,
		Command:        command,
		Stage:          stage,
		Outcome:        "error",
		Message:        message,
		OccurredAt:     time.Now().UTC(),
		Metadata:       metadata,
	}
}

func (e *Emitter) EmitAsync(backendURL string, authState config.AuthState, event contracts.TelemetryEvent) error {
	if err := e.enqueue(event); err != nil {
		return err
	}

	go func() {
		_ = e.Flush(context.Background(), backendURL, authState)
	}()
	return nil
}

func (e *Emitter) Flush(ctx context.Context, backendURL string, authState config.AuthState) error {
	if strings.TrimSpace(backendURL) == "" {
		return nil
	}

	files, err := os.ReadDir(e.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read pending events: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	events := make([]contracts.TelemetryEvent, 0, len(files))
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(e.dir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read pending event: %w", err)
		}

		var event contracts.TelemetryEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("decode pending event: %w", err)
		}

		events = append(events, event)
		paths = append(paths, path)
	}

	payload, err := json.Marshal(contracts.TelemetryBatch{Events: events})
	if err != nil {
		return fmt.Errorf("encode telemetry batch: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, auth.NormalizeBaseURL(backendURL)+"/api/v1/events", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build telemetry request: %w", err)
	}
	if authState.HasSession() {
		tokenType := authState.TokenType
		if strings.TrimSpace(tokenType) == "" {
			tokenType = "Bearer"
		}
		req.Header.Set("Authorization", tokenType+" "+authState.BearerToken())
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telemetry batch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("send telemetry batch: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove delivered event %s: %w", path, err)
		}
	}

	return nil
}

func (e *Emitter) enqueue(event contracts.TelemetryEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate telemetry event: %w", err)
	}

	if err := os.MkdirAll(e.dir, dirMode); err != nil {
		return fmt.Errorf("create pending event directory: %w", err)
	}

	path := filepath.Join(e.dir, event.EventID+".json")
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("encode telemetry event: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, fileMode); err != nil {
		return fmt.Errorf("persist telemetry event: %w", err)
	}
	return nil
}

func newID(prefix string) string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(buf[:])
}
