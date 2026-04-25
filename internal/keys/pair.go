package keys

import (
	"errors"
	"fmt"

	"github.com/wmango/tesla-cli/internal/tesla"
)

// PairURL 构造 Tesla App 配对深链。
// 用户在手机上点开此 URL,Tesla App 会拉起把虚拟钥匙加入车辆的流程。
//
// 注意:domain 必须先用 partner register 注册过且公钥可达。
func PairURL(domain string) (string, error) {
	if domain == "" {
		return "", errors.New("keys: domain required")
	}
	return tesla.PairDeepLink(domain), nil
}

// PublishInstructions 返回部署公钥到 .well-known 的人类可读步骤。
//
// pubPath 为本地公钥 PEM 文件路径;domain 为目标域名。
func PublishInstructions(domain, pubPath string) string {
	if domain == "" || pubPath == "" {
		return ""
	}
	return fmt.Sprintf(`将本地公钥部署到下列 URL,Tesla 服务端会拉取以校验你的身份:

  https://%s/%s

部署步骤(任选一种):

  方案 A:静态托管(Nginx / S3 / GitHub Pages)
    1. 把 %s 复制到站点根目录的 .well-known/appspecific/ 下
    2. 文件名必须是:com.tesla.3p.public-key.pem
    3. 确认浏览器访问上述 URL 能 200 返回 PEM

  方案 B:Vercel / Netlify
    1. 在项目 public/ (或同等位置) 建相同路径
    2. 重新部署
    3. 验证可访问

部署成功后再运行:
  tesla auth partner register --domain %s
  tesla key pair-url --domain %s    # 把链接交给车主`,
		domain, WellKnownPath, pubPath, domain, domain)
}
