package cli

import (
	"context"
	"errors"

	"github.com/wmango/tesla-cli/internal/auth"
	"github.com/wmango/tesla-cli/internal/config"
	"github.com/wmango/tesla-cli/internal/errs"
)

// ensureValidToken 加载当前 profile 的 token,如果过期且有 refresh_token,
// 透明地刷新并保存。返回可用的 *Token 或 *errs.Error。
//
// 行为约定:
//   - 没 token         → ExitAuth + hint "tesla auth login"
//   - 过期且无 refresh → ExitAuth + hint "tesla auth login"
//   - 刷新失败         → ExitAuth(原始错误包装,标记 Retryable=true)
//   - 一切正常         → 返回 *Token;若刷新过则已 Save
func ensureValidToken(ctx context.Context, cfg config.Config) (*auth.Token, error) {
	store := auth.NewFileStore(cfg.BaseDir)
	tok, err := store.Load(cfg.Profile)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil, errs.New(errs.ExitAuth, "no token for profile "+cfg.Profile).
				WithHint("run `tesla auth login` first")
		}
		return nil, errs.Wrap(errs.ExitGeneric, "load token", err)
	}
	if !tok.Expired() {
		return tok, nil
	}
	if tok.RefreshToken == "" {
		return nil, errs.New(errs.ExitAuth, "token expired and no refresh_token").
			WithHint("run `tesla auth login` again with offline_access scope")
	}

	values, cerr := loadConfigTOML(cfg.ConfigFilePath())
	if cerr != nil {
		return nil, errs.New(errs.ExitConfig, "no config file").
			WithHint("run `tesla config init` first")
	}
	region := firstNonEmpty(cfg.Region, values["region"], "na")
	newTok, rerr := auth.Refresh(ctx, auth.RefreshOptions{
		Region:       region,
		ClientID:     values["client_id"],
		ClientSecret: values["client_secret"],
		RefreshToken: tok.RefreshToken,
	})
	if rerr != nil {
		return nil, errs.Wrap(errs.ExitAuth, "auto-refresh", rerr).
			WithRetryable(true).
			WithHint("run `tesla auth refresh` manually to inspect")
	}
	if serr := store.Save(cfg.Profile, newTok); serr != nil {
		return nil, errs.Wrap(errs.ExitConfig, "save refreshed token", serr)
	}
	return newTok, nil
}
