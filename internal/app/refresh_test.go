package app

// 自动续期链路测试。
//
// 用户实测：年费账号的播放权益只认「新签发的 musickey + 新 session」，key 存放
// 3 天后对有权曲目也返回 code=1000。这里的测试验证 App 层阶梯：
//   isPrivilegeError 判定正确 → refreshCredential 能真换到新 key → 换完能正常解析。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"qqsecond/internal/app/qqmusic"
)

// newTestApp 造一个只依赖临时数据目录的 App（不碰用户真实配置）。
func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	a := &App{dataDir: dir, storePath: filepath.Join(dir, "store.json")}
	a.qq = qqmusic.NewClient()
	return a
}

func TestIsPrivilegeError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"cgi 1000", &qqmusic.CGIError{Code: 1000}, true},
		{"104003", errString("resolve failed result 104003"), true},
		{"104009", errString("resolve failed result 104009"), true},
		{"no playable", errString("no playable url (may require login)"), true},
		{"nil", nil, false},
		{"网络错误", errString("dial tcp: timeout"), false},
		{"其他 cgi", &qqmusic.CGIError{Code: 2001}, false},
	}
	for _, c := range cases {
		if got := isPrivilegeError(c.err); got != c.want {
			t.Errorf("%s: isPrivilegeError=%v, want %v", c.name, got, c.want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// TestRefreshCredentialWritesStore 验证续期会写回新的 key 与到期提示。
func TestRefreshCredentialWritesStore(t *testing.T) {
	uin, key := loadUserCredential(t)
	a := newTestApp(t)
	a.qq.SetCredential(qqmusic.Credential{
		MusicID:    parseNum(uin),
		StrMusicID: uin,
		MusicKey:   key,
		LoginType:  2,
	})
	a.store.Credential = Credential{Uin: uin, QQMusicKey: key, LoginType: 2, Nickname: "一抹清酒"}

	if !a.refreshCredential() {
		t.Fatal("refreshCredential 失败（本机凭据应可刷新）")
	}

	// 内存态：key 已轮换、到期提示已写入、昵称未丢
	if a.store.Credential.QQMusicKey == key {
		t.Error("musickey 没有轮换")
	}
	if a.store.Credential.ExpireHint == "" {
		t.Error("缺少到期提示")
	}
	if a.store.Credential.Nickname != "一抹清酒" {
		t.Errorf("昵称丢失: %q", a.store.Credential.Nickname)
	}

	// 落盘检查：写到了临时目录，用户真实配置未被修改
	raw, err := os.ReadFile(a.storePath)
	if err != nil {
		t.Fatalf("store 未写入: %v", err)
	}
	var saved Store
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("store 解析失败: %v", err)
	}
	if saved.Credential.QQMusicKey != a.store.Credential.QQMusicKey {
		t.Error("落盘的 key 与内存不一致")
	}
}

// TestRefreshThenResolve 验证续期之后能正常解析播放地址。
func TestRefreshThenResolve(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("set QQ_LIVE=1 to run live test")
	}
	uin, key := loadUserCredential(t)
	a := newTestApp(t)
	a.qq.SetCredential(qqmusic.Credential{
		MusicID: parseNum(uin), StrMusicID: uin, MusicKey: key, LoginType: 2,
	})

	if !a.refreshCredential() {
		t.Fatal("刷新失败")
	}
	// 免费曲（两只老虎）：刷新后用 ladder 解析应成功
	src, err := a.resolveQQStream("000Aphv73a34UV", "000Aphv73a34UV", "", "standard")
	if err != nil {
		t.Fatalf("续期后解析失败: %v", err)
	}
	if src == nil || src.URL == "" {
		t.Fatal("没有返回播放地址")
	}
}

// loadUserCredential 读取本机已登录凭据（只读；测试不写回用户配置）。
func loadUserCredential(t *testing.T) (string, string) {
	t.Helper()
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "AppData", "Roaming", "qq-second", "store.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip("本机没有 store.json")
	}
	var store struct {
		Credential struct {
			Uin        string `json:"uin"`
			QQMusicKey string `json:"qqMusicKey"`
		} `json:"credential"`
	}
	if err := json.Unmarshal(raw, &store); err != nil || store.Credential.QQMusicKey == "" {
		t.Skip("本机没有登录凭据")
	}
	return store.Credential.Uin, store.Credential.QQMusicKey
}
