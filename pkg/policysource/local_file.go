package policysource

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

type LocalFileSource struct {
	PolicyPath string
	DataPath   string
}

func NewLocalFileSource(policyPath string, dataPath string) *LocalFileSource {
	return &LocalFileSource{
		PolicyPath: policyPath,
		DataPath:   dataPath,
	}
}

func (s *LocalFileSource) LoadPolicy() ([]byte, error) {
	return os.ReadFile(s.PolicyPath)
}

func (s *LocalFileSource) LoadData() ([]byte, error) {
	if s.DataPath == "" {
		return nil, nil
	}
	return os.ReadFile(s.DataPath)
}

func (s *LocalFileSource) PolicyRef() (string, error) {
	b, err := s.LoadPolicy()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
