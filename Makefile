# QQ Second 构建脚本
VERSION ?= $(shell grep -oP 'const Version = "\K[^"]+' internal/version/version.go)

.PHONY: icons build-windows build-linux upx clean

# 生成品牌图标（appicon.png + icon.ico）
icons:
	go run ./cmd/icon

# 本地 Windows 构建（-s -w 瘦身；wails.json 的 postBuildHooks 会自动跑 UPX）
build-windows:
	wails build -clean -platform windows/amd64 -ldflags "-s -w"

# 手动单独压缩（正常构建已自动执行，这里仅备用）
upx:
	@pwsh -NoProfile -File scripts/upx.ps1 build/bin/QQSecond.exe

build-linux:
	wails build -clean -platform linux/amd64 -tags webkit2_41 -ldflags "-s -w"

clean:
	rm -rf build/bin dist
