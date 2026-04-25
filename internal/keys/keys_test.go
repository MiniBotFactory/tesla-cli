package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerate_writesBothFilesAndCorrectPEM(t *testing.T) {
	dir := t.TempDir()
	res, err := Generate(GenerateOptions{OutDir: dir})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.Curve != "secp256r1 (P-256)" {
		t.Errorf("Curve label mismatch: %q", res.Curve)
	}
	if res.PrivateKeyPath == "" || res.PublicKeyPath == "" {
		t.Errorf("paths should be non-empty")
	}

	priv, err := os.ReadFile(filepath.Join(dir, PrivateKeyFile))
	if err != nil {
		t.Fatalf("read private: %v", err)
	}
	if !strings.Contains(string(priv), "BEGIN EC PRIVATE KEY") {
		t.Errorf("private PEM header missing")
	}
	pub, err := os.ReadFile(filepath.Join(dir, PublicKeyFile))
	if err != nil {
		t.Fatalf("read public: %v", err)
	}
	if !strings.Contains(string(pub), "BEGIN PUBLIC KEY") {
		t.Errorf("public PEM header missing")
	}

	// 验证 PEM 解码后是合法 EC P-256 公私钥对。
	block, _ := pem.Decode(priv)
	if block == nil {
		t.Fatalf("private PEM decode failed")
	}
	parsed, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse EC private: %v", err)
	}
	if parsed.Curve != elliptic.P256() {
		t.Errorf("expected P-256 curve")
	}
	pubBlock, _ := pem.Decode(pub)
	pubAny, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		t.Fatalf("parse PKIX pub: %v", err)
	}
	if _, ok := pubAny.(*ecdsa.PublicKey); !ok {
		t.Errorf("public key not ECDSA, got %T", pubAny)
	}
}

func TestGenerate_filePermissionsAreLocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions not enforced on Windows")
	}
	dir := t.TempDir()
	if _, err := Generate(GenerateOptions{OutDir: dir}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	stPriv, _ := os.Stat(filepath.Join(dir, PrivateKeyFile))
	if mode := stPriv.Mode().Perm(); mode != 0o600 {
		t.Errorf("private key should be 0600, got %o", mode)
	}
	stPub, _ := os.Stat(filepath.Join(dir, PublicKeyFile))
	if mode := stPub.Mode().Perm(); mode != 0o644 {
		t.Errorf("public key should be 0644, got %o", mode)
	}
}

func TestGenerate_refusesOverwriteByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(GenerateOptions{OutDir: dir}); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	_, err := Generate(GenerateOptions{OutDir: dir})
	if err == nil {
		t.Fatalf("second generate without --force should fail")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got %v", err)
	}
}

func TestGenerate_forceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(GenerateOptions{OutDir: dir}); err != nil {
		t.Fatalf("first: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, PrivateKeyFile))
	if _, err := Generate(GenerateOptions{OutDir: dir, Force: true}); err != nil {
		t.Fatalf("force: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, PrivateKeyFile))
	if string(first) == string(second) {
		t.Errorf("force should produce a new key, but content unchanged")
	}
}

func TestGenerate_emptyOutDirRejected(t *testing.T) {
	_, err := Generate(GenerateOptions{OutDir: ""})
	if err == nil {
		t.Fatalf("empty OutDir should error")
	}
}

func TestPublicKeyPath_returnsAbsoluteToFile(t *testing.T) {
	got := PublicKeyPath("/tmp/x")
	if !strings.HasSuffix(got, PublicKeyFile) {
		t.Errorf("should end in %s, got %q", PublicKeyFile, got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("should be absolute: %q", got)
	}
}

func TestPairURL_format(t *testing.T) {
	got, err := PairURL("foo.example.com")
	if err != nil {
		t.Fatalf("PairURL: %v", err)
	}
	want := "https://tesla.com/_ak/foo.example.com"
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestPairURL_emptyDomainRejected(t *testing.T) {
	_, err := PairURL("")
	if err == nil {
		t.Fatalf("empty domain should error")
	}
}

func TestPublishInstructions_includesDomainAndPath(t *testing.T) {
	got := PublishInstructions("foo.example.com", "/tmp/pub.pem")
	if !strings.Contains(got, "foo.example.com") {
		t.Errorf("instructions should mention domain")
	}
	if !strings.Contains(got, WellKnownPath) {
		t.Errorf("instructions should reference well-known path")
	}
	if !strings.Contains(got, "/tmp/pub.pem") {
		t.Errorf("instructions should reference local pub path")
	}
}

func TestPublishInstructions_emptyArgsReturnEmpty(t *testing.T) {
	if got := PublishInstructions("", ""); got != "" {
		t.Errorf("empty args should yield empty string, got %q", got)
	}
}

func TestWellKnownPath_constant(t *testing.T) {
	if WellKnownPath != ".well-known/appspecific/com.tesla.3p.public-key.pem" {
		t.Errorf("WellKnownPath constant drift: %q", WellKnownPath)
	}
}
