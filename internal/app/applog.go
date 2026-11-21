package app

// 应用日志：每次启动删除重建，实时写入（文件写无缓冲，逐条落盘）。
// 捕获范围：Go 侧 log/fmt 输出（stdout+stderr 重定向）+ 前端经 SendLog 上报的错误。

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	logMu     sync.Mutex
	logFile   *os.File
	logsDir   string
	logsDirOK bool
)

// InitLogger 初始化日志：清空 logs 目录旧文件，重建 app-<日期>.log。
// 必须在 wails.Run 之前调用（捕获 wails 内部日志）。
func InitLogger(dataDir string) {
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	logsDir = dir

	// 每次启动删除旧日志
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}

	name := fmt.Sprintf("app-%s.log", time.Now().Format("2006-01-02-150405"))
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	logFile = f
	logsDirOK = true

	// log 包 → 文件（wails 内部日志走 log 包 → stderr）
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	// fmt.Println → os.Stdout 重定向到文件
	os.Stdout = f
	logsWrite("logger ready: " + name)
}

// logsWrite 追加一行（文件写无缓冲 = 实时落盘）。
func logsWrite(line string) {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Fprintf(logFile, "[%s] %s\n", ts, strings.TrimRight(line, "\n"))
}

// LogsDirPath 返回日志目录（未初始化时返回 dataDir/logs）。
func LogsDirPath(dataDir string) string {
	if logsDirOK {
		return logsDir
	}
	return filepath.Join(dataDir, "logs")
}

// SendLog 前端错误上报（window.onerror / ErrorBoundary 调用）。
func (a *App) SendLog(level, message string) {
	logsWrite("[frontend/" + strings.ToUpper(level) + "] " + message)
}

// OpenPath 用系统资源管理器打开指定目录（文件夹存在时）。
func (a *App) OpenPath(dir string) {
	if a.ctx == nil {
		return
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		_ = os.MkdirAll(dir, 0o755)
	}
	explorer := explorerOpen(dir)
	if explorer != nil {
		_ = explorer.Start()
	}
}
