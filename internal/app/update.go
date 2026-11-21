package app

// Release 版本检查：GitHub latest release 对比本地版本。
//
// 国内网络下 api.github.com 可能被 DNS 污染，按以下顺序尝试：
//  1. 域名固定 IP 直连（SNI 正确、证书正常校验，参考 config 包的实现）
//  2. 本地代理 127.0.0.1:7890（clash/mihomo 混合端口）
//  3. 系统直连（默认 DNS + 默认路由）
//
// 每种通道先试 api.github.com JSON，失败再退回 github.com releases/latest 的 302 重定向解析。

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"qqsecond/internal/version"
)

const (
	releasesAPIURL  = "https://api.github.com/repos/Admirepowered/qq-second/releases/latest"
	releasesPageURL = "https://github.com/Admirepowered/qq-second/releases/latest"
)

// fallbackProxyPorts 常见本地代理端口（env 与 git 配置都找不到时依次尝试）。
var fallbackProxyPorts = []string{"7890", "7897", "48952", "10809", "1080"}

// 域名固定 IP 表（DNS 被污染时的兜底）。
var pinnedIPs = map[string]string{
	"github.com":                    "140.82.116.3",
	"api.github.com":                "140.82.112.6",
	"objects.githubusercontent.com": "185.199.108.133",
}

// UpdateInfo 更新检查结果。
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseURL     string `json:"releaseUrl"`
	ReleaseNotes   string `json:"releaseNotes"`
	CheckedAt      int64  `json:"checkedAt"`
}

// lastGoodChannel 记住上次成功的通道，后续检查优先尝试。
var lastGoodChannel atomic.Value // string

// CheckUpdate 按多级通道查询最新版本。
func (a *App) CheckUpdate() (*UpdateInfo, error) {
	var errs []string
	channels := []string{"proxy", "pinned", "plain"}
	if good, ok := lastGoodChannel.Load().(string); ok && good != "" {
		channels = append([]string{good}, channels...)
	}
	for _, ch := range channels {
		info, err := checkUpdateVia(ch)
		if err == nil && info != nil {
			lastGoodChannel.Store(ch)
			return info, nil
		}
		if err != nil {
			errs = append(errs, ch+": "+err.Error())
		}
	}
	if len(errs) == 0 {
		errs = append(errs, "no channel available")
	}
	return nil, fmt.Errorf("%s", strings.Join(errs, " | "))
}

func checkUpdateVia(channel string) (*UpdateInfo, error) {
	client, err := githubClient(channel)
	if err != nil {
		return nil, err
	}

	// 1) API JSON
	if body, _, err := githubGet(client, releasesAPIURL); err == nil {
		var release struct {
			TagName string `json:"tag_name"`
			HTMLURL string `json:"html_url"`
			Body    string `json:"body"`
		}
		if json.Unmarshal([]byte(body), &release) == nil && release.TagName != "" {
			latest := strings.TrimPrefix(release.TagName, "v")
			return &UpdateInfo{
				CurrentVersion: version.Version,
				LatestVersion:  latest,
				ReleaseURL:     release.HTMLURL,
				ReleaseNotes:   truncateNotes(release.Body, 500),
				CheckedAt:      time.Now().Unix(),
				HasUpdate:      latest != "" && latest != version.Version,
			}, nil
		}
	}

	// 2) releases/latest 302 重定向解析
	_, loc, err := githubGet(client, releasesPageURL)
	if err != nil {
		return nil, err
	}
	tag := loc[strings.LastIndex(loc, "/")+1:]
	tag = strings.TrimPrefix(tag, "v")
	// 尚未发布任何 Release 时 GitHub 会重定向到 releases 列表页
	if tag == "" || tag == "releases" || tag == "latest" || tag == "tags" {
		return &UpdateInfo{
			CurrentVersion: version.Version,
			LatestVersion:  "",
			ReleaseURL:     releasesPageURL,
			CheckedAt:      time.Now().Unix(),
			HasUpdate:      false,
		}, nil
	}
	return &UpdateInfo{
		CurrentVersion: version.Version,
		LatestVersion:  tag,
		ReleaseURL:     releasesPageURL,
		CheckedAt:      time.Now().Unix(),
		HasUpdate:      tag != version.Version,
	}, nil
}

// githubClient 按通道构造 HTTP client。
func githubClient(channel string) (*http.Client, error) {
	switch channel {
	case "pinned":
		tr := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// addr 的 host 部分替换为固定 IP（端口保留）；
				// TLS 握手仍以 URL 域名做 SNI 与证书校验，安全性不降级。
				host, port, _ := net.SplitHostPort(addr)
				if ip := pinnedIPs[host]; ip != "" {
					return net.Dial(network, net.JoinHostPort(ip, port))
				}
				return net.Dial(network, addr)
			},
			TLSClientConfig: &tls.Config{},
		}
		return &http.Client{Timeout: 10 * time.Second, Transport: tr}, nil
	case "proxy":
		proxyStr := detectLocalProxy()
		if proxyStr == "" {
			return nil, fmt.Errorf("no local proxy found")
		}
		proxyURL, err := url.Parse(proxyStr)
		if err != nil {
			return nil, err
		}
		tr := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			// 本地代理（如 clash/GitHub 加速工具）常做 TLS MITM，跳过校验；
			// pinned/plain 通道仍正常校验。
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		return &http.Client{Timeout: 10 * time.Second, Transport: tr}, nil
	default:
		return &http.Client{Timeout: 10 * time.Second}, nil
	}
}

// githubGet 发起 GET；返回 (body, location, err)。location 非空表示 302。
func githubGet(client *http.Client, target string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "QQSecond-Updater")

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if loc := resp.Header.Get("Location"); loc != "" {
		return "", loc, nil
	}
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("http %d from %s", resp.StatusCode, hostOf(target))
	}
	body, err := readAllLimit(resp.Body, 1<<20)
	if err != nil {
		return "", "", err
	}
	return string(body), "", nil
}

func hostOf(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	return u.Host
}

func truncateNotes(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// detectLocalProxy 依次探测：环境变量 → git 全局配置 → 常见端口（TCP 可连即用）。
func detectLocalProxy() string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	if out, err := exec.Command("git", "config", "--global", "--get", "http.proxy").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	for _, port := range fallbackProxyPorts {
		addr := net.JoinHostPort("127.0.0.1", port)
		conn, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return "http://" + addr
		}
	}
	return ""
}

func readAllLimit(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}
