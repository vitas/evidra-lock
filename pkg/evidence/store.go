package evidence

import (
	"errors"
	"os"
	"path/filepath"
)

func NewStore() *Store {
	return NewStoreWithPath(defaultLogPath)
}

type Store struct {
	path string
}

func NewStoreWithPath(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Init() error {
	return os.MkdirAll(filepath.Dir(s.path), 0o755)
}

func (s *Store) Append(record Record) error {
	_, err := appendAtPath(s.path, record)
	return err
}

func (s *Store) ValidateChain() error {
	return validateChainAtPath(s.path)
}

func (s *Store) LastHash() (string, error) {
	last, ok, err := readLastRecord(s.path)
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
