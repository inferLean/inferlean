package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	InstallationIDBytes = 16
	RunIDSuffixBytes    = 4
)

func NewInstallationID() (string, error) {
	buf := make([]byte, InstallationIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate installation id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func NewRunID() (string, error) {
	suffix := make([]byte, RunIDSuffixBytes)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("generate run id suffix: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102-150405.000Z")
	return fmt.Sprintf("%s-%s", stamp, hex.EncodeToString(suffix)), nil
}
