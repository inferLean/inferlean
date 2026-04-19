package types

import "time"

type UploadAck struct {
	UploadID       string    `json:"upload_id"`
	RunID          string    `json:"run_id"`
	InstallationID string    `json:"installation_id"`
	Status         string    `json:"status"`
	StatusURL      string    `json:"status_url,omitempty"`
	ReportURL      string    `json:"report_url,omitempty"`
	ReceivedAt     time.Time `json:"received_at"`
}

type AuthState struct {
	BackendURL   string    `json:"backend_url,omitempty"`
	Issuer       string    `json:"issuer,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	UseIDToken   bool      `json:"use_id_token,omitempty"`
}

func (a AuthState) HasSession() bool {
	return a.BackendURL != "" && a.Issuer != "" && a.ClientID != "" && a.BearerToken() != ""
}

func (a AuthState) BearerToken() string {
	if a.UseIDToken && a.IDToken != "" {
		return a.IDToken
	}
	if a.AccessToken != "" {
		return a.AccessToken
	}
	return a.IDToken
}
