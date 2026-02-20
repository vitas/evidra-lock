package policysource

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

type LocalFilePolicySource struct {
	path string
}

func NewLocalFilePolicySource(path string) *LocalFilePolicySource {
	return &LocalFilePolicySource{path: path}
}

func (s *LocalFilePolicySource) LoadPolicy() ([]byte, error) {
	return os.ReadFile(s.path)
}

func (s *LocalFilePolicySource) PolicyRef() string {
	b, err := s.LoadPolicy()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
