# QQ Second 构建脚本
VERSION ?= $(shell grep -oP 'const Version = "\K[^"]+' internal/version/version.go)

.PHONY: icons build-windows build-linux upx clean

# 生成品牌图标（appicon.png + icon.ico）
icons:
	go run ./cmd/icon

# 本地 Windows 构建（-s -w 瘦身；UPX 需自行安装）
build-windows:
	wails build -clean -platform windows/amd64 -ldflags "-s -w"

# UPX 压缩（可选；未安装 upx 时跳过）
upx:
	@upx --best --force build/bin/QQSecond.exe 2>/dev/null || echo "upx 未安装，跳过压缩"

build-linux:
	wails build -clean -platform linux/amd64 -tags webkit2_41 -ldflags "-s -w"

clean:
	rm -rf build/bin dist
