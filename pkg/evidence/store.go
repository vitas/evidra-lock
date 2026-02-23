package evidence

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"samebits.com/evidra-mcp/pkg/config"
)

func NewStore() *Store {
	path, err := config.ResolveEvidencePath("")
	if err != nil {
		path = filepath.FromSlash(config.DefaultEvidenceRelativeDir)
	}
	return NewStoreWithPath(path)
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStoreWithPath(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Init() error {
	return withStoreLock(s.path, func() error {
		mode, resolved, err := detectStoreMode(s.path)
		if err != nil {
			return err
		}
		if mode == "segmented" {
			_, err := loadOrInitManifest(resolved, segmentMaxBytesFromEnv(), true)
			return err
		}
		return os.MkdirAll(filepath.Dir(resolved), 0o755)
	})
}

func (s *Store) Append(record Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := appendAtPath(s.path, record)
	return err
}

func (s *Store) ValidateChain() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return validateChainAtPath(s.path)
}

func (s *Store) LastHash() (string, error) {
	var out string
	err := withStoreLock(s.path, func() error {
		mode, resolved, err := detectStoreMode(s.path)
		if err != nil {
			return err
		}
		if mode == "segmented" {
			m, err := loadOrInitManifest(resolved, segmentMaxBytesFromEnv(), false)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					out = ""
					return nil
				}
				return err
			}
			out = m.LastHash
			return nil
		}

		last, ok, err := readLastRecord(resolved)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				out = ""
				return nil
			}
			return err
		}
		if !ok {
			out = ""
			return nil
		}
		out = last.Hash
		return nil
	})
	if err != nil {
		return "", err
	}
	return out, nil
}
