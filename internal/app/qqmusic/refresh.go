package qqmusic

// 凭据刷新：music.login.LoginServer / Login。
//
// 关键实测结论（决定整个续期策略）：
//   - 登录响应的 musickey 只有 **3 天** 有效期（keyExpiresIn=259200，
//     musickeyCreateTime 为签发时间）。
//   - 有效期内：只凭 musickey + musicid 就能刷新出新 key（旧版登录没存
//     refresh_token 也能刷）。
//   - **过期之后：用这把 key 再刷新会返回 login failed 1000，任何参数形态都救不回来**
//     （已穷举 loginMode/tmeLoginType/空 token 字段等 10 种组合验证）。
//   - 过期后账号读接口（收藏/搜索）仍正常，只有播放权益失效 —— 所以绝不能靠
//     「播放失败时再刷新」保命，必须**在过期前主动续期**。
//
// 因此：续期要在剩余寿命过半时就开始，并且刷新失败要退避，不能每次播放都重试。

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoRefreshToken 表示凭据不足以刷新（既无 refresh_token 也无 musickey）。
var ErrNoRefreshToken = errors.New("凭据不足以刷新，请重新扫码登录")

// ErrKeyExpired 表示 musickey 已超过有效期，只能重新扫码登录。
var ErrKeyExpired = errors.New("登录密钥已过期（有效期 3 天），请重新扫码登录")

// KeyLifetime 是服务端签发 key 的有效期（实测 keyExpiresIn=259200 秒）。
const KeyLifetime = 72 * time.Hour

// SessionInvalid 清空缓存（session + 刷新状态），用于重新登录后重置。
func (c *Client) SessionInvalid() {
	c.mu.Lock()
	c.uid = ""
	c.sid = ""
	c.refreshFailAt = 0
	c.refreshFails = 0
	c.keyIssuedAt = 0
	c.mu.Unlock()
}

// SetKeyIssuedAt 记录当前 key 的签发时间（登录/刷新成功后由调用方设置）。
func (c *Client) SetKeyIssuedAt(t time.Time) {
	c.mu.Lock()
	c.keyIssuedAt = t.Unix()
	c.refreshFailAt = 0
	c.refreshFails = 0
	c.mu.Unlock()
}

// KeyIssuedAt 返回当前 key 的签发时间（未知时为零值）。
func (c *Client) KeyIssuedAt() time.Time {
	c.mu.Lock()
	ts := c.keyIssuedAt
	c.mu.Unlock()
	if ts <= 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}

// KeyExpired 判断 key 是否已过期（未知签发时间时按未过期处理）。
func (c *Client) KeyExpired() bool {
	issued := c.KeyIssuedAt()
	if issued.IsZero() {
		return false
	}
	return time.Since(issued) > KeyLifetime
}

// KeyAge 返回 key 已存在时长。
func (c *Client) KeyAge() time.Duration {
	issued := c.KeyIssuedAt()
	if issued.IsZero() {
		return 0
	}
	return time.Since(issued)
}

// InvalidateSession 丢弃缓存的 Android session（uid/sid）。
//
// 实测：播放权益校验依赖「新签发的 key + 新申请的 session」这一组合。session
// 或 key 变旧后，vkey 对年费账号也返回 code=1000（整组拒绝）。丢弃 session 让
// 下次请求重新 GetSession，通常即可恢复；仍不行再走 RefreshCredential 换 key。
func (c *Client) InvalidateSession() {
	c.mu.Lock()
	c.uid = ""
	c.sid = ""
	c.mu.Unlock()
}

// CanAttemptRefresh 判断现在是否值得尝试刷新。
//
// 失败退避：同一次失效只试一次，失败后 10 分钟内不再重试，避免播放失败时
// 每首歌都打一次刷新请求（历史问题：日志几分钟内刷了几十条 login failed 1000）。
func (c *Client) CanAttemptRefresh() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refreshFailAt == 0 {
		return true
	}
	// 失败次数越多退避越久：10min, 20min, 40min…（上限 2h）
	backoff := 10 * time.Minute
	for i := 1; i < c.refreshFails && i < 4; i++ {
		backoff *= 2
	}
	if backoff > 2*time.Hour {
		backoff = 2 * time.Hour
	}
	return time.Since(time.Unix(c.refreshFailAt, 0)) > backoff
}

// markRefreshFailed 记录刷新失败（用于退避）。
func (c *Client) markRefreshFailed() {
	c.mu.Lock()
	c.refreshFailAt = time.Now().Unix()
	c.refreshFails++
	c.mu.Unlock()
}

// RefreshStatus 返回当前刷新退避状态（空 = 可刷新）。
func (c *Client) RefreshStatus() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refreshFailAt == 0 {
		return ""
	}
	if !c.canAttemptRefreshLocked() {
		return fmt.Sprintf("刷新失败 %d 次，暂停重试", c.refreshFails)
	}
	return ""
}

func (c *Client) canAttemptRefreshLocked() bool {
	backoff := 10 * time.Minute
	for i := 1; i < c.refreshFails && i < 4; i++ {
		backoff *= 2
	}
	if backoff > 2*time.Hour {
		backoff = 2 * time.Hour
	}
	return time.Since(time.Unix(c.refreshFailAt, 0)) > backoff
}

// RefreshCredential 换取新凭据（免扫码）。
//
// 有效期内只凭 musickey + musicid 即可；**过期后必然失败**（返回 ErrKeyExpired，
// 让上层给出「重新扫码」的明确指引，而不是反复打无意义的请求）。
func (c *Client) RefreshCredential() (*Credential, error) {
	c.mu.Lock()
	cred := c.cred
	c.mu.Unlock()
	if cred.MusicKey == "" || cred.StrMusicID == "" {
		return nil, ErrNotLoggedIn
	}
	if c.KeyExpired() {
		return nil, ErrKeyExpired
	}

	param := map[string]interface{}{
		"musicid":   cred.MusicID,
		"musickey":  cred.MusicKey,
		"loginMode": 2,
	}
	if cred.Refreshable() {
		param["openid"] = cred.OpenID
		param["access_token"] = cred.AccessToken
		param["refresh_token"] = cred.RefreshToken
		param["refresh_key"] = cred.RefreshKey
		param["expired_in"] = cred.ExpiredAt
	}
	if cred.StrMusicID != "" {
		param["str_musicid"] = cred.StrMusicID
	}

	data, err := c.cgiWithCommOverride("music.login.LoginServer", "Login",
		param, map[string]interface{}{"tmeLoginType": loginTypeOrDefault(cred.LoginType)})
	if err != nil {
		c.markRefreshFailed()
		return nil, err
	}
	fresh, err := credentialFromLoginData(data, cred)
	if err != nil {
		c.markRefreshFailed()
		return nil, err
	}

	// 旧鉴权下申请的 Android session 不再可用，清掉让下次请求重新获取。
	c.mu.Lock()
	c.uid = ""
	c.sid = ""
	c.mu.Unlock()
	return fresh, nil
}

// credentialFromLoginData 从 Login/refresh 响应解析新凭据（字段名兼容蛇形/驼峰）。
func credentialFromLoginData(data map[string]interface{}, prev Credential) (*Credential, error) {
	musicID, _ := numberAt(data, "musicid")
	musicKey := firstField(data, "musickey", "musicKey")
	strMusicID := firstField(data, "strMusicid", "str_musicid", "strMusicId")
	if strMusicID == "" {
		strMusicID = firstField(data, "uin")
	}
	if strMusicID == "" {
		strMusicID = prev.StrMusicID
	}
	if musicID == 0 {
		musicID = prev.MusicID
	}
	if strMusicID == "" || musicKey == "" {
		return nil, errors.New("refresh response incomplete (musickey/strMusicid missing)")
	}

	fresh := Credential{
		MusicID:    musicID,
		StrMusicID: strMusicID,
		MusicKey:   musicKey,
		// 刷新响应里 nick 可能为空：沿用旧昵称，否则界面上会变成一串 uin
		Nickname: pickOr(data, prev.Nickname, "nick", "nickname"),
		// 刷新响应可能不再下发刷新令牌：沿用上一次的（官方客户端同此处理）
		OpenID:       pickOr(data, prev.OpenID, "openid"),
		RefreshToken: pickOr(data, prev.RefreshToken, "refresh_token", "refreshToken"),
		AccessToken:  pickOr(data, prev.AccessToken, "access_token", "accessToken"),
		RefreshKey:   pickOr(data, prev.RefreshKey, "refresh_key", "refreshKey"),
		UnionID:      pickOr(data, prev.UnionID, "unionid"),
		LoginType:    prev.LoginType,
	}
	if t, ok := numberAt(data, "musickeyCreateTime"); ok && t != 0 {
		fresh.MusickeyCreateTime = t
	} else {
		fresh.MusickeyCreateTime = prev.MusickeyCreateTime
	}
	if t, ok := numberAt(data, "keyExpiresIn"); ok && t != 0 {
		fresh.KeyExpiresIn = t
	} else {
		fresh.KeyExpiresIn = prev.KeyExpiresIn
	}
	// 到期时间 = 签发时间 + 有效期（签发时间可能是毫秒或秒）
	created := fresh.MusickeyCreateTime
	if created > 0 && fresh.KeyExpiresIn > 0 {
		if created < 1_000_000_000_000 {
			created *= 1000
		}
		fresh.ExpiredAt = (created + fresh.KeyExpiresIn*1000) / 1000
	} else {
		fresh.ExpiredAt = prev.ExpiredAt
	}
	return &fresh, nil
}

// pickOr 从响应里按候选键取非空值，都没有则用 fallback。
func pickOr(m map[string]interface{}, fallback string, keys ...string) string {
	if v := firstField(m, keys...); v != "" {
		return v
	}
	return fallback
}
