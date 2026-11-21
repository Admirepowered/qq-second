package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"qqsecond/internal/app/qqmusic"
	"qqsecond/internal/version"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是绑定给前端的唯一结构体。
type App struct {
	ctx       context.Context
	mu        sync.Mutex
	dataDir   string
	storePath string
	store     Store
	forceQuit bool

	qq      *qqmusic.Client
	library *LocalLibrary

	playMu             sync.Mutex
	playIndex          map[string]*playToken // token -> 解析结果
	updateInfo         *UpdateInfo           // 最近一次更新检查结果
	singleInstRelease  func()                // 单实例锁释放
}

// playToken 是一次 QQ 播放解析的结果，前端只拿 token，签名 URL 不出后端。
type playToken struct {
	url       string
	quality   string
	format    string
	sizeBytes int64
	mid       string
	mediaMid  string
	albumMid  string
	triedQ    string
	createdAt time.Time
}

// 代理流客户端（按代理地址缓存，设置变更后自动重建）。
var (
	proxyStreamClient  *http.Client
	proxyStreamFor     string
	proxyStreamClientMu sync.Mutex
)

// NewApp 创建 App 实例（wails.Run 之前调用）。
func NewApp() *App {
	a := &App{
		playIndex: make(map[string]*playToken),
	}
	// 供流代理在签名 URL 过期时刷新 token 对应的 URL。
	refreshStreamToken = a.qqReplayRefresh
	return a
}

// Startup 是 wails 生命周期钩子。
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	dataDir, err := appDataDir()
	if err != nil {
		println("resolve data dir failed:", err.Error())
		dataDir = ".qq-second-data"
	}
	a.dataDir = dataDir
	a.storePath = filepath.Join(dataDir, "store.json")
	for _, dir := range []string{"backgrounds", "covers", "cache"} {
		if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
			println("mkdir", dir, "failed:", err.Error())
		}
	}
	if err := a.initStore(); err != nil {
		println("init store failed:", err.Error())
	}

	a.qq = qqmusic.NewClient()
	// 恢复设备身份（减少限流）
	if idData, err := os.ReadFile(filepath.Join(dataDir, "qq-identity.json")); err == nil {
		a.qq.LoadIdentity(idData)
	}
	a.qq.SetIdentityChangedHook(func() {
		if data := a.qq.SaveIdentity(); len(data) > 0 {
			tmp := filepath.Join(dataDir, "qq-identity.json.tmp")
			if err := os.WriteFile(tmp, data, 0o644); err == nil {
				_ = os.Rename(tmp, filepath.Join(dataDir, "qq-identity.json"))
			}
		}
	})
	if cred := a.store.Credential; cred.QQMusicKey != "" {
		a.qq.SetCredential(qqmusic.Credential{
			MusicID:    parseNum(cred.Uin),
			StrMusicID: cred.Uin,
			MusicKey:   cred.QQMusicKey,
			LoginType:  int(cred.LoginType),
			Nickname:   cred.Nickname,
		})
	}
	a.library = NewLocalLibrary(dataDir, filepath.Join(dataDir, "covers"))

	// 单实例：二次启动唤起已有窗口后退出（避免双实例互踩 store.json）。
	// 必须放在 Startup（绑定生成模式会执行 main() 但不走生命周期钩子）。
	if release, acquired, err := AcquireSingleInstance(dataDir); err != nil {
		println("single instance check failed:", err.Error())
	} else if !acquired {
		_ = SignalRunningInstance(dataDir)
		wailsruntime.Quit(a.ctx)
		return
	} else {
		a.singleInstRelease = release
	}

	// 唤起信号监听（二次启动 → 显示主窗口）
	a.WatchSignalFile()

	// 托盘（Windows 立即可见）
	a.startTray()

	// 后台恢复/扫描本地音乐库，不阻塞启动。
	go func() {
		a.mu.Lock()
		folders := make([]string, 0, len(a.store.Settings.LocalFolders)+1)
	_ = folders
		a.mu.Unlock()
		a.library.LoadCache()
		if len(folders) > 0 {
			a.library.Rescan(folders)
		}
	}()
}

func parseNum(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// DomReady 在前端 DOM 就绪后触发。
func (a *App) DomReady(ctx context.Context) {}

// Shutdown 退出前保存状态、移除托盘、释放单实例锁。
func (a *App) Shutdown(ctx context.Context) {
	a.stopTray()
	if a.singleInstRelease != nil {
		a.singleInstRelease()
		a.singleInstRelease = nil
	}
	a.mu.Lock()
	_ = a.saveStoreLocked()
	a.mu.Unlock()
}

// ---- 窗口控制（无边框窗口的自定义三键 + 托盘） ----

func (a *App) MinimizeWindow() {
	if a.ctx != nil {
		wailsruntime.WindowMinimise(a.ctx)
	}
}

func (a *App) ToggleMaximizeWindow() {
	if a.ctx != nil {
		wailsruntime.WindowToggleMaximise(a.ctx)
	}
}

// ShowWindow 唤起主窗口（托盘双击/菜单）。
func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
}

// CloseWindow 点关闭按钮：按关闭行为设置决定隐藏到托盘还是退出。
func (a *App) CloseWindow() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	behavior := a.store.Settings.CloseBehavior
	a.mu.Unlock()
	if behavior == "exit" {
		a.QuitNow()
		return
	}
	wailsruntime.WindowHide(a.ctx)
}

// QuitNow 真正退出（托盘菜单 / 设置为直接退出时）。
func (a *App) QuitNow() {
	a.forceQuit = true
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
}

// OnBeforeClose 返回 true 拦截关闭（隐藏到托盘）。
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.forceQuit {
		return false
	}
	a.mu.Lock()
	behavior := a.store.Settings.CloseBehavior
	a.mu.Unlock()
	if behavior == "exit" {
		return false
	}
	wailsruntime.WindowHide(ctx)
	return true
}

// GetVersion 返回应用版本号。
func (a *App) GetVersion() string {
	return version.Version
}

// GetDataDirPath 返回数据目录绝对路径。
func (a *App) GetDataDirPath() string {
	return a.dataDir
}

// OpenDataDir 用资源管理器打开数据目录。
func (a *App) OpenDataDir() {
	a.OpenPath(a.dataDir)
}

// OpenLogsDir 用资源管理器打开日志目录。
func (a *App) OpenLogsDir() {
	a.OpenPath(LogsDirPath(a.dataDir))
}

// ---- 会话快照（独立 session.json，见 session.go） ----

// SaveSession 由前端定期保存播放器状态。
func (a *App) SaveSession(session SessionData) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.saveSessionFile(session)
	return nil
}

// GetSession 返回上次退出时的播放器状态。
func (a *App) GetSession() SessionData {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.loadSessionFile()
}

// ---- 凭据 ----

// GetCredential 返回当前登录状态（不含密钥本体）。
func (a *App) GetCredential() Credential {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Credential{
		Uin:      a.store.Credential.Uin,
		Nickname: a.store.Credential.Nickname,
		LoggedAt: a.store.Credential.LoggedAt,
	}
}

func (a *App) saveCredentialLocked(c Credential) {
	a.store.Credential = c
	if a.qq != nil {
		a.qq.SetCredential(qqmusic.Credential{
			MusicID:    parseNum(c.Uin),
			StrMusicID: c.Uin,
			MusicKey:   c.QQMusicKey,
			LoginType:  int(c.LoginType),
			Nickname:   c.Nickname,
		})
	}
	_ = a.saveStoreLocked()
}

// ---- 播放解析（token 表） ----

// QQResolvePlay 解析一首 QQ 曲目的播放源，返回 token 及音质信息。
// 签名 URL 保留在后端；前端通过 /audio/stream?id=<token> 播放。
func (a *App) QQResolvePlay(track Track, preferredQuality string) (PlaybackSource, error) {
	if a.qq == nil {
		return PlaybackSource{}, errors.New("app not ready")
	}
	if preferredQuality == "" {
		a.mu.Lock()
		preferredQuality = a.store.Settings.PreferredQuality
		a.mu.Unlock()
	}
	src, err := a.qq.ResolvePlay(track.Mid, track.MediaMid, track.AlbumMid, preferredQuality)
	if err != nil {
		return PlaybackSource{}, err
	}
	token := randomToken()
	a.playMu.Lock()
	// 清理过期 token，避免无限增长。
	now := time.Now()
	for k, v := range a.playIndex {
		if now.Sub(v.createdAt) > 6*time.Hour {
			delete(a.playIndex, k)
		}
	}
	a.playIndex[token] = &playToken{
		url:       src.URL,
		quality:   src.Quality,
		format:    src.Format,
		sizeBytes: src.SizeBytes,
		mid:       track.Mid,
		mediaMid:  track.MediaMid,
		albumMid:  track.AlbumMid,
		triedQ:    preferredQuality,
		createdAt: now,
	}
	a.playMu.Unlock()
	return PlaybackSource{Token: token, Quality: src.Quality, Format: src.Format, SizeBytes: src.SizeBytes, StreamPath: "/audio/stream?id=" + token}, nil
}

// PlaybackSource 是解析结果。
type PlaybackSource struct {
	Token      string `json:"token"`
	Quality    string `json:"quality"`
	Format     string `json:"format"`
	SizeBytes  int64  `json:"sizeBytes"`
	StreamPath string `json:"streamPath"`
}

// qqReplayRefresh 在流过期（403/410）时用同一个 token 重新解析一次，实现无感续播。
func (a *App) qqReplayRefresh(token string) bool {
	a.playMu.Lock()
	old, ok := a.playIndex[token]
	a.playMu.Unlock()
	if !ok || a.qq == nil {
		return false
	}
	track := Track{Kind: "qq", Mid: old.mid, MediaMid: old.mediaMid, AlbumMid: old.albumMid}
	src, err := a.qq.ResolvePlay(track.Mid, track.MediaMid, track.AlbumMid, old.triedQ)
	if err != nil {
		return false
	}
	a.playMu.Lock()
	old.url = src.URL
	old.quality = src.Quality
	old.format = src.Format
	old.sizeBytes = src.SizeBytes
	old.createdAt = time.Now()
	a.playMu.Unlock()
	return true
}

// ---- 本地音乐绑定 ----

// AddLocalFolder 弹出目录选择框并加入本地音乐文件夹列表，返回完整扫描后的曲库。
func (a *App) AddLocalFolder() (LocalLibraryView, error) {
	if a.ctx == nil {
		return LocalLibraryView{}, errors.New("app not ready")
	}
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择音乐文件夹",
	})
	if err != nil || strings.TrimSpace(dir) == "" {
		return a.LocalLibrary(), err
	}
	return a.AddLocalFolderPath(dir)
}

// AddLocalFolderPath 程序化添加本地音乐文件夹（无对话框）。
func (a *App) AddLocalFolderPath(dir string) (LocalLibraryView, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return a.LocalLibrary(), nil
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return a.LocalLibrary(), fmt.Errorf("目录不存在: %s", dir)
	}
	a.mu.Lock()
	folders := a.store.Settings.LocalFolders
	dup := false
	for _, f := range folders {
		if strings.EqualFold(f, dir) {
			dup = true
			break
		}
	}
	if !dup {
		folders = append(folders, dir)
		a.store.Settings.LocalFolders = folders
		_ = a.saveStoreLocked()
	}
	list := make([]string, 0, len(folders))
	list = append(list, folders...)
	a.mu.Unlock()
	a.library.Rescan(list)
	return a.LocalLibrary(), nil
}

// RemoveLocalFolder 从列表移除文件夹并重扫。
func (a *App) RemoveLocalFolder(dir string) LocalLibraryView {
	a.mu.Lock()
	folders := make([]string, 0, len(a.store.Settings.LocalFolders))
	for _, f := range a.store.Settings.LocalFolders {
		if !strings.EqualFold(f, dir) {
			folders = append(folders, f)
		}
	}
	a.store.Settings.LocalFolders = folders
	_ = a.saveStoreLocked()
	list := make([]string, 0, len(folders))
	list = append(list, folders...)
	a.mu.Unlock()
	a.library.Rescan(list)
	return a.LocalLibrary()
}

// RescanLocalLibrary 强制重扫全部文件夹。
func (a *App) RescanLocalLibrary() LocalLibraryView {
	a.mu.Lock()
	list := make([]string, 0, len(a.store.Settings.LocalFolders))
	list = append(list, a.store.Settings.LocalFolders...)
	a.mu.Unlock()
	a.library.Rescan(list)
	return a.LocalLibrary()
}

// LocalLibraryView 是本地曲库视图。
type LocalLibraryView struct {
	Folders []string     `json:"folders"`
	Tracks  []Track      `json:"tracks"`
	Scanning bool        `json:"scanning"`
}

// LocalLibrary 返回本地曲库（folders 来自设置）。
func (a *App) LocalLibrary() LocalLibraryView {
	a.mu.Lock()
	folders := make([]string, 0, len(a.store.Settings.LocalFolders)+1)
	_ = folders
	a.mu.Unlock()
	return LocalLibraryView{Folders: folders, Tracks: a.library.Tracks()}
}

// ReadLocalCover 返回本地曲目的内嵌封面 data URL。
func (a *App) ReadLocalCover(trackID string) (string, error) {
	return a.library.CoverDataURL(trackID)
}

// ---- 音频流服务（AssetServer Middleware 入口） ----

// ServeAudio 处理 /audio/ 前缀的请求，返回 true 表示已处理。
func (a *App) ServeAudio(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/audio/") {
		return false
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/audio/local/"):
		a.serveLocalAudio(w, r, strings.TrimPrefix(r.URL.Path, "/audio/local/"))
	case strings.HasPrefix(r.URL.Path, "/audio/stream"):
		a.serveStreamAudio(w, r)
	default:
		http.NotFound(w, r)
	}
	return true
}

// serveLocalAudio 按曲目 id 提供本地文件（支持 Range seek）。
func (a *App) serveLocalAudio(w http.ResponseWriter, r *http.Request, trackID string) {
	track, ok := a.library.FindTrack(trackID)
	if !ok {
		http.Error(w, "track not found", http.StatusNotFound)
		return
	}
	f, err := os.Open(track.Path)
	if err != nil {
		http.Error(w, "file open failed", http.StatusNotFound)
		return
	}
	defer f.Close()
	name := filepath.Base(track.Path)
	ct := contentTypeByExt(strings.ToLower(track.Ext))
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, name, time.Now(), f)
}

// serveStreamAudio 供给 QQ 音频：优先磁盘缓存（原生 Range seek），
// 未缓存时边播边落盘 + 代理流。网络模式由设置决定：direct 直连；proxy 走手动代理。
func (a *App) serveStreamAudio(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("id"))
	a.playMu.Lock()
	tok, ok := a.playIndex[token]
	a.playMu.Unlock()
	if !ok || tok.url == "" {
		http.Error(w, "stream not found or expired", http.StatusNotFound)
		return
	}

	// 命中磁盘缓存：本地文件直接供给（ServeContent 原生支持 Range）
	if cached := a.mediaCachePath(tok.mid, tok.format); cached != "" {
		f, err := os.Open(cached)
		if err == nil {
			defer f.Close()
			w.Header().Set("Content-Type", contentTypeByExt(tok.format))
			w.Header().Set("Accept-Ranges", "bytes")
			http.ServeContent(w, r, filepath.Base(cached), time.Now(), f)
			return
		}
	}

	// 未缓存：代理流 + 后台落盘
	a.startMediaDownload(tok.mid, tok.format, tok.url)
	status, refreshable := a.proxyStreamWith(a.streamClient(), w, r, tok.url)
	if refreshable && refreshStreamToken(token) {
		a.playMu.Lock()
		tok2 := a.playIndex[token]
		var newURL string
		if tok2 != nil {
			newURL = tok2.url
		}
		a.playMu.Unlock()
		if newURL != "" {
			// 重新解析后若已缓存完成，切换到缓存供给
			if cached := a.mediaCachePath(tok2.mid, tok2.format); cached != "" {
				f, err := os.Open(cached)
				if err == nil {
					defer f.Close()
					w.Header().Set("Content-Type", contentTypeByExt(tok2.format))
					w.Header().Set("Accept-Ranges", "bytes")
					http.ServeContent(w, r, filepath.Base(cached), time.Now(), f)
					return
				}
			}
			a.proxyStreamWith(a.streamClient(), w, r, newURL)
			return
		}
	}
	_ = status
}

// streamClient 按播放网络设置返回流下载 client。
func (a *App) streamClient() *http.Client {
	a.mu.Lock()
	mode := a.store.Settings.PlaybackNetwork
	proxyStr := strings.TrimSpace(a.store.Settings.UpdateProxy)
	a.mu.Unlock()
	if mode == "proxy" && proxyStr != "" {
		return proxyStreamClientFor(proxyStr)
	}
	return streamHTTPClient
}

// proxyStreamClientFor 构造（并按代理地址缓存）走代理的流下载 client。
func proxyStreamClientFor(proxyStr string) *http.Client {
	proxyStreamClientMu.Lock()
	defer proxyStreamClientMu.Unlock()
	if proxyStreamClient == nil || proxyStreamFor != proxyStr {
		u, err := url.Parse(proxyStr)
		if err != nil {
			return streamHTTPClient
		}
		proxyStreamClient = &http.Client{
			Timeout: 0, // 流式下载不设整体超时
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(u),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 本地代理常做 MITM
				ForceAttemptHTTP2: true,
			},
		}
		proxyStreamFor = proxyStr
	}
	return proxyStreamClient
}

// proxyStreamWith 用指定 client 转发上游响应；返回 (status, 是否为可刷新的过期响应)。
// 注意：调用 refreshable 后不得再写响应（此时响应头尚未提交）。
func (a *App) proxyStreamWith(client *http.Client, w http.ResponseWriter, r *http.Request, url string) (int, bool) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		http.Error(w, "bad upstream", http.StatusBadGateway)
		return 0, false
	}
	// 透传 Range，附上 QQ 域要求的请求头。
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}
	req.Header.Set("Referer", "https://y.qq.com/")
	req.Header.Set("Origin", "https://y.qq.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "upstream failed", http.StatusBadGateway)
		return 0, false
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		// 正常，转发。
	case http.StatusForbidden, http.StatusNotFound, http.StatusGone, http.StatusRequestedRangeNotSatisfiable:
		return resp.StatusCode, true
	default:
		http.Error(w, "upstream error", resp.StatusCode)
		return resp.StatusCode, false
	}

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if w.Header().Get("Accept-Ranges") == "" {
		w.Header().Set("Accept-Ranges", "bytes")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = copyStream(w, resp.Body)
	return resp.StatusCode, false
}

// refreshStreamToken 由 App 提供的包装，避免 proxyStream 直接依赖 App 实例。
var refreshStreamToken func(string) bool

var streamHTTPClient = &http.Client{Timeout: 0} // 流式下载不设整体超时

func copyStream(w http.ResponseWriter, body interface{ Read([]byte) (int, error) }) (int64, error) {
	buf := make([]byte, 64*1024)
	var total int64
	for {
		n, err := body.Read(buf)
		if n > 0 {
			written, werr := w.Write(buf[:n])
			total += int64(written)
			if werr != nil {
				return total, werr
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			return total, nil // io.EOF 或读错误都结束响应
		}
	}
}

func randomToken() string {
	b := make([]byte, 16)
	if _, err := randomRead(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hexEncode(b)
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
