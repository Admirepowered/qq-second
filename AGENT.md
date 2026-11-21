# AGENT.md — 开发者 / AI 助手工作指南

本文件面向在本仓库工作的 AI 助手与新加入的开发者，描述项目约定、关键实现位置与
验证流程。改动前请先阅读，避免破坏已验证的链路。

## 项目概要

QQ Second：Wails v2（Go 1.24 + React 18 + TS + zustand + 纯 CSS）桌面音乐播放器。
两大数据源：QQ 音乐（游客 + 扫码登录）与本地音乐库。平台：Windows（主）、macOS arm64（CI 构建）。

## 目录速览

| 路径 | 职责 |
| --- | --- |
| `main.go` | wails.Run 配置；内存优化（GOGC/GOMEMLIMIT/FreeOSMemory）；`OnBeforeClose` 挂载 |
| `internal/app/app.go` | App struct（唯一绑定）、生命周期、`/audio/local`、`/audio/stream` 代理、播放 token 表 |
| `internal/app/qqmusic/` | QQ 协议客户端（见下） |
| `internal/app/qqmusic/client.go` | 平台 comm（Android/Web）、QIMEI、GetSession、musicu.fcg 请求、限流 2001 自动换设备退避重试 |
| `internal/app/qqmusic/catalog.go` | 搜索 / 首页 / 榜单 / 歌单 / 专辑 / 歌手（含 `<em>` 高亮清洗） |
| `internal/app/qqmusic/play.go` | vkey 清晰流解析（音质降级链）、CDN host 白名单 |
| `internal/app/qqmusic/lyric.go` | QRC 解密（3DES+zlib，带测试向量）、LRC 解析、翻译对齐 |
| `internal/app/qqmusic/account.go` | 账号数据：enc_uin、收藏歌曲（dirid=201）、创建/收藏歌单（宽松解析） |
| `internal/app/qqmusic/auth.go` | QQ 扫码登录（CookieJar 全链路）、hash33/ptqrtoken |
| `internal/app/qqmusic/device.go` | 随机设备指纹、QIMEI 申请（RSA+AES）、多级回退 |
| `internal/app/qqmusic/zzcsign.go` / `tripledes.go` | 签名与解密算法（均有已知向量测试 `sign_test.go`） |
| `internal/app/localmusic.go` | 本地曲库扫描、标签、时长、封面缓存 |
| `internal/app/background.go` | 背景图导入/列表/删除/dataURL（源自 Pulse 移植） |
| `internal/app/settings.go` | Settings 全量结构 + clamp + store.json 持久化 |
| `internal/app/tray.go` | 托盘菜单（内部 `internal/tray` 为零依赖 Win32 实现） |
| `internal/app/update.go` | 更新检查：通道 = 手动代理（仅手动）→ 域名固定 IP SNI → 直连 |
| `internal/app/updater.go` | Windows 一键更新（下载 → 替换 → 重启） |
| `internal/app/autostart_windows.go` | HKCU Run 键自启 |
| `internal/version/version.go` | 当前版本号；**CI 发布成功后自动 bump 小版本** |
| `cmd/icon/` | 生成 `build/appicon.png` + `build/windows/icon.ico`（Q2 字标） |
| `frontend/src/store/` | player（音频引擎/队列）、prefs（外观/设置）、nav（路由+历史栈） |
| `frontend/src/components/` | AppBackground / Sidebar / TopBar / PlayerBar / QueuePanel / LyricsPanel |
| `frontend/src/pages/` | Home / Search / Charts / Detail(Playlist/Album/Artist) / Account / LocalMusic / Settings |

## 硬性约定

1. **播放安全边界**：只用 vkey 明文流（mp3/m4a/flac）。不实现 mflac/加密格式解密，
   不伪造客户端签名绕过权益。错误码 104003 → 前端自动跳下一首（连续 3 次停止）。
2. **签名 URL 不出后端**：前端只拿 `/audio/stream?id=<token>`；token 表在 `App.playIndex`，
   过期(403/410)由 `qqReplayRefresh` 重解析续播。
3. **代理不做自动探测**：只读 `Settings.UpdateProxy`（手动设置）。改回自动探测需用户明确要求。
4. **设置 clamp 在 Go 端 `clampSettings`**：前端 `normalizeSettings` 是镜像。新增设置字段
   两边都要加（Go struct + 前端 types.ts + normalizeSettings + defaultSettings）。
5. **SettingsPage 的 section 必须是模块级组件**：定义在组件体内会因 React 重挂载丢失
   输入框焦点（历史 bug，勿回退）。
6. **毛玻璃范围**：backdrop-filter 仅 sidebar / player-bar 两处（GPU 负载原因）；
   其他面板用 `--card-glass` 半透明底。动画只允许 opacity/color 过渡。
7. **面板底色 alpha**：由 `theme.ts applyThemeTokens` 按 `surfaceOpacity` 写入 CSS 变量
   （背景全窗可见的实现核心），不要在 CSS 里写死面板背景色。
8. **新增 App 绑定方法**后必须 `wails generate module`，并在 `frontend/src/api.ts`
   的解构导出清单里补名字（易漏，历史 bug：GetVersion/OpenReleasePage）。
9. **Git Bash 下不要用命令行内联中文 JSON**（GBK 编码会打坏请求体）——写 UTF-8 文件
   用 `--data-binary @file`。
10. **中文查询返回空 ≠ 接口坏**：先确认是不是新设备频控（cgi 2001）。客户端已带
    退避重试 + 换设备；连续失败时等待几分钟再测，勿盲目改代码。

## 验证清单（改动后逐项走查）

```bash
go build ./... && go vet ./...
cd frontend && npx tsc --noEmit && npm run build
go test ./internal/app/qqmusic/            # zzc 签名 + QRC 解密测试向量
QQ_LIVE=1 go test ./internal/app/qqmusic/ -run TestLiveGuestFlow -v   # 真实链路冒烟
wails dev -browser                          # 浏览器调试（绑定可用）
wails build -platform windows/amd64 -ldflags "-s -w" && upx --best --force build/bin/QQSecond.exe
```

功能走查：搜索→播放；榜单/歌单连播；逐字歌词 + 翻译；本地文件夹→播放→seek；
背景四模式 + 透明度滑杆；深浅色 + 主色；托盘菜单/双击；自启开关往返；更新检查。

## 已知坑

- **频控**：QQ 对新设备/IP 有间歇性 2001 限流（重试内建）。测试勿高频重建 client。
- **WebView2** 辅助进程多个是正常的（browser/GPU/renderer/network），能省的是
  backdrop-filter 数量与动画，不是进程数。
- **assetServer Middleware**：`/audio/*` 在 main.go 里拦截，不经过 dist 静态文件。
- **vite HMR** 会重置 zustand store（含 __player/__prefs 调试句柄），dev 下测试前先 reload。
- `frontend/wailsjs`、`build/bin`、`frontend/dist` 均在 .gitignore，CI 里由 wails 重新生成。

## 发布流程

1. 发版前 `internal/version/version.go` 改成目标版本（CI 只读它）。
2. GitHub Actions → **Build & Release** → 勾选 `release`（可选 `prerelease`）→ 运行。
3. Windows（UPX 强制）+ macOS arm64 双平台产物自动上传 Release。
4. **发布成功后 CI 自动把小版本号 +1 提交回 main**（`chore: bump version ... [skip ci]`）；
   中间版本号变化由维护者手动改。
5. 老版本客户端在 设置→关于 检查更新；Windows 可一键更新（按资产命名约定
   `QQSecond-windows-amd64-<ver>.exe` 下载替换重启），macOS 下载 zip 手动安装。
