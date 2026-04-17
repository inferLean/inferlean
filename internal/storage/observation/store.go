package observation

import (
	"fmt"
	"os"
	"path/filepath"
)

const filePerm = 0o600

type Store struct{}

func NewStore() Store {
	return Store{}
}

func (Store) SaveRaw(dir, name string, data []byte) (string, error) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return "", fmt.Errorf("write observation %s: %w", name, err)
	}
	return path, nil
}
