package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/browser"
	"github.com/inferLean/inferlean-main/cli/internal/types"
)

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

func randomURLString(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func openBrowser(url string) error {
	return browser.Open(url)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
