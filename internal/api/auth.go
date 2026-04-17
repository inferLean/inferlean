package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

const (
	callbackAddress = "127.0.0.1:38080"
	callbackPath    = "/oauth/callback"
	loginTimeout    = 2 * time.Minute
)

type AuthManager struct {
	http *http.Client
}

type authConfig struct {
	Issuer     string   `json:"issuer"`
	ClientID   string   `json:"client_id"`
	Scopes     []string `json:"scopes"`
	UseIDToken bool     `json:"use_id_token"`
}

type discoveryDoc struct {
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

func NewAuthManager() AuthManager {
	return AuthManager{http: &http.Client{Timeout: 20 * time.Second}}
}

func (m AuthManager) Login(ctx context.Context, backendURL string) (types.AuthState, error) {
	cfg, err := m.fetchAuthConfig(ctx, backendURL)
	if err != nil {
		return types.AuthState{}, err
	}
	doc, err := m.fetchDiscovery(ctx, cfg.Issuer)
	if err != nil {
		return types.AuthState{}, err
	}
	state, err := randomURLString(24)
	if err != nil {
		return types.AuthState{}, err
	}
	verifier, err := randomURLString(48)
	if err != nil {
		return types.AuthState{}, err
	}
	redirectURI := "http://" + callbackAddress + callbackPath
	resultCh, errCh, closeServer, err := startAuthorizationServer(state)
	if err != nil {
		return types.AuthState{}, err
	}
	defer closeServer()
	loginURL := authURL(doc.AuthorizationEndpoint, cfg, redirectURI, state, verifier, codeChallenge(verifier))
	if err := openBrowser(loginURL); err != nil {
		fmt.Printf("Open this URL manually to login:\n%s\n", loginURL)
	}
	code, err := waitForAuthorizationCode(ctx, resultCh, errCh)
	if err != nil {
		return types.AuthState{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	resp, err := m.exchangeToken(waitCtx, doc.TokenEndpoint, cfg.ClientID, redirectURI, verifier, code)
	if err != nil {
		return types.AuthState{}, err
	}
	return types.AuthState{
		BackendURL:   normalizeBaseURL(backendURL),
		Issuer:       cfg.Issuer,
		ClientID:     cfg.ClientID,
		TokenType:    firstNonEmpty(resp.TokenType, "Bearer"),
		AccessToken:  strings.TrimSpace(resp.AccessToken),
		IDToken:      strings.TrimSpace(resp.IDToken),
		RefreshToken: strings.TrimSpace(resp.RefreshToken),
		ExpiresAt:    time.Now().UTC().Add(time.Duration(resp.ExpiresIn) * time.Second),
		UseIDToken:   cfg.UseIDToken,
	}, nil
}

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

func shutdownServer(server *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func (m AuthManager) fetchAuthConfig(ctx context.Context, backendURL string) (authConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizeBaseURL(backendURL)+"/api/v1/auth/config", nil)
	if err != nil {
		return authConfig{}, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return authConfig{}, fmt.Errorf("fetch auth config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return authConfig{}, fmt.Errorf("fetch auth config: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var cfg authConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return authConfig{}, fmt.Errorf("decode auth config: %w", err)
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email", "offline_access"}
	}
	return cfg, nil
}

func (m AuthManager) fetchDiscovery(ctx context.Context, issuer string) (discoveryDoc, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(issuer, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return discoveryDoc{}, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return discoveryDoc{}, fmt.Errorf("fetch oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return discoveryDoc{}, fmt.Errorf("fetch oidc discovery: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var doc discoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return discoveryDoc{}, fmt.Errorf("decode oidc discovery: %w", err)
	}
	return doc, nil
}

func (m AuthManager) exchangeToken(ctx context.Context, tokenEndpoint, clientID, redirectURI, verifier, code string) (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := m.http.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("exchange token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return tokenResponse{}, fmt.Errorf("exchange token: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return tokenResponse{}, fmt.Errorf("decode token response: %w", err)
	}
	return token, nil
}

func authURL(endpoint string, cfg authConfig, redirectURI, state, verifier, challenge string) string {
	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(cfg.Scopes, " "))
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("code_verifier", verifier)
	return endpoint + "?" + params.Encode()
}

func randomURLString(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
