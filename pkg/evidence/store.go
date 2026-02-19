package evidence

import (
	"os"
	"path/filepath"
)

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Init() error {
	return os.MkdirAll(filepath.Dir(logPath), 0o755)
}

func (s *Store) Append(record EvidenceRecord) (EvidenceRecord, error) {
	return Append(record)
}

func (s *Store) ValidateChain() error {
	return ValidateChain()
}
