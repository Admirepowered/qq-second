package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeExe 生成一个最小的"可执行文件"（MZ 头 + 足够大的填充）。
func fakeExe(t *testing.T, dir, name string, marker byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	data := make([]byte, 1<<20)
	data[0] = 'M'
	data[1] = 'Z'
	data[len(data)-1] = marker
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallUpdateSameVolume(t *testing.T) {
	// 两个目录分开，模拟"下载目录"与"程序目录"（同盘时也要走 复制→改名 流程）
	downloadDir := t.TempDir()
	installDir := t.TempDir()

	downloaded := fakeExe(t, downloadDir, "QQSecond-new.exe", 'N')
	installed := fakeExe(t, installDir, "QQSecond.exe", 'O')

	if err := installUpdate(installed, downloaded); err != nil {
		t.Fatalf("installUpdate: %v", err)
	}

	// 主程序应变成新内容
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1] != 'N' {
		t.Fatalf("主程序没有被替换（末尾标记 %q）", got[len(got)-1])
	}

	// 旧程序应保留为 .old.<ts>
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatal(err)
	}
	foundOld := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "QQSecond.exe.old.") {
			foundOld = true
			old, err := os.ReadFile(filepath.Join(installDir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if old[len(old)-1] != 'O' {
				t.Fatalf("旧程序备份内容不对（末尾 %q）", old[len(old)-1])
			}
		}
	}
	if !foundOld {
		t.Fatal("没有找到 .old 备份")
	}

	// 暂存文件不应残留
	if _, err := os.Stat(filepath.Join(installDir, ".qqsecond-update.tmp")); !os.IsNotExist(err) {
		t.Fatal("暂存文件没有清理")
	}
}

func TestSanityCheckExe(t *testing.T) {
	dir := t.TempDir()

	// 太小
	small := filepath.Join(dir, "small.exe")
	if err := os.WriteFile(small, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := sanityCheckExe(small); err == nil {
		t.Fatal("应该拒绝过小的文件")
	}

	// 不是 MZ 头
	html := filepath.Join(dir, "page.exe")
	data := make([]byte, 1<<20)
	copy(data, "<html>")
	if err := os.WriteFile(html, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := sanityCheckExe(html); err == nil {
		t.Fatal("应该拒绝非 PE 文件")
	}

	// 正常
	good := fakeExe(t, dir, "good.exe", 'X')
	if err := sanityCheckExe(good); err != nil {
		t.Fatalf("正常文件被拒绝: %v", err)
	}
}

// TestCrossVolumeInstall 真实跨盘场景：下载目录在 C:（t.TempDir），程序目录在另一个盘。
// 需要显式设置 CROSS_VOLUME_DIR（例如 D:\Project\qq-second\tmp\xvol）才会执行。
func TestCrossVolumeInstall(t *testing.T) {
	target := os.Getenv("CROSS_VOLUME_DIR")
	if target == "" {
		t.Skip("set CROSS_VOLUME_DIR to a directory on another volume to run")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	downloaded := fakeExe(t, t.TempDir(), "QQSecond-new.exe", 'N')
	installed := fakeExe(t, target, "QQSecond.exe", 'O')

	if err := installUpdate(installed, downloaded); err != nil {
		t.Fatalf("跨盘 installUpdate 失败: %v", err)
	}
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if got[len(got)-1] != 'N' {
		t.Fatal("跨盘更新后主程序内容不对")
	}
	// 清理
	entries, _ := os.ReadDir(target)
	for _, e := range entries {
		_ = os.Remove(filepath.Join(target, e.Name()))
	}
}
