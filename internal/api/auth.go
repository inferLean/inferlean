package api

import (
	"net/http"
	"time"
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
