// Package client 实现 Tesla Fleet API 的 REST 客户端。
//
// 关注点:
//   - token 注入(Authorization: Bearer)
//   - 5xx / 429 自动重试(指数退避 + jitter)
//   - 错误信封解析,把 HTTP 状态映射到 errs.ExitCode
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/wmango/tesla-cli/internal/errs"
	"github.com/wmango/tesla-cli/internal/tesla"
)

// Options 是构造 Client 的参数集。
type Options struct {
	Region      string        // na | eu | cn
	AccessToken string        // 必需(Bearer)
	Timeout     time.Duration // 单次请求超时,默认 30s
	Retry       int           // 重试次数(5xx/429),默认 3
	UserAgent   string        // 覆盖默认 UA(可选)
}

// Client 是线程安全的 Fleet API REST 客户端。
//
// 不持有 token 文件;由调用方在外层保证 token 已刷新(见 ensureValidToken)。
type Client struct {
	http      *http.Client
	baseURL   string
	token     string
	retry     int
	userAgent string
}

// New 构造 Client。AccessToken 为空时返回 error。
func New(opts Options) (*Client, error) {
	if opts.AccessToken == "" {
		return nil, errors.New("client: AccessToken required")
	}
	ep, err := tesla.EndpointsFor(opts.Region)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	retry := opts.Retry
	if retry <= 0 {
		retry = 3
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = tesla.UserAgent
	}
	return &Client{
		http:      tesla.NewHTTPClient(timeout),
		baseURL:   strings.TrimRight(ep.APIBase, "/"),
		token:     opts.AccessToken,
		retry:     retry,
		userAgent: ua,
	}, nil
}

// Get 发 GET <baseURL><path>。返回 raw JSON 字节;非 2xx 转成 *errs.Error。
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post 发 POST <baseURL><path>,body 是任意可 JSON 编码的对象(可为 nil)。
func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("client: marshal body: %w", err)
		}
		rdr = bytes.NewReader(raw)
	}
	return c.do(ctx, http.MethodPost, path, rdr)
}

// do 发请求并按规则重试;不可重试的错误立即返回。
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	full := c.baseURL + ensureLeadingSlash(path)

	var bodyBytes []byte
	if body != nil {
		raw, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("client: read body: %w", err)
		}
		bodyBytes = raw
	}

	var lastErr error
	for attempt := 0; attempt <= c.retry; attempt++ {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, full, reqBody)
		if err != nil {
			return nil, fmt.Errorf("client: build req: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.retry && isRetryableNetwork(err) {
				sleepBackoff(attempt)
				continue
			}
			return nil, errs.Wrap(errs.ExitTimeout, "http", err).WithRetryable(true)
		}

		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode/100 == 2 {
			return raw, nil
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			if attempt < c.retry {
				sleepBackoff(attempt)
				continue
			}
			return nil, errs.New(errs.ExitRateLimit,
				fmt.Sprintf("rate limited: %s", trimResp(raw))).WithRetryable(true)
		}
		if resp.StatusCode/100 == 5 {
			if attempt < c.retry {
				sleepBackoff(attempt)
				continue
			}
			return nil, errs.New(errs.ExitUpstream5xx,
				fmt.Sprintf("tesla %d: %s", resp.StatusCode, trimResp(raw))).WithRetryable(true)
		}
		// 4xx 不重试
		return nil, classify4xx(resp.StatusCode, raw)
	}
	if lastErr == nil {
		lastErr = errors.New("exhausted retries")
	}
	return nil, errs.Wrap(errs.ExitGeneric, "http", lastErr)
}

// classify4xx 把 4xx 状态映射成有意义的退出码。
func classify4xx(status int, raw []byte) error {
	msg := fmt.Sprintf("tesla %d: %s", status, trimResp(raw))
	switch status {
	case http.StatusUnauthorized:
		return errs.New(errs.ExitAuth, msg).
			WithHint("token may be invalid; try `tesla auth refresh` or `tesla auth login`")
	case http.StatusForbidden:
		return errs.New(errs.ExitScope, msg).
			WithHint("missing OAuth scope or virtual key not paired")
	case http.StatusNotFound:
		return errs.New(errs.ExitUsage, msg)
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return errs.New(errs.ExitUsage, msg)
	case http.StatusRequestTimeout:
		return errs.New(errs.ExitTimeout, msg).WithRetryable(true)
	default:
		return errs.New(errs.ExitGeneric, msg)
	}
}

func trimResp(raw []byte) string {
	const max = 512
	s := strings.TrimSpace(string(raw))
	if len(s) > max {
		s = s[:max] + "...(truncated)"
	}
	return s
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

func isRetryableNetwork(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

func sleepBackoff(attempt int) {
	base := 200 * time.Millisecond
	mult := time.Duration(1<<attempt) * base
	jitter := time.Duration(rand.Int63n(int64(base)))
	time.Sleep(mult + jitter)
}
