package qqmusic

// 凭据刷新：music.login.LoginServer / Login。
//
// 背景：musickey 本身长期有效（账号读接口一直正常），但**播放权益校验只认新签发
// 的 key**——存放几天的旧 key 在 vkey 上表现为组合整组拒绝（cgi 1000，M800 无权
// 连坐 128k）。官方客户端的做法是用登录响应下发的 refresh_token/refresh_key 定期
// 换新 key（等价于免扫码重新登录）。我们在登录时保存这些令牌，启动时自动刷新。

import (
	"errors"
)

// ErrNoRefreshToken 表示凭据不足以刷新（既无 refresh_token 也无 musickey）。
var ErrNoRefreshToken = errors.New("凭据不足以刷新，请重新扫码登录")

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

// RefreshCredential 换取新凭据（免扫码）。
//
// 实测：Login 刷新接口**只凭 musickey + musicid 即可**返回新 key
// （响应带 musickeyCreateTime / keyExpiresIn，当前为 3 天有效期）。
// 有 refresh_token 时一并带上（官方完整形态）；没有也能刷新。
// 成功后调用方应 SetCredential 并持久化。
func (c *Client) RefreshCredential() (*Credential, error) {
	c.mu.Lock()
	cred := c.cred
	c.mu.Unlock()
	if cred.MusicKey == "" || cred.StrMusicID == "" {
		return nil, ErrNotLoggedIn
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
		return nil, err
	}
	fresh, err := credentialFromLoginData(data, cred)
	if err != nil {
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
