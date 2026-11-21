package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// BackgroundImage 是背景图列表项。
type BackgroundImage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SelectBackgroundImage 弹出文件选择框并把选中的图片导入后台目录，返回文件 id。
func (a *App) SelectBackgroundImage() (string, error) {
	if a.ctx == nil {
		return "", errors.New("app not ready")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择背景图片",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "图片", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.bmp;*.gif"},
		},
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return path, err
	}
	return a.ImportBackgroundImage(path)
}

// ImportBackgroundImage 把指定路径的图片拷贝到受管目录，返回文件 id。
func (a *App) ImportBackgroundImage(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	data, contentType, err := readBackgroundImage(path)
	if err != nil {
		return "", err
	}
	exts, _ := mime.ExtensionsByType(contentType)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" && len(exts) > 0 {
		ext = exts[0]
	}
	if ext == "" {
		ext = ".img"
	}
	id := sanitizeFilename(fmt.Sprintf("%d%s", time.Now().UnixNano(), ext))
	target := filepath.Join(a.dataDir, "backgrounds", id)
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return filepath.Base(target), nil
}

// ListBackgroundImages 返回受管背景图列表（按导入时间排序）。
func (a *App) ListBackgroundImages() ([]BackgroundImage, error) {
	entries, err := os.ReadDir(filepath.Join(a.dataDir, "backgrounds"))
	if err != nil {
		return nil, err
	}
	type item struct {
		entry os.DirEntry
		mod   time.Time
	}
	items := make([]item, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, item{entry: entry, mod: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.Before(items[j].mod) })
	backgrounds := make([]BackgroundImage, 0, len(items))
	for _, item := range items {
		backgrounds = append(backgrounds, BackgroundImage{ID: item.entry.Name(), Name: fmt.Sprintf("背景%d", len(backgrounds)+1)})
	}
	return backgrounds, nil
}

// DeleteBackgroundImage 删除背景图；若删除的是当前选中项则清空设置。
func (a *App) DeleteBackgroundImage(id string) error {
	id = sanitizeFilename(strings.TrimSpace(id))
	if id == "" {
		return nil
	}
	if err := os.Remove(filepath.Join(a.dataDir, "backgrounds", id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	a.mu.Lock()
	if a.store.Settings.BackgroundImage == id {
		a.store.Settings.BackgroundImage = ""
		if a.store.Settings.BackgroundMode == "image" {
			a.store.Settings.BackgroundMode = "default"
		}
		err := a.saveStoreLocked()
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()
	return nil
}

// ReadBackgroundImageDataURL 读取背景图并转为 data URL（前端直接用于 background-image）。
func (a *App) ReadBackgroundImageDataURL(id string) (string, error) {
	id = sanitizeFilename(strings.TrimSpace(id))
	if id == "" {
		return "", nil
	}
	data, contentType, err := readBackgroundImage(filepath.Join(a.dataDir, "backgrounds", id))
	if err != nil {
		return "", err
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func readBackgroundImage(path string) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		return nil, "", errors.New("background image path is a directory")
	}
	if info.Size() > 20*1024*1024 {
		return nil, "", errors.New("背景图片必须小于 20MB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", errors.New("所选文件不是图片")
	}
	return data, contentType, nil
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return strings.TrimSpace(replacer.Replace(name))
}
