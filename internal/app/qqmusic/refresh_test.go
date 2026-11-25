package qqmusic

import (
	"testing"
)

// 用模拟的 Login 响应验证 credentialFromLoginData 的字段映射
// （蛇形/驼峰兼容、到期时间计算、令牌沿旧值）。
func TestCredentialFromLoginData(t *testing.T) {
	prev := Credential{
		MusicID:    1204844044,
		StrMusicID: "1204844044",
		MusicKey:   "OLDKEY",
		LoginType:  2,
		OpenID:     "OPENID_OLD",
		RefreshToken: "RT_OLD",
		AccessToken:  "AT_OLD",
		RefreshKey:   "RK_OLD",
		MusickeyCreateTime: 1732000000000,
		KeyExpiresIn:       7776000,
	}

	cases := []struct {
		name string
		data map[string]interface{}
	}{
		{
			"蛇形字段（完整下发）",
			map[string]interface{}{
				"musicid": 1204844044.0, "musickey": "NEWKEY1",
				"str_musicid": "1204844044",
				"openid": "OPENID_NEW", "refresh_token": "RT_NEW",
				"access_token": "AT_NEW", "refresh_key": "RK_NEW",
				"musickeyCreateTime": 1732000000.0, "keyExpiresIn": 7776000.0,
			},
		},
		{
			"驼峰字段（只换 key，不下发令牌）",
			map[string]interface{}{
				"musicid": 1204844044.0, "musicKey": "NEWKEY2",
				"uin": "1204844044",
			},
		},
	}

	for _, tc := range cases {
		fresh, err := credentialFromLoginData(tc.data, prev)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if fresh.StrMusicID != "1204844044" || fresh.MusicID != 1204844044 {
			t.Fatalf("%s: 身份字段错误: %+v", tc.name, fresh)
		}
		switch tc.name {
		case cases[0].name:
			if fresh.MusicKey != "NEWKEY1" {
				t.Fatalf("key 未更新: %s", fresh.MusicKey)
			}
			if fresh.OpenID != "OPENID_NEW" || fresh.RefreshToken != "RT_NEW" ||
				fresh.AccessToken != "AT_NEW" || fresh.RefreshKey != "RK_NEW" {
				t.Fatalf("刷新令牌未采用新值: %+v", fresh)
			}
			if fresh.ExpiredAt == 0 {
				t.Fatal("到期时间未计算")
			}
		case cases[1].name:
			if fresh.MusicKey != "NEWKEY2" {
				t.Fatalf("key 未更新: %s", fresh.MusicKey)
			}
			if fresh.OpenID != "OPENID_OLD" || fresh.RefreshToken != "RT_OLD" {
				t.Fatalf("令牌未沿用旧值: %+v", fresh)
			}
			// 响应没下发 create/expires 时沿用旧值重算到期时间
			want := (prev.MusickeyCreateTime + prev.KeyExpiresIn*1000) / 1000
			if fresh.ExpiredAt != want {
				t.Fatalf("到期时间应 = %d, got %d", want, fresh.ExpiredAt)
			}
		}
	}
}

// 刷新响应不带 nick 时必须沿用旧昵称（否则界面显示一串 uin）。
func TestCredentialFromLoginDataKeepsNickname(t *testing.T) {
	prev := Credential{
		MusicID: 1204844044, StrMusicID: "1204844044", MusicKey: "OLD", LoginType: 2,
		Nickname: "一抹清酒",
	}
	fresh, err := credentialFromLoginData(map[string]interface{}{
		"musicid": 1204844044.0, "musickey": "NEW", "str_musicid": "1204844044",
	}, prev)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Nickname != "一抹清酒" {
		t.Fatalf("昵称丢失: %q", fresh.Nickname)
	}
}

func TestCredentialRefreshable(t *testing.T) {
	if (Credential{}).Refreshable() {
		t.Fatal("空凭据不应可刷新")
	}
	if (Credential{MusicKey: "k", OpenID: "o"}).Refreshable() {
		t.Fatal("缺 refresh_token 不应可刷新")
	}
	if !(Credential{MusicKey: "k", OpenID: "o", RefreshToken: "r"}).Refreshable() {
		t.Fatal("齐全令牌应可刷新")
	}
}
