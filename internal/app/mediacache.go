package app

// 媒体磁盘缓存：QQ 音频流首次播放时边播边落盘，缓存完成后直接由本地文件
// 供给（http.ServeContent 原生 Range seek）。
// 上限与目录可在设置中调整（上限最大 10GB，默认 512MB；目录切换自动迁移文件）。

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const mediaCacheDirName = "media"

var mediaDownloading sync.Map // "mid|quality|format" -> struct{}{}（进行中标记）

// currentCacheDir 返回当前媒体缓存目录（设置里自定义了就用自定义的）。
func (a *App) currentCacheDir() string {
	a.mu.Lock()
	custom := strings.TrimSpace(a.store.Settings.CacheDir)
	a.mu.Unlock()
	if custom != "" {
		return custom
	}
	return filepath.Join(a.dataDir, "cache", mediaCacheDirName)
}

// mediaCacheLimitBytes 返回缓存上限（字节）。
func (a *App) mediaCacheLimitBytes() int64 {
	a.mu.Lock()
	mb := a.store.Settings.CacheLimitMB
	a.mu.Unlock()
	if mb <= 0 {
		mb = 512
	}
	return int64(mb) << 20
}

// mediaCachePath 返回已完成缓存的文件路径；未缓存返回空串。
func (a *App) mediaCachePath(mid, format string) string {
	if mid == "" {
		return ""
	}
	p := filepath.Join(a.currentCacheDir(), mid+"."+format)
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		return p
	}
	return ""
}

// mediaCacheFind 返回该 mid 的任一缓存文件名（旧命名 mid.ext 或新命名 mid-质量.ext）。
func (a *App) mediaCacheFind(mid string) string {
	if mid == "" {
		return ""
	}
	entries, err := os.ReadDir(a.currentCacheDir())
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, mid+"-") || strings.HasPrefix(name, mid+".") {
			if st, err := e.Info(); err == nil && st.Size() > 0 {
				return name
			}
		}
	}
	return ""
}

// qualityFromCacheName 从缓存文件名推断音质标记。
func qualityFromCacheName(name string) string {
	switch {
	case strings.Contains(name, "-lossless"), strings.HasSuffix(name, ".flac"):
		return "lossless"
	case strings.Contains(name, "-high"):
		return "high"
	case strings.Contains(name, "-standard"), strings.HasSuffix(name, ".m4a"):
		return "standard"
	}
	return ""
}

// CachedSong 缓存歌曲列表项。
type CachedSong struct {
	FileName  string `json:"fileName"`
	Mid       string `json:"mid"`
	Quality   string `json:"quality"`
	Format    string `json:"format"`
	SizeBytes int64  `json:"sizeBytes"`
	ModTime   int64  `json:"modTime"`
}

// ListCachedSongs 列出全部缓存歌曲。
func (a *App) ListCachedSongs() ([]CachedSong, error) {
	out := []CachedSong{}
	entries, err := os.ReadDir(a.currentCacheDir())
	if err != nil {
		return out, nil // 目录不存在 = 空缓存
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".part") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		name := e.Name()
		base := strings.TrimSuffix(name, filepath.Ext(name))
		mid := base
		quality := qualityFromCacheName(name)
		if i := strings.LastIndex(base, "-"); i > 0 {
			mid = base[:i]
			quality = base[i+1:]
		}
		out = append(out, CachedSong{
			FileName:  name,
			Mid:       mid,
			Quality:   quality,
			Format:    strings.TrimPrefix(filepath.Ext(name), "."),
			SizeBytes: info.Size(),
			ModTime:   info.ModTime().Unix(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime > out[j].ModTime })
	return out, nil
}

// ChangeCacheDir 弹出目录选择框更改缓存目录，并把已有缓存文件迁移过去。
func (a *App) ChangeCacheDir() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not ready")
	}
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择媒体缓存目录",
	})
	if err != nil || strings.TrimSpace(dir) == "" {
		return a.currentCacheDir(), nil // 用户取消：保持原样
	}
	return a.SetCacheDir(dir)
}

// SetCacheDir 设置缓存目录并迁移已有缓存文件。
func (a *App) SetCacheDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	a.mu.Lock()
	old := a.currentCacheDirLocked()
	same := strings.EqualFold(old, dir)
	a.store.Settings.CacheDir = dir
	_ = a.saveStoreLocked()
	a.mu.Unlock()

	if !same {
		a.migrateCacheFiles(old)
	}
	return dir, nil
}

// currentCacheDirLocked 在已持有 a.mu 时读取缓存目录。
func (a *App) currentCacheDirLocked() string {
	custom := strings.TrimSpace(a.store.Settings.CacheDir)
	if custom != "" {
		return custom
	}
	return filepath.Join(a.dataDir, "cache", mediaCacheDirName)
}

// migrateCacheFiles 把旧目录下的缓存文件移动到新目录（跨盘自动降级为复制）。
func (a *App) migrateCacheFiles(oldDir string) {
	if oldDir == "" {
		return
	}
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return
	}
	newDir := a.currentCacheDir()
	_ = os.MkdirAll(newDir, 0o755)
	moved := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(oldDir, e.Name())
		dst := filepath.Join(newDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			// 跨盘 rename 失败 → 复制 + 删除
			if in, err2 := os.Open(src); err2 == nil {
				if out, err3 := os.Create(dst); err3 == nil {
					_, _ = io.Copy(out, in)
					_ = out.Close()
					_ = in.Close()
					_ = os.Remove(src)
				}
			}
		}
		moved++
	}
	logsWrite(fmt.Sprintf("cache migrated: %d files %s -> %s", moved, oldDir, newDir))
}

// startMediaDownload 后台全量下载到缓存（去重：同曲目进行中不重复下载）。
func (a *App) startMediaDownload(mid, quality, format, url string) {
	key := mid + "|" + quality + "|" + format
	if _, loading := mediaDownloading.Load(key); loading {
		return
	}
	mediaDownloading.Store(key, struct{}{})
	go func() {
		defer mediaDownloading.Delete(key)
		if err := a.downloadMediaFile(mid, quality, format, url); err != nil {
			println("media cache download failed:", err.Error())
		}
	}()
}

// downloadMediaFile 全量下载音频到缓存目录（.part → 原子改名）。
func (a *App) downloadMediaFile(mid, quality, format, url string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", "https://y.qq.com/")
	req.Header.Set("Origin", "https://y.qq.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := streamHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("cache download http %d", resp.StatusCode)
	}

	if quality == "" {
		quality = "standard"
	}
	dir := a.currentCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := filepath.Join(dir, mid+"-"+quality+"."+format)
	part := final + ".part"
	out, err := os.Create(part)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		_ = os.Remove(part)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Rename(part, final)
	a.trimMediaCache()
	return nil
}

// trimMediaCache 清理缓存：删除超过 1 小时的 .part 残留；总量超限时按 mtime LRU 删除。
func (a *App) trimMediaCache() {
	dir := a.currentCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type item struct {
		path  string
		size  int64
		mtime int64
	}
	var items []item
	var total int64
	now := time.Now().Unix()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if strings.HasSuffix(e.Name(), ".part") && now-info.ModTime().Unix() > 3600 {
			_ = os.Remove(filepath.Join(dir, e.Name()))
			continue
		}
		items = append(items, item{filepath.Join(dir, e.Name()), info.Size(), info.ModTime().UnixNano()})
		total += info.Size()
	}
	limit := a.mediaCacheLimitBytes()
	if total <= limit {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mtime < items[j].mtime })
	for _, it := range items {
		if total <= limit {
			break
		}
		if err := os.Remove(it.path); err == nil {
			total -= it.size
		}
	}
}
