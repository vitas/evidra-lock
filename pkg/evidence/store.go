package evidence

import (
	"errors"
	"os"
	"path/filepath"
)

func NewStore() *Store {
	return NewStoreWithPath(defaultEvidenceRoot)
}

type Store struct {
	path string
}

func NewStoreWithPath(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Init() error {
	mode, resolved, err := detectStoreMode(s.path)
	if err != nil {
		return err
	}
	if mode == "segmented" {
		_, err := loadOrInitManifest(resolved, segmentMaxBytesFromEnv(), true)
		return err
	}
	return os.MkdirAll(filepath.Dir(resolved), 0o755)
}

func (s *Store) Append(record Record) error {
	_, err := appendAtPath(s.path, record)
	return err
}

func (s *Store) ValidateChain() error {
	return validateChainAtPath(s.path)
}

func (s *Store) LastHash() (string, error) {
	mode, resolved, err := detectStoreMode(s.path)
	if err != nil {
		return "", err
	}
	if mode == "segmented" {
		m, err := loadOrInitManifest(resolved, segmentMaxBytesFromEnv(), false)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", nil
			}
			return "", err
		}
		return m.LastHash, nil
	}

	last, ok, err := readLastRecord(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if !ok {
		return "", nil
	}
	return last.Hash, nil
}
