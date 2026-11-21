package qqmusic

// QQ 扫码登录：ptqrshow → ptqrlogin → check_sig → oauth authorize → QQLogin CGI。

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	qqQRAppID    = "716027609"
	qqQRDaID     = "383"
	qqPT3rdAID   = "100497308"
	ptReferer    = "https://xui.ptlogin2.qq.com/"
)

// QRLoginState 登录二维码状态。
type QRLoginState struct {
	Identifier string `json:"-"` // qrsig
}

// QRCodeImage 二维码图。
type QRCodeImage struct {
	DataURL string `json:"dataUrl"`
	State   string `json:"state"`
}

// QRLoginPoll 轮询结果。
type QRLoginPoll struct {
	Status     string `json:"status"` // waiting | scanned | expired | done | refuse | error
	Message    string `json:"message"`
	Credential *Credential `json:"credential,omitempty"`
}

var ptuiCBRe = regexp.MustCompile(`ptuiCB\((.*)\)`)
var quotedRe = regexp.MustCompile(`'([^']*)'`)

// StartQRLogin 获取登录二维码（qrsig 保存到会话状态；整个登录流程共用一个 cookie jar）。
func (c *Client) StartQRLogin() (*QRCodeImage, error) {
	// 登录链路（ptqrshow → ptqrlogin → check_sig → authorize）共享 cookie jar，
	// authorize 需要 check_sig 写入的全部 ptlogin 会话 cookie。
	jar, _ := cookiejar.New(nil)
	loginHTTP := &http.Client{Timeout: c.http.Timeout, Jar: jar}
	c.mu.Lock()
	c.loginHTTP = loginHTTP
	c.mu.Unlock()

	params := url.Values{}
	params.Set("appid", qqQRAppID)
	params.Set("e", "2")
	params.Set("l", "M")
	params.Set("s", "3")
	params.Set("d", "72")
	params.Set("v", "4")
	params.Set("t", fmt.Sprintf("%f", float64(time.Now().UnixMilli())/1000.0))
	params.Set("daid", qqQRDaID)
	params.Set("pt_3rd_aid", qqPT3rdAID)

	req, err := http.NewRequest(http.MethodGet, "https://ssl.ptlogin2.qq.com/ptqrshow?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", ptReferer)
	req.Header.Set("User-Agent", browserUA)

	resp, err := loginHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ptqrshow http %d", resp.StatusCode)
	}
	qrsig := extractSetCookie(resp.Header, "qrsig")
	if qrsig == "" {
		return nil, fmt.Errorf("qrsig missing")
	}
	return &QRCodeImage{
		DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw),
		State:   "waiting",
	}, saveQrsig(c, qrsig)
}

// saveQrsig 把 qrsig 临时存到 client（不落盘）。
func saveQrsig(c *Client, qrsig string) error {
	c.mu.Lock()
	c.qrsig = qrsig
	c.mu.Unlock()
	return nil
}

// PollQRLogin 轮询扫码状态；done 时返回凭据。
func (c *Client) PollQRLogin() (*QRLoginPoll, error) {
	c.mu.Lock()
	qrsig := c.qrsig
	c.mu.Unlock()
	if qrsig == "" {
		return nil, fmt.Errorf("no qr session")
	}

	ptqrtoken := hash33(qrsig, 0)
	params := url.Values{}
	params.Set("u1", "https://graph.qq.com/oauth2.0/login_jump")
	params.Set("ptqrtoken", strconv.FormatInt(ptqrtoken, 10))
	params.Set("ptredirect", "0")
	params.Set("h", "1")
	params.Set("t", "1")
	params.Set("g", "1")
	params.Set("from_ui", "1")
	params.Set("ptlang", "2052")
	params.Set("action", fmt.Sprintf("0-0-%d", time.Now().UnixMilli()))
	params.Set("js_ver", "20102616")
	params.Set("js_type", "1")
	params.Set("pt_uistyle", "40")
	params.Set("aid", qqQRAppID)
	params.Set("daid", qqQRDaID)
	params.Set("pt_3rd_aid", qqPT3rdAID)
	params.Set("has_onekey", "1")

	req, err := http.NewRequest(http.MethodGet, "https://ssl.ptlogin2.qq.com/ptqrlogin?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", ptReferer)
	req.Header.Set("Cookie", "qrsig="+qrsig)
	req.Header.Set("User-Agent", browserUA)

	loginHTTP := c.loginHTTPClient()
	if loginHTTP == nil {
		return nil, fmt.Errorf("no qr session")
	}
	resp, err := loginHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	text := string(raw)

	cb := ptuiCBRe.FindStringSubmatch(text)
	if cb == nil {
		return nil, fmt.Errorf("bad ptqrlogin response")
	}
	args := quotedRe.FindAllStringSubmatch(cb[1], -1)
	if len(args) < 1 {
		return nil, fmt.Errorf("bad ptuiCB args")
	}
	code, _ := strconv.Atoi(args[0][1])
	switch code {
	case 66, 408:
		return &QRLoginPoll{Status: "waiting", Message: "等待扫码"}, nil
	case 67, 404:
		return &QRLoginPoll{Status: "scanned", Message: "已扫码，等待确认"}, nil
	case 65, 402:
		c.mu.Lock()
		c.qrsig = ""
		c.mu.Unlock()
		return &QRLoginPoll{Status: "expired", Message: "二维码已过期"}, nil
	case 68, 403:
		return &QRLoginPoll{Status: "refuse", Message: "已取消登录"}, nil
	case 0, 405:
		// 登录成功
	default:
		msg := ""
		if len(args) >= 5 {
			msg = args[4][1]
		}
		return nil, fmt.Errorf("qr login code %d: %s", code, msg)
	}
	if len(args) < 3 {
		return nil, fmt.Errorf("missing redirect url")
	}
	redirectURL := args[2][1]

	sigx := extractBetween(redirectURL, "ptsigx=", "&s_url")
	uin := extractBetween(redirectURL, "uin=", "&service")
	if sigx == "" || uin == "" {
		return nil, fmt.Errorf("bad redirect url")
	}

	cred, err := c.authorizeQQQR(uin, sigx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.qrsig = ""
	c.mu.Unlock()
	return &QRLoginPoll{Status: "done", Credential: cred}, nil
}

// loginHTTPClient 返回登录流程专用 client（带 cookie jar）。
func (c *Client) loginHTTPClient() *http.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loginHTTP
}

// authorizeQQQR check_sig → oauth authorize → QQLogin。
func (c *Client) authorizeQQQR(uin, sigx string) (*Credential, error) {
	loginHTTP := c.loginHTTPClient()
	if loginHTTP == nil {
		return nil, fmt.Errorf("no qr session")
	}
	// 1. check_sig 拿 p_skey
	params := url.Values{}
	params.Set("uin", uin)
	params.Set("pttype", "1")
	params.Set("service", "ptqrlogin")
	params.Set("nodirect", "0")
	params.Set("ptsigx", sigx)
	params.Set("s_url", "https://graph.qq.com/oauth2.0/login_jump")
	params.Set("ptlang", "2052")
	params.Set("ptredirect", "100")
	params.Set("aid", qqQRAppID)
	params.Set("daid", qqQRDaID)
	params.Set("j_later", "0")
	params.Set("low_login_hour", "0")
	params.Set("regmaster", "0")
	params.Set("pt_login_type", "3")
	params.Set("pt_aid", "0")
	params.Set("pt_aaid", "16")
	params.Set("pt_light", "0")
	params.Set("pt_3rd_aid", qqPT3rdAID)

	req, err := http.NewRequest(http.MethodGet, "https://ssl.ptlogin2.graph.qq.com/check_sig?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", ptReferer)
	req.Header.Set("User-Agent", browserUA)

	// 不跟随 check_sig 的重定向（jar 已记录 cookie）
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: c.http.Timeout,
		Jar:     loginHTTP.Jar,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	// check_sig 会为多个域设置 p_skey；graph.qq.com 域的才是 authorize 需要的。
	pSkey := extractSetCookieForDomain(resp.Header, "p_skey", "graph.qq.com")
	if pSkey == "" {
		pSkey = extractSetCookie(resp.Header, "p_skey")
	}
	if pSkey == "" {
		return nil, fmt.Errorf("p_skey missing")
	}

	// 2. oauth authorize 拿 code
	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("client_id", qqPT3rdAID)
	form.Set("redirect_uri", "https://y.qq.com/portal/wx_redirect.html?login_type=1&surl=https://y.qq.com/")
	form.Set("scope", "get_user_info,get_app_friends")
	form.Set("state", "state")
	form.Set("switch", "")
	form.Set("from_ptlogin", "1")
	form.Set("src", "1")
	form.Set("update_auth", "1")
	form.Set("openapi", "1010_1030")
	form.Set("g_tk", strconv.FormatInt(hash33(pSkey, 5381), 10))
	form.Set("auth_time", fmt.Sprintf("%d", time.Now().UnixMilli()))

	authReq, err := http.NewRequest(http.MethodPost, "https://graph.qq.com/oauth2.0/authorize", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	authReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authReq.Header.Set("Referer", ptReferer)
	// 显式补一份核心凭据（jar 里的 cookie 之外）
	authReq.Header.Set("Cookie", "uin="+uin+"; p_skey="+pSkey)
	authReq.Header.Set("User-Agent", browserUA)

	// 跟随完整重定向链（带 jar），从最终 URL 提取 code。
	authClient := &http.Client{Timeout: c.http.Timeout, Jar: loginHTTP.Jar}
	authResp, err := authClient.Do(authReq)
	if err != nil {
		return nil, err
	}
	authResp.Body.Close()
	finalURL := authResp.Request.URL.String()
	authCode := queryParam(finalURL, "code")
	if authCode == "" {
		return nil, fmt.Errorf("oauth code missing (final=%s)", truncateURL(finalURL))
	}

	// 3. QQLogin CGI 换取凭据
	data, err := c.cgiWithCommOverride("QQConnectLogin.LoginServer", "QQLogin",
		map[string]interface{}{"code": authCode},
		map[string]interface{}{"tmeLoginType": 2})
	if err != nil {
		return nil, err
	}
	musicID, _ := numberAt(data, "musicid")
	musicKey := stringAt(data, "musickey")
	strMusicID := stringAt(data, "strMusicid")
	if strMusicID == "" {
		strMusicID = stringAt(data, "uin")
	}
	if strMusicID == "" {
		strMusicID = strconv.FormatInt(musicID, 10)
	}
	if musicID == 0 || musicKey == "" {
		return nil, fmt.Errorf("credential invalid")
	}
	nickname := stringAt(data, "nick")
	return &Credential{
		MusicID:    musicID,
		StrMusicID: strMusicID,
		MusicKey:   musicKey,
		LoginType:  2,
		Nickname:   nickname,
	}, nil
}

// cgiWithCommOverride 以 comm 覆盖方式调用 CGI。
func (c *Client) cgiWithCommOverride(module, method string, param, commOverride map[string]interface{}) (map[string]interface{}, error) {
	comm := c.buildComm(platformAndroid)
	for k, v := range commOverride {
		comm[k] = v
	}
	if err := c.ensureQimei(); err != nil {
		fmt.Println("qimei failed:", err)
	}
	if err := c.ensureSession(); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("missing req_0")
	}
	code, _ := numberAt(req0, "code")
	data, _ := dig(req0, "data")
	if data == nil {
		data = map[string]interface{}{}
	}
	if code != 0 {
		if msg := stringAt(data, "message"); msg != "" {
			return nil, fmt.Errorf("login failed %d: %s", code, msg)
		}
		return nil, fmt.Errorf("login failed %d", code)
	}
	return data, nil
}

func extractSetCookie(header http.Header, name string) string {
	for _, sc := range header.Values("Set-Cookie") {
		parts := strings.SplitN(sc, ";", 2)
		kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
		if len(kv) == 2 && strings.EqualFold(kv[0], name) {
			return kv[1]
		}
	}
	return ""
}

// extractSetCookieForDomain 提取指定域（含子域匹配）的 Set-Cookie 值。
func extractSetCookieForDomain(header http.Header, name, domain string) string {
	for _, sc := range header.Values("Set-Cookie") {
		attrs := strings.Split(sc, ";")
		if len(attrs) == 0 {
			continue
		}
		kv := strings.SplitN(strings.TrimSpace(attrs[0]), "=", 2)
		if len(kv) != 2 || !strings.EqualFold(kv[0], name) {
			continue
		}
		for _, attr := range attrs[1:] {
			attr = strings.TrimSpace(attr)
			if d, ok := strings.CutPrefix(strings.ToLower(attr), "domain="); ok && strings.HasSuffix(d, strings.ToLower(domain)) {
				return kv[1]
			}
		}
	}
	return ""
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := strings.Index(s[i:], end)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

func queryParam(rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

func truncateURL(rawURL string) string {
	if len(rawURL) > 160 {
		return rawURL[:160] + "..."
	}
	return rawURL
}

// Logout 清除凭据。
func (c *Client) Logout() {
	c.SetCredential(Credential{})
}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
