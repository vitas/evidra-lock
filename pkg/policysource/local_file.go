package policysource

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func (s *LocalFileSource) LoadPolicy() (map[string][]byte, error) {
	info, err := os.Stat(s.PolicyPath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return s.loadPolicyDir(s.PolicyPath)
	}
	dirRoot := filepath.Join(filepath.Dir(s.PolicyPath), strings.TrimSuffix(filepath.Base(s.PolicyPath), filepath.Ext(s.PolicyPath)))
	if dirInfo, err := os.Stat(dirRoot); err == nil && dirInfo.IsDir() {
		modules, err := s.loadPolicyDir(dirRoot)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(s.PolicyPath)
		if err != nil {
			return nil, err
		}
		modules[filepath.Base(s.PolicyPath)] = b
		return modules, nil
	}
	b, err := os.ReadFile(s.PolicyPath)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		filepath.Base(s.PolicyPath): b,
	}, nil
}

func (s *LocalFileSource) loadPolicyDir(root string) (map[string][]byte, error) {
	modules := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".rego" {
			return nil
		}
		rel, err := filepath.Rel(s.PolicyPath, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		modules[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(modules) == 0 {
		return nil, fs.ErrNotExist
	}
	return modules, nil
}

func (s *LocalFileSource) LoadData() ([]byte, error) {
	if s.DataPath == "" {
		return nil, nil
	}
	return os.ReadFile(s.DataPath)
}

func (s *LocalFileSource) PolicyRef() (string, error) {
	modules, err := s.LoadPolicy()
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(modules))
	for k := range modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	hasher := sha256.New()
	for _, k := range keys {
		hasher.Write([]byte(k))
		hasher.Write([]byte{0})
		hasher.Write(modules[k])
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
