// Package keys 处理虚拟钥匙的密钥对生成、部署指引和配对深链。
//
// Tesla 要求用 EC secp256r1 (P-256) 公私钥对,公钥托管于
// https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem。
package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// 文件常量。
const (
	PrivateKeyFile = "private-key.pem"
	PublicKeyFile  = "public-key.pem"

	// WellKnownPath 是 Tesla 服务端拉取公钥的相对 URL 路径。
	WellKnownPath = ".well-known/appspecific/com.tesla.3p.public-key.pem"
)

// GenerateOptions 控制密钥生成行为。
type GenerateOptions struct {
	OutDir string // 输出目录;为空时报错
	Force  bool   // 覆盖已存在文件
}

// GenerateResult 是写盘后的结果,便于 CLI 输出绝对路径。
type GenerateResult struct {
	PrivateKeyPath string `json:"private_key_path" yaml:"private_key_path"`
	PublicKeyPath  string `json:"public_key_path"  yaml:"public_key_path"`
	Curve          string `json:"curve"            yaml:"curve"`
}

// Generate 生成 EC P-256 密钥对并写入两个 PEM 文件。
func Generate(opts GenerateOptions) (*GenerateResult, error) {
	if opts.OutDir == "" {
		return nil, errors.New("keys: OutDir required")
	}
	if err := os.MkdirAll(opts.OutDir, 0o700); err != nil {
		return nil, fmt.Errorf("keys: mkdir %s: %w", opts.OutDir, err)
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("keys: generate: %w", err)
	}
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("keys: marshal private: %w", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("keys: marshal public: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	privPath := filepath.Join(opts.OutDir, PrivateKeyFile)
	pubPath := filepath.Join(opts.OutDir, PublicKeyFile)

	if err := writeIfAbsent(privPath, privPEM, 0o600, opts.Force); err != nil {
		return nil, err
	}
	if err := writeIfAbsent(pubPath, pubPEM, 0o644, opts.Force); err != nil {
		return nil, err
	}
	abs := func(p string) string {
		ap, err := filepath.Abs(p)
		if err != nil {
			return p
		}
		return ap
	}
	return &GenerateResult{
		PrivateKeyPath: abs(privPath),
		PublicKeyPath:  abs(pubPath),
		Curve:          "secp256r1 (P-256)",
	}, nil
}

// writeIfAbsent 拒绝覆盖已存在文件,除非 force=true。
func writeIfAbsent(path string, data []byte, mode os.FileMode, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("keys: %s already exists (use --force to overwrite)", path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("keys: stat %s: %w", path, err)
		}
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("keys: write %s: %w", path, err)
	}
	return nil
}

// PublicKeyPath 返回 baseDir 下 public-key.pem 的绝对路径(不读盘,仅拼路径)。
func PublicKeyPath(baseDir string) string {
	p, _ := filepath.Abs(filepath.Join(baseDir, PublicKeyFile))
	return p
}
