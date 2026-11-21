package app

// 媒体磁盘缓存：QQ 音频流首次播放时边播边落盘，缓存完成后直接由本地文件
// 供给（http.ServeContent 原生 Range seek）。上限 512MB，LRU 清理。

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
)

const (
	mediaCacheLimitBytes = 512 << 20
	mediaCacheDirName    = "media"
)

var (
	mediaDownloading sync.Map // "mid|format" -> struct{}{}（进行中标记）
	mediaDownloadMu  sync.Mutex
)

// mediaCacheDir 返回缓存目录。
func (a *App) mediaCacheDir() string {
	return filepath.Join(a.dataDir, "cache", mediaCacheDirName)
}

// mediaCachePath 返回已完成缓存的文件路径；未缓存返回空串。
func (a *App) mediaCachePath(mid, format string) string {
	if mid == "" {
		return ""
	}
	p := filepath.Join(a.mediaCacheDir(), mid+"."+format)
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
	entries, err := os.ReadDir(a.mediaCacheDir())
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

	dir := a.mediaCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if quality == "" {
		quality = "standard"
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
	dir := a.mediaCacheDir()
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
	if total <= mediaCacheLimitBytes {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mtime < items[j].mtime })
	for _, it := range items {
		if total <= mediaCacheLimitBytes {
			break
		}
		if err := os.Remove(it.path); err == nil {
			total -= it.size
		}
	}
}
