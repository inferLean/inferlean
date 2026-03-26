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
)

const (
	DefaultDirName   = ".inferlean"
	DefaultFileName  = "config"
	filePermissions  = 0o600
	dirPermissions   = 0o700
	installationSize = 16
)

type Config struct {
	InstallationID string `json:"installation_id,omitempty"`
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
