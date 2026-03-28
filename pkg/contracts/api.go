package contracts

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TelemetryEventTypeWorkflow = "workflow"
	TelemetryEventTypeCrash    = "crash"
)

type AuthConfig struct {
	Issuer     string   `json:"issuer"`
	ClientID   string   `json:"client_id"`
	Scopes     []string `json:"scopes,omitempty"`
	UseIDToken bool     `json:"use_id_token,omitempty"`
}

func (c AuthConfig) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Issuer) == "" {
		errs = append(errs, errors.New("issuer is required"))
	}
	if strings.TrimSpace(c.ClientID) == "" {
		errs = append(errs, errors.New("client_id is required"))
	}
	if len(c.Scopes) == 0 {
		errs = append(errs, errors.New("at least one scope is required"))
	}

	return errors.Join(errs...)
}

type ArtifactUploadAck struct {
	UploadID       string    `json:"upload_id"`
	RunID          string    `json:"run_id"`
	InstallationID string    `json:"installation_id"`
	Status         string    `json:"status"`
	ReceivedAt     time.Time `json:"received_at"`
	StatusURL      string    `json:"status_url,omitempty"`
}

func (a ArtifactUploadAck) Validate() error {
	var errs []error

	if strings.TrimSpace(a.UploadID) == "" {
		errs = append(errs, errors.New("upload_id is required"))
	}
	if strings.TrimSpace(a.RunID) == "" {
		errs = append(errs, errors.New("run_id is required"))
	}
	if strings.TrimSpace(a.InstallationID) == "" {
		errs = append(errs, errors.New("installation_id is required"))
	}
	switch a.Status {
	case "accepted":
	default:
		errs = append(errs, fmt.Errorf("status must be accepted"))
	}
	if a.ReceivedAt.IsZero() {
		errs = append(errs, errors.New("received_at is required"))
	}

	return errors.Join(errs...)
}

type InstallationClaimRequest struct {
	InstallationID string `json:"installation_id"`
}

func (r InstallationClaimRequest) Validate() error {
	if strings.TrimSpace(r.InstallationID) == "" {
		return errors.New("installation_id is required")
	}
	return nil
}

type InstallationClaimResponse struct {
	InstallationID   string    `json:"installation_id"`
	Subject          string    `json:"subject"`
	Email            string    `json:"email,omitempty"`
	ClaimedAt        time.Time `json:"claimed_at"`
	AssignedRunCount int       `json:"assigned_run_count"`
}

type RunSummary struct {
	UploadID         string    `json:"upload_id"`
	RunID            string    `json:"run_id"`
	InstallationID   string    `json:"installation_id"`
	SchemaVersion    string    `json:"schema_version"`
	CollectorVersion string    `json:"collector_version"`
	ReceivedAt       time.Time `json:"received_at"`
}

type RunListResponse struct {
	Runs []RunSummary `json:"runs"`
}

type TelemetryBatch struct {
	Events []TelemetryEvent `json:"events"`
}

func (b TelemetryBatch) Validate() error {
	if len(b.Events) == 0 {
		return errors.New("events must not be empty")
	}

	var errs []error
	for idx, event := range b.Events {
		if err := event.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("events[%d]: %w", idx, err))
		}
	}

	return errors.Join(errs...)
}

type TelemetryEvent struct {
	EventID        string            `json:"event_id"`
	EventType      string            `json:"event_type"`
	InstallationID string            `json:"installation_id,omitempty"`
	RunID          string            `json:"run_id,omitempty"`
	Command        string            `json:"command,omitempty"`
	Stage          string            `json:"stage,omitempty"`
	Outcome        string            `json:"outcome,omitempty"`
	Message        string            `json:"message,omitempty"`
	OccurredAt     time.Time         `json:"occurred_at"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

func (e TelemetryEvent) Validate() error {
	var errs []error

	if strings.TrimSpace(e.EventID) == "" {
		errs = append(errs, errors.New("event_id is required"))
	}
	switch e.EventType {
	case TelemetryEventTypeWorkflow, TelemetryEventTypeCrash:
	default:
		errs = append(errs, errors.New("event_type must be workflow or crash"))
	}
	if e.OccurredAt.IsZero() {
		errs = append(errs, errors.New("occurred_at is required"))
	}

	return errors.Join(errs...)
}

type TelemetryIngestAck struct {
	Accepted   int       `json:"accepted"`
	ReceivedAt time.Time `json:"received_at"`
}
