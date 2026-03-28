package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultDirName   = ".inferlean"
	DefaultFileName  = "config"
	filePermissions  = 0o600
	dirPermissions   = 0o700
	installationSize = 16
)

type Config struct {
	InstallationID string     `json:"installation_id,omitempty"`
	Auth           *AuthState `json:"auth,omitempty"`
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
	return strings.TrimSpace(a.BackendURL) != "" &&
		strings.TrimSpace(a.Issuer) != "" &&
		strings.TrimSpace(a.ClientID) != "" &&
		strings.TrimSpace(a.BearerToken()) != ""
}

func (a AuthState) BearerToken() string {
	if a.UseIDToken && strings.TrimSpace(a.IDToken) != "" {
		return a.IDToken
	}
	if strings.TrimSpace(a.AccessToken) != "" {
		return a.AccessToken
	}
	return a.IDToken
}

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	return &Store{path: DefaultPath(home)}, nil
}

func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

func DefaultPath(home string) string {
	return filepath.Join(home, DefaultDirName, DefaultFileName)
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (Config, error) {
	var cfg Config

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	if strings.TrimSpace(cfg.InstallationID) == "" {
		return errors.New("installation_id is required")
	}

	if err := os.MkdirAll(filepath.Dir(s.path), dirPermissions); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(s.path, data, filePermissions); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func (s *Store) Ensure() (Config, error) {
	cfg, err := s.Load()
	if err != nil {
		return Config{}, err
	}

	if strings.TrimSpace(cfg.InstallationID) == "" {
		cfg.InstallationID, err = newInstallationID()
		if err != nil {
			return Config{}, err
		}
		if err := s.Save(cfg); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func newInstallationID() (string, error) {
	var buf [installationSize]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate installation id: %w", err)
	}

	return hex.EncodeToString(buf[:]), nil
}
