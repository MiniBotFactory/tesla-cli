// Package auth 实现 OAuth 2.0(authorization_code + PKCE)与 Token 持久化。
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCE 是一对 code_verifier / code_challenge。
// 调用方在 authorize 请求里发 challenge,在 token 交换里发 verifier。
type PKCE struct {
	Verifier  string
	Challenge string
	Method    string // 固定为 "S256"
}

// NewPKCE 生成 PKCE 对(43 字符 base64url verifier + S256 challenge)。
func NewPKCE() (PKCE, error) {
	const verifierBytes = 32 // 32 字节 → 43 字符 base64url(无 padding)
	buf := make([]byte, verifierBytes)
	if _, err := rand.Read(buf); err != nil {
		return PKCE{}, fmt.Errorf("pkce: read random: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge, Method: "S256"}, nil
}
