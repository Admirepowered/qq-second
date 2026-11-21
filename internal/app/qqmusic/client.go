package qqmusic

// QQ 音乐 CGI 客户端：平台 comm 构建、Android session、musicu.fcg/musics.fcg 请求。
// 参考公开协议行为实现，与 qm-api-rs context.rs / versioning.rs 等价。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const cgiBase = "https://u.y.qq.com/cgi-bin"

// Credential 是登录凭据。
type Credential struct {
	MusicID    int64  `json:"musicid"`
	StrMusicID string `json:"strMusicId"`
	MusicKey   string `json:"musickey"`
	LoginType  int    `json:"loginType"`
	Nickname   string `json:"nickname"`
}

// platform 请求平台。
type platform int

const (
	platformAndroid platform = iota
	platformWeb
)

// Client 是 QQ 音乐 API 客户端（并发安全）。
type Client struct {
	http       *http.Client
	loginHTTP  *http.Client // 扫码登录专用（带 cookie jar）
	mu         sync.Mutex
	cred       Credential
	dev        *Device
	uid        string
	sid        string
	sidAt      int64
	qrsig      string
	onIdentity func()
}

// SetIdentityChangedHook 设置身份变更回调（用于持久化）。
func (c *Client) SetIdentityChangedHook(fn func()) {
	c.mu.Lock()
	c.onIdentity = fn
	c.mu.Unlock()
}

// identitySnapshot 是可持久化的设备/会话快照。
type identitySnapshot struct {
	Device *Device `json:"device"`
	UID    string  `json:"uid"`
	SID    string  `json:"sid"`
	SidAt  int64   `json:"sidAt"`
}

// SaveIdentity 保存设备身份与会话。
func (c *Client) SaveIdentity() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	snap := identitySnapshot{Device: c.dev, UID: c.uid, SID: c.sid, SidAt: c.sidAt}
	data, _ := json.MarshalIndent(snap, "", " ")
	return data
}

// LoadIdentity 恢复设备身份与会话。
func (c *Client) LoadIdentity(data []byte) {
	var snap identitySnapshot
	if err := json.Unmarshal(data, &snap); err != nil || snap.Device == nil {
		return
	}
	c.mu.Lock()
	c.dev = snap.Device
	c.uid = snap.UID
	c.sid = snap.SID
	c.sidAt = snap.SidAt
	c.mu.Unlock()
}

// rotateDevice 更换设备身份（限流时使用）。
func (c *Client) rotateDevice() {
	c.mu.Lock()
	c.dev = RandomDevice()
	c.uid = ""
	c.sid = ""
	c.sidAt = 0
	c.mu.Unlock()
}

// NewClient 创建客户端。
func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 20 * time.Second},
		dev:  RandomDevice(),
	}
}

// SetCredential 更新登录凭据（同时失效 Android session 缓存）。
func (c *Client) SetCredential(cred Credential) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cred = cred
	c.uid = ""
	c.sid = ""
}

// Credential 读取凭据快照。
func (c *Client) Credential() Credential {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cred
}

// gTk 计算 g_tk。
func gTk(musicKey string) int64 {
	if musicKey == "" {
		return 5381
	}
	return hash33(musicKey, 5381)
}

// buildComm 构建平台 comm。
func (c *Client) buildComm(p platform) map[string]interface{} {
	c.mu.Lock()
	cred := c.cred
	dev := c.dev
	uid, sid := c.uid, c.sid
	c.mu.Unlock()

	switch p {
	case platformWeb:
		return map[string]interface{}{
			"ct": 24, "cv": 4747474, "platform": "yqq.json", "chid": "0",
			"uin": cred.StrMusicID,
			"g_tk": gTk(cred.MusicKey), "g_tk_new_20200303": gTk(cred.MusicKey),
			"format": "json", "inCharset": "utf-8", "outCharset": "utf-8",
			"notice": 0, "need_new_code": 1,
		}
	default: // android
		comm := map[string]interface{}{
			"ct": 11, "cv": 14090008, "v": 14090008, "chid": "10003505",
			"qq":        strconv.FormatInt(cred.MusicID, 10),
			"authst":    cred.MusicKey,
			"tmeAppID":  "qqmusic",
			"tmeLoginType": loginTypeOrDefault(cred.LoginType),
			"QIMEI":     dev.Qimei,
			"QIMEI36":   dev.Qimei36,
			"OpenUDID":  dev.OpenUDID,
			"udid":      dev.OpenUDID,
			"uid":       uid,
			"OpenUDID2": dev.OpenUDID,
			"sid":       sid,
			"aid":       dev.AndroidID,
			"os_ver":    dev.Version.Release,
			"phonetype": dev.Model,
			"devicelevel":   strconv.Itoa(dev.Version.SDK),
			"newdevicelevel": strconv.Itoa(dev.Version.SDK),
			"rom":       dev.Fingerprint,
		}
		return comm
	}
}

func loginTypeOrDefault(t int) int {
	if t <= 0 {
		return 2
	}
	return t
}

// ensureQimei 在 QIMEI 缺失或过期时申请并写回设备。
func (c *Client) ensureQimei() error {
	c.mu.Lock()
	dev := c.dev
	fresh := dev.Qimei != "" && dev.Qimei36 != "" && time.Now().Unix()-dev.QimeiSaveAt < 86400
	c.mu.Unlock()
	if fresh {
		return nil
	}
	q16, q36, err := fetchQimei(dev, c.http)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.dev.Qimei = q16
	c.dev.Qimei36 = q36
	c.dev.QimeiSaveAt = time.Now().Unix()
	c.mu.Unlock()
	return nil
}

// ensureSession 获取（并缓存 24h）Android session。
func (c *Client) ensureSession() error {
	c.mu.Lock()
	valid := c.uid != "" && c.sid != "" && time.Now().Unix()-c.sidAt < 86400
	c.mu.Unlock()
	if valid {
		return nil
	}
	if err := c.ensureQimei(); err != nil {
		return fmt.Errorf("qimei: %w", err)
	}
	comm := c.buildComm(platformAndroid)
	payload := map[string]interface{}{
		"comm": comm,
		"req_0": map[string]interface{}{
			"module": "music.getSession.session",
			"method": "GetSession",
			"param":  map[string]interface{}{"uid": "", "vkey": 0, "caller": 0},
		},
	}
	data, err := c.postCGI(cgiBase+"/musicu.fcg", payload, nil)
	if err != nil {
		return err
	}
	session, _ := dig(data, "req_0", "data", "session")
	uid := stringAt(session, "uid")
	sid := stringAt(session, "sid")
	if uid == "" || sid == "" {
		return errors.New("get session failed")
	}
	c.mu.Lock()
	c.uid = uid
	c.sid = sid
	c.sidAt = time.Now().Unix()
	hook := c.onIdentity
	c.mu.Unlock()
	if hook != nil {
		hook()
	}
	return nil
}

// requestHeaders 组装 CGI 请求头（凭据以 Cookie 形式显式写入）。
func (c *Client) requestHeaders() [][2]string {
	c.mu.Lock()
	cred := c.cred
	c.mu.Unlock()

	headers := [][2]string{
		{"Content-Type", "application/json"},
		{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"},
		{"Referer", "https://y.qq.com/"},
		{"Origin", "https://y.qq.com"},
	}
	var cookies []string
	if cred.StrMusicID != "" {
		cookies = append(cookies, "uin="+cred.StrMusicID, "qqmusic_uin="+cred.StrMusicID)
	}
	if cred.MusicKey != "" {
		cookies = append(cookies, "qm_keyst="+cred.MusicKey, "qqmusic_key="+cred.MusicKey)
	}
	if len(cookies) > 0 {
		headers = append(headers, [2]string{"Cookie", strings.Join(cookies, "; ")})
	}
	return headers
}

// postCGI 发送 JSON 请求并返回整个响应 JSON（map）。
func (c *Client) postCGI(url string, payload interface{}, extraHeaders [][2]string) (map[string]interface{}, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for _, h := range c.requestHeaders() {
		req.Header.Set(h[0], h[1])
	}
	for _, h := range extraHeaders {
		req.Header.Set(h[0], h[1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// cgi 发起一次 CGI 调用，返回 req_0.data（业务 code 由调用方判断）。
func (c *Client) cgi(p platform, module, method string, param interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"comm": c.buildComm(p),
		"req_0": map[string]interface{}{
			"module": module,
			"method": method,
			"param":  param,
		},
	}
	url := cgiBase + "/musicu.fcg"
	var out map[string]interface{}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		out, lastErr = c.postCGI(url, payload, nil)
		if lastErr == nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	globalCode, _ := numberAt(out, "code")
	if globalCode != 0 {
		return nil, fmt.Errorf("global api code %v", globalCode)
	}
	req0, _ := dig(out, "req_0")
	if req0 == nil {
		return nil, errors.New("missing req_0")
	}
	code, _ := numberAt(req0, "code")
	data, _ := dig(req0, "data")
	if data == nil {
		data = map[string]interface{}{}
	}
	if code != 0 {
		if msg := stringAt(data, "message"); msg != "" {
			return data, &CGIError{Code: int(code), Message: msg}
		}
		return data, &CGIError{Code: int(code)}
	}
	return data, nil
}

// cgiAndroid Android 平台调用（自动补 QIMEI + session；限流 2001 时换设备退避重试）。
func (c *Client) cgiAndroid(module, method string, param interface{}) (map[string]interface{}, error) {
	data, err := c.cgiAndroidOnce(module, method, param)
	if err == nil {
		return data, nil
	}
	var cgiErr *CGIError
	backoffs := []time.Duration{1200 * time.Millisecond, 4 * time.Second}
	for _, wait := range backoffs {
		if !errors.As(err, &cgiErr) || cgiErr.Code != 2001 {
			return nil, err
		}
		fmt.Println("rate limited (2001), rotating device and retrying after", wait)
		time.Sleep(wait)
		c.rotateDevice()
		data, err = c.cgiAndroidOnce(module, method, param)
		if err == nil {
			return data, nil
		}
	}
	return nil, err
}

func (c *Client) cgiAndroidOnce(module, method string, param interface{}) (map[string]interface{}, error) {
	if err := c.ensureQimei(); err != nil {
		// QIMEI 申请失败不致命：不带指纹继续尝试。
		fmt.Println("qimei failed:", err)
	}
	if err := c.ensureSession(); err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}
	return c.cgi(platformAndroid, module, method, param)
}

// cgiAnonymousWeb 匿名 Web 平台调用（发现页/榜单/歌词等公开数据）。
func (c *Client) cgiAnonymousWeb(module, method string, param interface{}) (map[string]interface{}, error) {
	c.mu.Lock()
	cred := c.cred
	c.mu.Unlock()
	comm := map[string]interface{}{
		"ct": 24, "cv": 0,
		"uin": cred.StrMusicID, "g_tk": gTk(cred.MusicKey), "g_tk_new_20200303": gTk(cred.MusicKey),
		"format": "json", "inCharset": "utf-8", "outCharset": "utf-8",
		"notice": 0, "need_new_code": 1,
		"platform": "yqq.json", "chid": "0",
	}
	payload := map[string]interface{}{
		"comm": comm,
		"req_0": map[string]interface{}{
			"module": module,
			"method": method,
			"param":  param,
		},
	}
	out, err := c.postCGI(cgiBase+"/musicu.fcg", payload, nil)
	if err != nil {
		return nil, err
	}
	if code, _ := numberAt(out, "code"); code != 0 {
		return nil, fmt.Errorf("global api code %v", code)
	}
	req0, _ := dig(out, "req_0")
	if req0 == nil {
		return nil, errors.New("missing req_0")
	}
	code, _ := numberAt(req0, "code")
	data, _ := dig(req0, "data")
	if data == nil {
		data = map[string]interface{}{}
	}
	if code != 0 {
		if msg := stringAt(data, "message"); msg != "" {
			return data, &CGIError{Code: int(code), Message: msg}
		}
		return data, &CGIError{Code: int(code)}
	}
	return data, nil
}

// getSearchID 生成随机 searchid。
func getSearchID() string {
	return strconv.FormatInt(rand.Int63n(9_000_000_000)+1_000_000_000, 10)
}

func getGUID() string { return uuidSimple() }

// CGIError 是业务错误。
type CGIError struct {
	Code    int
	Message string
}

func (e *CGIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("cgi %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("cgi %d", e.Code)
}

// ---- JSON 帮助函数 ----

func dig(m map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func stringAt(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func numberAt(m map[string]interface{}, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

func boolAt(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	switch v := m[key].(type) {
	case bool:
		return v
	case float64:
		return v != 0
	}
	return false
}

func arrayAt(m map[string]interface{}, key string) []interface{} {
	if m == nil {
		return nil
	}
	v, _ := m[key].([]interface{})
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
