package app

// Release 版本检查：GitHub latest release 对比本地版本。
//
// 通道顺序：手动代理（设置里填写才启用）→ 域名固定 IP 直连（SNI 正确、证书正常校验）→ 系统直连。
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
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"qqsecond/internal/version"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	releasesAPIURL  = "https://api.github.com/repos/Admirepowered/qq-second/releases/latest"
	releasesPageURL = "https://github.com/Admirepowered/qq-second/releases/latest"
)

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
	Platform       string `json:"platform"`   // windows | macos | other
	AssetURL       string `json:"assetUrl"`   // 当前平台对应资产下载地址
	AssetName      string `json:"assetName"`
}

// lastGoodChannel 记住上次成功的通道，后续检查优先尝试。
var lastGoodChannel atomic.Value // string

// CheckUpdate 按多级通道查询最新版本。
func (a *App) CheckUpdate() (*UpdateInfo, error) {
	var errs []string
	channels := []string{"pinned", "plain"}
	// 只在手动设置了代理时启用 proxy 通道（不做任何自动探测）。
	if a.manualProxy() != "" {
		channels = append([]string{"proxy"}, channels...)
	}
	if good, ok := lastGoodChannel.Load().(string); ok && good != "" {
		channels = append([]string{good}, channels...)
	}
	var errsSeen = map[string]bool{}
	for _, ch := range channels {
		if errsSeen[ch] {
			continue
		}
		errsSeen[ch] = true
		info, err := a.checkUpdateVia(ch)
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

func (a *App) checkUpdateVia(channel string) (*UpdateInfo, error) {
	client, err := a.githubClient(channel)
	if err != nil {
		return nil, err
	}

	// 1) API JSON
	if body, _, err := githubGet(client, releasesAPIURL); err == nil {
		var release struct {
			TagName string `json:"tag_name"`
			HTMLURL string `json:"html_url"`
			Body    string `json:"body"`
			Assets  []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if json.Unmarshal([]byte(body), &release) == nil && release.TagName != "" {
			latest := strings.TrimPrefix(release.TagName, "v")
			info := &UpdateInfo{
				CurrentVersion: version.Version,
				LatestVersion:  latest,
				ReleaseURL:     release.HTMLURL,
				ReleaseNotes:   truncateNotes(release.Body, 500),
				CheckedAt:      time.Now().Unix(),
				HasUpdate:      latest != "" && latest != version.Version,
			}
			a.fillPlatformAsset(info, latest)
			// API 列表里的资产地址优先
			want := assetName(latest)
			for _, asset := range release.Assets {
				if asset.Name == want {
					info.AssetURL = asset.BrowserDownloadURL
					info.AssetName = asset.Name
					break
				}
			}
			a.mu.Lock()
			a.updateInfo = info
			a.mu.Unlock()
			return info, nil
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
	info := &UpdateInfo{
		CurrentVersion: version.Version,
		LatestVersion:  tag,
		ReleaseURL:     releasesPageURL,
		CheckedAt:      time.Now().Unix(),
		HasUpdate:      tag != version.Version,
	}
	a.fillPlatformAsset(info, tag)
	a.mu.Lock()
	a.updateInfo = info
	a.mu.Unlock()
	return info, nil
}

// fillPlatformAsset 填充平台与默认资产地址（redirect 通道没有资产列表，按命名约定构造）。
func (a *App) fillPlatformAsset(info *UpdateInfo, latest string) {
	switch runtime.GOOS {
	case "windows":
		info.Platform = "windows"
	case "darwin":
		info.Platform = "macos"
	default:
		info.Platform = "other"
	}
	name := assetName(latest)
	if name != "" && info.AssetURL == "" {
		info.AssetName = name
		info.AssetURL = fmt.Sprintf("https://github.com/Admirepowered/qq-second/releases/download/v%s/%s", latest, name)
	}
}

// manualProxy 读取手动设置的代理地址（不做任何自动探测）。
func (a *App) manualProxy() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.TrimSpace(a.store.Settings.UpdateProxy)
}

// githubClient 按通道构造 HTTP client。
func (a *App) githubClient(channel string) (*http.Client, error) {
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
		return &http.Client{Timeout: 6 * time.Second, Transport: tr}, nil
	case "proxy":
		proxyStr := a.manualProxy()
		if proxyStr == "" {
			return nil, fmt.Errorf("未设置代理")
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
		return &http.Client{Timeout: 6 * time.Second, Transport: tr}, nil
	default:
		return &http.Client{Timeout: 6 * time.Second}, nil
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

// OpenReleasePage 用系统浏览器打开 Release 页面。
func (a *App) OpenReleasePage() {
	if a.ctx != nil {
		wailsruntime.BrowserOpenURL(a.ctx, releasesPageURL)
	}
}

func truncateNotes(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func readAllLimit(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}
