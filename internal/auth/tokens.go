package auth

import "time"

// Token 是 Tesla OAuth 第三方令牌。
//
// 持久化:JSON 编码,写入 ~/.config/tesla/profiles/<profile>.json (mode 0600)。
// expires_at / obtained_at 用 RFC3339;Tesla token 实际寿命 8 小时。
type Token struct {
	AccessToken  string    `json:"access_token"            yaml:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty" yaml:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"              yaml:"token_type"`
	Scopes       []string  `json:"scopes,omitempty"        yaml:"scopes,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"              yaml:"expires_at"`
	ObtainedAt   time.Time `json:"obtained_at"             yaml:"obtained_at"`
}

// Expired 判断 token 是否已过期(留 30 秒安全边际)。
func (t *Token) Expired() bool {
	if t == nil {
		return true
	}
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

// SafeView 返回脱敏后的副本(用于 `tesla auth status`,避免输出真 token)。
// 不修改原对象(保持不可变)。
func (t *Token) SafeView() map[string]any {
	if t == nil {
		return nil
	}
	return map[string]any{
		"token_type":           t.TokenType,
		"scopes":               t.Scopes,
		"expires_at":           t.ExpiresAt.Format(time.RFC3339),
		"obtained_at":          t.ObtainedAt.Format(time.RFC3339),
		"access_token_masked":  mask(t.AccessToken),
		"refresh_token_masked": mask(t.RefreshToken),
		"expired":              t.Expired(),
	}
}

// mask 返回类似 "abc***xyz" 的形式;长度 <8 时全部打码。
func mask(s string) string {
	if len(s) < 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}
