package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inferLean/inferlean-main/cli/internal/identity"
	"github.com/inferLean/inferlean-main/cli/internal/types"
)

const (
	dirPerm  = 0o700
	filePerm = 0o600
)

type Config struct {
	InstallationID string          `json:"installation_id,omitempty"`
	Auth           types.AuthState `json:"auth,omitempty"`
}

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return &Store{path: filepath.Join(home, ".inferlean", "config")}, nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), dirPerm); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.path, data, filePerm); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (s *Store) Ensure() (Config, error) {
	cfg, err := s.Load()
	if err != nil {
		return Config{}, err
	}
	if cfg.InstallationID != "" {
		return cfg, nil
	}
	id, err := identity.NewInstallationID()
	if err != nil {
		return Config{}, err
	}
	cfg.InstallationID = id
	if err := s.Save(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
