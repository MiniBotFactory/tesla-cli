package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNotFound 表示对应 profile 没有保存过 token。
var ErrNotFound = errors.New("token not found for profile")

// Store 抽象了 token 的持久化。M2 阶段提供文件实现;
// M3+ 可加 keyring 实现并由 NewStore 按平台选择。
type Store interface {
	Save(profile string, t *Token) error
	Load(profile string) (*Token, error)
	Delete(profile string) error
	Path(profile string) string
}

// FileStore 把 token 写到 baseDir/profiles/<profile>.json (mode 0600)。
type FileStore struct {
	BaseDir string
}

// NewFileStore 构造文件存储。baseDir 一般是 ~/.config/tesla。
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{BaseDir: baseDir}
}

// Path 返回 profile 文件路径。
func (s *FileStore) Path(profile string) string {
	return filepath.Join(s.BaseDir, "profiles", profile+".json")
}

// Save 原子写入 token。父目录权限 0700,文件权限 0600。
func (s *FileStore) Save(profile string, t *Token) error {
	if t == nil {
		return errors.New("store: nil token")
	}
	full := s.Path(profile)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+profile+".*.tmp")
	if err != nil {
		return fmt.Errorf("store: create tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanupTmp := func() { _ = os.Remove(tmpPath) }

	if _, werr := tmp.Write(data); werr != nil {
		_ = tmp.Close()
		cleanupTmp()
		return fmt.Errorf("store: write tmp: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		cleanupTmp()
		return fmt.Errorf("store: close tmp: %w", cerr)
	}
	if cerr := os.Chmod(tmpPath, 0o600); cerr != nil {
		cleanupTmp()
		return fmt.Errorf("store: chmod tmp: %w", cerr)
	}
	if rerr := os.Rename(tmpPath, full); rerr != nil {
		cleanupTmp()
		return fmt.Errorf("store: rename: %w", rerr)
	}
	return nil
}

// Load 读取并反序列化。文件不存在返回 ErrNotFound。
func (s *FileStore) Load(profile string) (*Token, error) {
	full := s.Path(profile)
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("store: read %s: %w", full, err)
	}
	var t Token
	if uerr := json.Unmarshal(data, &t); uerr != nil {
		return nil, fmt.Errorf("store: unmarshal %s: %w", full, uerr)
	}
	return &t, nil
}

// Delete 删除 profile 文件。文件不存在视为成功(幂等)。
func (s *FileStore) Delete(profile string) error {
	full := s.Path(profile)
	if err := os.Remove(full); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("store: remove %s: %w", full, err)
	}
	return nil
}
