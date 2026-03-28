package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/config"
	"github.com/inferLean/inferlean/pkg/contracts"
)

const (
	callbackAddress    = "127.0.0.1:38080"
	callbackPath       = "/oauth/callback"
	loginTimeout       = 2 * time.Minute
	defaultHTTPTimeout = 15 * time.Second
	defaultClientID    = "inferlean-cli"
)

var defaultScopes = []string{"openid", "profile", "email", "offline_access"}

type Manager struct {
	client      *http.Client
	openBrowser func(string) error
	now         func() time.Time
}

type discoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

type callbackResult struct {
	Code  string
	State string
	Err   string
}

func NewManager() *Manager {
	return NewManagerWithClient(&http.Client{Timeout: defaultHTTPTimeout})
}

func NewManagerWithClient(client *http.Client) *Manager {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &Manager{
		client:      client,
		openBrowser: defaultBrowserOpener,
		now:         time.Now,
	}
}

func (m *Manager) WithBrowserOpener(opener func(string) error) *Manager {
	m.openBrowser = opener
	return m
}

func NormalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func (m *Manager) Login(ctx context.Context, backendURL string, notify func(string)) (config.AuthState, error) {
	backendURL = NormalizeBaseURL(backendURL)
	if backendURL == "" {
		return config.AuthState{}, errors.New("backend URL is required")
	}

	authConfig, err := m.fetchAuthConfig(ctx, backendURL)
	if err != nil {
		return config.AuthState{}, err
	}
	discovery, err := m.fetchDiscovery(ctx, authConfig.Issuer)
	if err != nil {
		return config.AuthState{}, err
	}

	state, err := randomURLString(24)
	if err != nil {
		return config.AuthState{}, err
	}
	verifier, err := randomURLString(48)
	if err != nil {
		return config.AuthState{}, err
	}
	challenge := buildCodeChallenge(verifier)
	redirectURI := "http://" + callbackAddress + callbackPath

	listener, err := net.Listen("tcp", callbackAddress)
	if err != nil {
		return config.AuthState{}, fmt.Errorf("start login callback listener: %w", err)
	}
	defer listener.Close()

	codeCh := make(chan callbackResult, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		codeCh <- callbackResult{
			Code:  req.URL.Query().Get("code"),
			State: req.URL.Query().Get("state"),
			Err:   req.URL.Query().Get("error"),
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "InferLean login complete. You can close this tab.\n")
	})}
	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	loginURL := buildAuthorizationURL(discovery.AuthorizationEndpoint, authConfig, redirectURI, state, verifier, challenge)
	if notify != nil {
		notify(loginURL)
	}
	_ = m.openBrowser(loginURL)

	waitCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	var callback callbackResult
	select {
	case <-waitCtx.Done():
		return config.AuthState{}, errors.New("timed out waiting for browser login to finish")
	case callback = <-codeCh:
	}
	if callback.Err != "" {
		return config.AuthState{}, fmt.Errorf("login failed: %s", callback.Err)
	}
	if callback.State != state {
		return config.AuthState{}, errors.New("login callback state did not match")
	}
	if strings.TrimSpace(callback.Code) == "" {
		return config.AuthState{}, errors.New("login callback did not include an authorization code")
	}

	token, err := m.exchangeToken(waitCtx, discovery.TokenEndpoint, authConfig.ClientID, redirectURI, verifier, callback.Code)
	if err != nil {
		return config.AuthState{}, err
	}

	return buildAuthState(backendURL, authConfig, token, m.now()), nil
}

func (m *Manager) EnsureValid(ctx context.Context, session config.AuthState) (config.AuthState, error) {
	if !session.HasSession() {
		return config.AuthState{}, errors.New("saved login state is incomplete; run inferlean login first")
	}
	if session.ExpiresAt.IsZero() || m.now().Before(session.ExpiresAt.Add(-30*time.Second)) {
		return session, nil
	}
	if strings.TrimSpace(session.RefreshToken) == "" {
		return config.AuthState{}, errors.New("saved login expired and no refresh token is available; run inferlean login again")
	}

	discovery, err := m.fetchDiscovery(ctx, session.Issuer)
	if err != nil {
		return config.AuthState{}, err
	}

	token, err := m.refreshToken(ctx, discovery.TokenEndpoint, session.ClientID, session.RefreshToken)
	if err != nil {
		return config.AuthState{}, err
	}

	updated := session
	updated.AccessToken = token.AccessToken
	if strings.TrimSpace(token.IDToken) != "" {
		updated.IDToken = token.IDToken
	}
	if strings.TrimSpace(token.RefreshToken) != "" {
		updated.RefreshToken = token.RefreshToken
	}
	if strings.TrimSpace(token.TokenType) != "" {
		updated.TokenType = token.TokenType
	}
	if token.ExpiresIn > 0 {
		updated.ExpiresAt = m.now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return updated, nil
}

func (m *Manager) ClaimInstallation(ctx context.Context, session config.AuthState, installationID string) (contracts.InstallationClaimResponse, config.AuthState, error) {
	updatedSession, err := m.EnsureValid(ctx, session)
	if err != nil {
		return contracts.InstallationClaimResponse{}, config.AuthState{}, err
	}

	request := contracts.InstallationClaimRequest{InstallationID: installationID}
	if err := request.Validate(); err != nil {
		return contracts.InstallationClaimResponse{}, config.AuthState{}, err
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return contracts.InstallationClaimResponse{}, config.AuthState{}, fmt.Errorf("encode installation claim: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, NormalizeBaseURL(updatedSession.BackendURL)+"/api/v1/installations/claim", strings.NewReader(string(payload)))
	if err != nil {
		return contracts.InstallationClaimResponse{}, config.AuthState{}, fmt.Errorf("build installation claim request: %w", err)
	}
	req.Header.Set("Authorization", defaultTokenType(updatedSession.TokenType)+" "+updatedSession.BearerToken())
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return contracts.InstallationClaimResponse{}, config.AuthState{}, fmt.Errorf("claim installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return contracts.InstallationClaimResponse{}, config.AuthState{}, fmt.Errorf("claim installation: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var response contracts.InstallationClaimResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return contracts.InstallationClaimResponse{}, config.AuthState{}, fmt.Errorf("decode installation claim response: %w", err)
	}

	return response, updatedSession, nil
}

func (m *Manager) fetchAuthConfig(ctx context.Context, backendURL string) (contracts.AuthConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, backendURL+"/api/v1/auth/config", nil)
	if err != nil {
		return contracts.AuthConfig{}, fmt.Errorf("build auth config request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return defaultAuthConfig(backendURL), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return defaultAuthConfig(backendURL), nil
	}

	var cfg contracts.AuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return defaultAuthConfig(backendURL), nil
	}
	if err := cfg.Validate(); err != nil {
		return defaultAuthConfig(backendURL), nil
	}

	return cfg, nil
}

func defaultAuthConfig(backendURL string) contracts.AuthConfig {
	return contracts.AuthConfig{
		Issuer:     NormalizeBaseURL(backendURL) + "/dex",
		ClientID:   defaultClientID,
		Scopes:     append([]string{}, defaultScopes...),
		UseIDToken: true,
	}
}

func (m *Manager) fetchDiscovery(ctx context.Context, issuer string) (discoveryDocument, error) {
	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("build oidc discovery request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("fetch oidc discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return discoveryDocument{}, fmt.Errorf("fetch oidc discovery document: unexpected status %s", resp.Status)
	}

	var discovery discoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return discoveryDocument{}, fmt.Errorf("decode oidc discovery document: %w", err)
	}
	if strings.TrimSpace(discovery.AuthorizationEndpoint) == "" || strings.TrimSpace(discovery.TokenEndpoint) == "" {
		return discoveryDocument{}, errors.New("oidc discovery document is missing endpoints")
	}

	return discovery, nil
}

func (m *Manager) exchangeToken(ctx context.Context, tokenEndpoint, clientID, redirectURI, verifier, code string) (tokenResponse, error) {
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}
	return m.submitTokenRequest(ctx, tokenEndpoint, values)
}

func (m *Manager) refreshToken(ctx context.Context, tokenEndpoint, clientID, refreshToken string) (tokenResponse, error) {
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}
	return m.submitTokenRequest(ctx, tokenEndpoint, values)
}

func (m *Manager) submitTokenRequest(ctx context.Context, tokenEndpoint string, values url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("exchange token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return tokenResponse{}, fmt.Errorf("exchange token: unexpected status %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return tokenResponse{}, fmt.Errorf("decode token response: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" && strings.TrimSpace(token.IDToken) == "" {
		return tokenResponse{}, errors.New("token response did not include a bearer token")
	}

	return token, nil
}

func buildAuthorizationURL(endpoint string, cfg contracts.AuthConfig, redirectURI, state, verifier, challenge string) string {
	values := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(cfg.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	return endpoint + "?" + values.Encode()
}

func buildAuthState(backendURL string, authConfig contracts.AuthConfig, token tokenResponse, now time.Time) config.AuthState {
	state := config.AuthState{
		BackendURL:   backendURL,
		Issuer:       authConfig.Issuer,
		ClientID:     authConfig.ClientID,
		TokenType:    defaultTokenType(token.TokenType),
		AccessToken:  token.AccessToken,
		IDToken:      token.IDToken,
		RefreshToken: token.RefreshToken,
		UseIDToken:   authConfig.UseIDToken,
	}
	if token.ExpiresIn > 0 {
		state.ExpiresAt = now.Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	return state
}

func defaultTokenType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Bearer"
	}
	return value
}

func buildCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLString(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random string: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func defaultBrowserOpener(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	return cmd.Start()
}
