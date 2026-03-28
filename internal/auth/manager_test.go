package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestLoginCompletesBrowserFlowAndReturnsReusableSession(t *testing.T) {
	tokenIssuedAt := time.Unix(1700000000, 0).UTC()

	var authServer *httptest.Server
	authServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/config":
			writeJSON(t, w, contracts.AuthConfig{
				Issuer:     authServer.URL,
				ClientID:   "inferlean-cli",
				Scopes:     []string{"openid", "profile", "email"},
				UseIDToken: true,
			})
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"authorization_endpoint": authServer.URL + "/auth",
				"token_endpoint":         authServer.URL + "/token",
			})
		case "/auth":
			http.Redirect(w, r, "http://"+callbackAddress+callbackPath+"?code=test-code&state="+r.URL.Query().Get("state"), http.StatusFound)
		case "/token":
			writeJSON(t, w, map[string]any{
				"access_token":  "access-token",
				"id_token":      "id-token",
				"refresh_token": "refresh-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer authServer.Close()

	manager := NewManagerWithClient(authServer.Client())
	manager.now = func() time.Time { return tokenIssuedAt }
	manager.openBrowser = func(target string) error {
		resp, err := authServer.Client().Get(target)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	session, err := manager.Login(context.Background(), authServer.URL, nil)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.BackendURL != authServer.URL {
		t.Fatalf("BackendURL = %q, want %q", session.BackendURL, authServer.URL)
	}
	if session.BearerToken() != "id-token" {
		t.Fatalf("BearerToken() = %q, want %q", session.BearerToken(), "id-token")
	}
	if !session.ExpiresAt.Equal(tokenIssuedAt.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %s, want %s", session.ExpiresAt, tokenIssuedAt.Add(time.Hour))
	}
	if !session.HasSession() {
		t.Fatal("HasSession() = false, want true")
	}
}

func TestEnsureValidRefreshesExpiredSession(t *testing.T) {
	refreshedAt := time.Unix(1700003600, 0).UTC()

	var authServer *httptest.Server
	authServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"authorization_endpoint": authServer.URL + "/auth",
				"token_endpoint":         authServer.URL + "/token",
			})
		case "/token":
			writeJSON(t, w, map[string]any{
				"access_token":  "new-access-token",
				"id_token":      "new-id-token",
				"refresh_token": "new-refresh-token",
				"token_type":    "Bearer",
				"expires_in":    1800,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer authServer.Close()

	manager := NewManagerWithClient(authServer.Client())
	manager.now = func() time.Time { return refreshedAt }

	updated, err := manager.EnsureValid(context.Background(), config.AuthState{
		BackendURL:   authServer.URL,
		Issuer:       authServer.URL,
		ClientID:     "inferlean-cli",
		TokenType:    "Bearer",
		AccessToken:  "old-access-token",
		IDToken:      "old-id-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    refreshedAt.Add(-time.Minute),
		UseIDToken:   true,
	})
	if err != nil {
		t.Fatalf("EnsureValid() error = %v", err)
	}
	if updated.BearerToken() != "new-id-token" {
		t.Fatalf("BearerToken() = %q, want %q", updated.BearerToken(), "new-id-token")
	}
	if updated.RefreshToken != "new-refresh-token" {
		t.Fatalf("RefreshToken = %q, want %q", updated.RefreshToken, "new-refresh-token")
	}
	if !updated.ExpiresAt.Equal(refreshedAt.Add(30 * time.Minute)) {
		t.Fatalf("ExpiresAt = %s, want %s", updated.ExpiresAt, refreshedAt.Add(30*time.Minute))
	}
}

func TestLoginFallsBackToDexIssuerWhenBackendAuthConfigIsMissing(t *testing.T) {
	tokenIssuedAt := time.Unix(1700000000, 0).UTC()

	var authServer *httptest.Server
	authServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/config":
			http.NotFound(w, r)
		case "/dex/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"authorization_endpoint": authServer.URL + "/dex/auth",
				"token_endpoint":         authServer.URL + "/dex/token",
			})
		case "/dex/auth":
			if got := r.URL.Query().Get("client_id"); got != defaultClientID {
				t.Fatalf("client_id = %q, want %q", got, defaultClientID)
			}
			http.Redirect(w, r, "http://"+callbackAddress+callbackPath+"?code=test-code&state="+r.URL.Query().Get("state"), http.StatusFound)
		case "/dex/token":
			writeJSON(t, w, map[string]any{
				"access_token":  "access-token",
				"id_token":      "id-token",
				"refresh_token": "refresh-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer authServer.Close()

	manager := NewManagerWithClient(authServer.Client())
	manager.now = func() time.Time { return tokenIssuedAt }
	manager.openBrowser = func(target string) error {
		resp, err := authServer.Client().Get(target)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	session, err := manager.Login(context.Background(), authServer.URL, nil)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.Issuer != authServer.URL+"/dex" {
		t.Fatalf("Issuer = %q, want %q", session.Issuer, authServer.URL+"/dex")
	}
	if session.ClientID != defaultClientID {
		t.Fatalf("ClientID = %q, want %q", session.ClientID, defaultClientID)
	}
}

func TestClaimInstallationSendsInstallationIDWithBearerToken(t *testing.T) {
	var apiServer *httptest.Server
	apiServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"authorization_endpoint": apiServer.URL + "/auth",
				"token_endpoint":         apiServer.URL + "/token",
			})
		case "/api/v1/installations/claim":
			if got := r.Header.Get("Authorization"); got != "Bearer id-token" {
				t.Fatalf("Authorization header = %q, want %q", got, "Bearer id-token")
			}
			var payload contracts.InstallationClaimRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if payload.InstallationID != "inst-456" {
				t.Fatalf("InstallationID = %q, want %q", payload.InstallationID, "inst-456")
			}
			writeJSON(t, w, contracts.InstallationClaimResponse{
				InstallationID:   payload.InstallationID,
				Subject:          "user-123",
				Email:            "user@example.com",
				ClaimedAt:        time.Unix(1700000000, 0).UTC(),
				AssignedRunCount: 3,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	manager := NewManagerWithClient(apiServer.Client())
	response, updated, err := manager.ClaimInstallation(context.Background(), config.AuthState{
		BackendURL:   apiServer.URL,
		Issuer:       apiServer.URL,
		ClientID:     defaultClientID,
		TokenType:    "Bearer",
		IDToken:      "id-token",
		RefreshToken: "refresh-token",
		UseIDToken:   true,
		ExpiresAt:    time.Now().Add(time.Hour),
	}, "inst-456")
	if err != nil {
		t.Fatalf("ClaimInstallation() error = %v", err)
	}
	if response.AssignedRunCount != 3 {
		t.Fatalf("AssignedRunCount = %d, want %d", response.AssignedRunCount, 3)
	}
	if updated.BearerToken() != "id-token" {
		t.Fatalf("BearerToken() = %q, want %q", updated.BearerToken(), "id-token")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}
