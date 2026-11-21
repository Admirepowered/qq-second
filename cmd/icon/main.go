// Command icon 渲染 QQ Second 的品牌图标（托盘 / 窗口 / 可执行文件资源）。
//
//	go run ./cmd/icon                      # 写出 build/appicon.png + build/windows/icon.ico
//	go run ./cmd/icon -preview build/icon  # 同时导出每个尺寸的 PNG 预览
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"qqsecond/internal/tray"
)

func main() {
	var (
		out     = flag.String("out", "build", "output directory")
		preview = flag.String("preview", "", "optional directory for per-size PNG previews")
	)
	flag.Parse()

	if err := os.MkdirAll(filepath.Join(*out, "windows"), 0o755); err != nil {
		fatal(err)
	}

	if err := writePNG(filepath.Join(*out, "appicon.png"), tray.Render(1024)); err != nil {
		fatal(err)
	}
	fmt.Println("wrote", filepath.Join(*out, "appicon.png"))

	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	ico := tray.BuildMultiICO(sizes)
	icoPath := filepath.Join(*out, "windows", "icon.ico")
	if err := os.WriteFile(icoPath, ico, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d bytes, sizes %v)\n", icoPath, len(ico), sizes)

	if *preview == "" {
		return
	}
	if err := os.MkdirAll(*preview, 0o755); err != nil {
		fatal(err)
	}
	for _, size := range sizes {
		p := filepath.Join(*preview, fmt.Sprintf("icon-%d.png", size))
		if err := writePNG(p, tray.Render(size)); err != nil {
			fatal(err)
		}
		fmt.Println("wrote", p)
	}
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	// Render 输出是 premultiplied (image.RGBA)；PNG 存 straight alpha，
	// 先转换，否则半透明边缘会被破坏。
	b := img.Bounds()
	nrgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, b.Min, draw.Src)
	if err := png.Encode(f, nrgba); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
