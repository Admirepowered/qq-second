# AGENT.md — 开发者 / AI 助手工作指南

本文件面向在本仓库工作的 AI 助手与新加入的开发者，描述项目约定、关键实现位置与
验证流程。**本文件必须随功能与坑点变化同步更新**（新增约定/踩坑/架构变化时，在对应
小节补条目并随同一次 commit 提交）。改动前请先阅读，避免破坏已验证的链路。

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
| `internal/app/mediacache.go` | QQ 音频磁盘缓存（边播边落盘，512MB LRU；命中后本地 Range 供给） |
| `internal/app/qqmusic/favor.go` | 歌曲收藏（music.musicfavor.SongFavor Add/DelFavSong，需数字 songId + 登录）；`AddSongToPlaylist` 加入自建歌单（dirId + tid） |
| `internal/app/recognize/` | 听歌识曲：Shazam 指纹签名（2048 点 FFT + 峰值扩散/打包，含单测）+ 识别请求（复用设置里的手动代理） |
| `internal/app/identify.go` | `IdentifyAudio` 绑定：PCM 重采样 → 识别 → 用「标题+歌手」映射 QQ 曲库 |
| `internal/app/background.go` | 背景图导入/列表/删除/dataURL（源自 Pulse 移植） |
| `internal/app/settings.go` | Settings 全量结构 + clamp + store.json 持久化 |
| `internal/app/session.go` | 播放会话快照（独立 session.json，与设置分开写） |
| `internal/app/singleinst_*.go` | 单实例锁（命名互斥量 + 唤起信号文件；dev 构建用独立名字） |
| `internal/app/updater.go` | Windows 一键更新（下载资产 → 替换 → 重启） |
| `internal/app/tray.go` | 托盘菜单（内部 `internal/tray` 为零依赖 Win32 实现） |
| `internal/app/update.go` | 更新检查：通道 = 手动代理（仅手动）→ 域名固定 IP SNI → 直连 |
| `internal/app/updater.go` | Windows 一键更新（下载 → 替换 → 重启） |
| `internal/app/autostart_windows.go` | HKCU Run 键自启 |
| `internal/version/version.go` | 当前版本号；**CI 发布成功后自动 bump 小版本** |
| `cmd/icon/` | 生成 `build/appicon.png` + `build/windows/icon.ico`（Q2 字标） |
| `frontend/src/store/` | player（音频引擎/队列）、prefs（外观/设置）、nav（路由+历史栈）、fav（我喜欢+歌单缓存）、toast（操作提示） |
| `frontend/src/components/` | AppBackground / Sidebar / TopBar / PlayerBar / QueuePanel / LyricsPanel / ContextMenu / ToastHost |
| `frontend/src/pages/` | Home / Search / Charts / Detail(Playlist/Album/Artist) / Account / LocalMusic / CachedSongs / Identify / Settings |
| `frontend/src/recorder.ts` | 麦克风录音 / 音频文件解码 → s16le PCM（识曲输入） |

## 硬性约定

1. **播放安全边界**：只用 vkey 明文流（mp3/m4a/flac）。不实现 mflac/加密格式解密，
   不伪造客户端签名绕过权益。错误码 104003 → 前端自动跳下一首（连续 3 次停止）。
2. **签名 URL 不出后端**：前端只拿 `/audio/stream?id=<token>`；token 表在 `App.playIndex`，
   过期(403/410)由 `qqReplayRefresh` 重解析续播。
3. **代理不做自动探测**：只读 `Settings.UpdateProxy`（手动设置）。改回自动探测需用户明确要求。
4. **设置 clamp 在 Go 端 `clampSettings`**：前端 `normalizeSettings` 是镜像。新增设置字段
   两边都要加（Go struct + 前端 types.ts + normalizeSettings + defaultSettings）。
5. **SettingsPage 的 section 必须是模块级组件**：定义在组件体内会因 React 重挂载丢失
   输入框焦点（历史 bug：代理地址输入框无法聚焦，勿回退）。
6. **不允许 backdrop-filter（毛玻璃）**：用户已明确去掉全部模糊效果，背景图要原样清晰
   显示。面板只允许半透明底色（alpha 由滑杆驱动）+ 文字 text-shadow 保证可读。
7. **面板底色 alpha**：由 `theme.ts applyThemeTokens` 按 `surfaceOpacity` 写入 CSS 变量
   （背景全窗可见的实现核心，系数<1），不要在 CSS 里写死面板背景色；所有组件统一受滑杆影响。
8. **新增 App 绑定方法**后必须 `wails generate module`，并在 `frontend/src/api.ts`
   的解构导出清单里补名字（易漏，历史 bug：GetVersion/OpenReleasePage）。
9. **Git Bash 下不要用命令行内联中文 JSON**（GBK 编码会打坏请求体）——写 UTF-8 文件
   用 `--data-binary @file`。
10. **中文查询返回空 ≠ 接口坏**：先确认是不是新设备频控（cgi 2001）。客户端已带
    退避重试 + 换设备；连续失败时等待几分钟再测，勿盲目改代码。
11. **会话与设置分文件写**：播放会话在 `session.json`、设置/凭据在 `store.json`。
    历史坑：两者曾共写 store.json，播放中每 2 秒的会话快照会把并发修改的设置
    （如面板透明度）整体覆盖回旧值——表现为"透明度没保存"。新持久化数据一律
    单独建文件，不要塞进 store.json。
12. **单实例**：`AcquireSingleInstance`（互斥量）+ `show.signal` 唤起文件；二次启动
    唤起已有窗口后退出。dev 构建用独立互斥量/信号名（`-tags dev`），可与正式版共存。
13. **本文件随更新同步维护**：功能变化、新坑、新约定都要回写到对应小节。
14. **Go nil 切片序列化为 null**：返回前端的 slice 字段必须保证非 nil（空时给
    `[]Track{}` 等），否则前端 `xxx.length` 直接崩（历史 bug：全新设备空曲库
    folders=null → 本地音乐页崩溃）。新增接口/字段时逐个确认。
15. **测试禁止动用户真实数据**：调 QQLogout 等会清凭据/覆盖设置的操作只能在
    测试账号上做；在用户主环境验证时禁止清空其登录态。
15b. **测试进程纪律**：功能验证一律用 dev 浏览器页（不占桌面）；仅验证原生窗口
    行为（托盘/拖动/单实例）时短暂启动正式版并立即关闭；每次收尾清理全部相关
    进程（QQSecond-dev.exe / 5173 端口 node / QQSecond.exe），不留后台实例。
16. **版本号迭代规则**：`major.minor.patch`。patch 由 AI 每次对话开始时 +1；
    minor 由维护者升级（升级后 patch 归 1，因为是修补 commit 计数）；major 同理。
    CI 不做任何 bump。
17. **媒体磁盘缓存**：已缓存曲目在 `QQResolvePlay` 直接返回 `/audio/cache/<文件>`
    跳过 vkey 解析；缓存文件名 `mid-<quality>.<ext>`；供给走严格文件名校验的
    `serveCacheAudio`。
18. **QRC 字段语义**：`[行开始ms,行时长ms]词(词开始ms,词时长ms)…`——**两个第二个数字
    都是时长**，词时间是相对整首歌的绝对时间。历史实现把行第二值当结束时间、把词括号
    内第一个数字当时长×10 累加，导致逐字填充整体错位（表现：当前句只亮开头很短一段）。
    解析用 `qrcLineHeader` / `qrcWordTag` 两个正则，正文里的字面括号（如 `Moon (I)`）
    不会误判。
19. **歌词整句高亮**：逐字渐变只在「句首词开始 → 句末词结束」区间启用，之外整句直接
    渲染纯文本（`.lyrics-line.current` 为白色），保证一句话从头到尾都是亮的。
20. **听歌识曲的网络**：Shazam 接口在国内常直连超时，识别请求复用
    `Settings.UpdateProxy`（手动代理）——与更新检查一致，**不做自动探测**。
    签名算法移植自公开实现，改动后跑 `go test ./internal/app/recognize/`。
21. **WebView2 麦克风**：`getUserMedia` 在 Wails 的 WebView2 里实测可直接用（无需
    `--use-fake-ui-for-media-stream`，该开关反而会让 WebView2 环境创建失败/窗口不显示，
    已移除）。若某些机器权限被拒，识别页的"选择音频文件"是兜底路径。
22. **操作反馈统一走 toast**：`store/toast` + `components/ToastHost`（收藏到歌单、
    识曲失败等）；不要在页面里各写一套提示。

## 验证清单（改动后逐项走查）

```bash
go build ./... && go vet ./...
# 非 Windows 平台交叉编译检查（wails build 时绑定生成会执行 main()，
# 平台差异声明冲突必须在本地抓掉，别等 CI 报错——历史坑：signalFile 重复声明）
GOOS=darwin go build ./internal/... ./cmd/...
GOOS=darwin go build -tags dev ./internal/...
GOOS=linux go build ./internal/... ./cmd/...
cd frontend && npx tsc --noEmit && npm run build
go test ./internal/app/qqmusic/            # zzc 签名 + QRC 解密/解析测试向量
go test ./internal/app/recognize/          # 识曲签名结构（magic/crc/TLV）+ 重采样
QQ_LIVE=1 go test ./internal/app/qqmusic/ -run TestLiveGuestFlow -v   # 真实链路冒烟
wails dev -browser                          # 浏览器调试（绑定可用）
wails build -platform windows/amd64 -ldflags "-s -w" && upx --best --force build/bin/QQSecond.exe
```

功能走查：搜索→播放；榜单/歌单连播；逐字歌词 + 翻译（整句高亮、行间不早退）；本地文件夹→播放→seek；
背景四模式 + 透明度滑杆；深浅色 + 主色；托盘菜单/双击；自启开关往返；更新检查；
右键「收藏到歌单…」→ 选「我喜欢」/自建歌单 → 心形与歌单同步；听歌识曲（录音 8s / 选音频文件）。

## 已知坑

- **频控**：QQ 对新设备/IP 有间歇性 2001 限流（重试内建）。测试勿高频重建 client。
- **WebView2** 辅助进程多个是正常的（browser/GPU/renderer/network），能省的是
  GPU 合成负担（已去掉全部 backdrop-filter 与位移动画），不是进程数。
- **assetServer Middleware**：`/audio/*` 在 main.go 里拦截，不经过 dist 静态文件。
- **vite HMR** 会重置 zustand store（含 __player/__prefs 调试句柄），dev 下测试前先 reload。
- **歌词活动行守卫不能用 startMs 真值判断**：首行（元数据行）startMs 常为 0，
  ```!lines[0].startMs``` 会把整个歌词同步判废。用 syncMode 判断。
  歌词全屏层需设置 ```--wails-draggable: drag```（交互区覆盖 no-drag），否则无边框窗口拖不动。
- **QRC 的第二个数字是时长**：见硬性约定 18。历史 bug：词 EndMs 小于 StartMs，
  逐字填充只推进到开头一点点，用户观感是"这一句只亮 1 秒"。
- **Shazam 直连超时**：`dial tcp ...:443` 超时通常是网络被墙，不是代码问题；
  设置里填手动代理（如 `http://127.0.0.1:7890`）后识曲即可用（已内建 3 次重试）。
- **麦克风录不到声音**：先确认识别页的提示文案有没有出现"正在聆听… N%"；前端会把
  录音/识别结果写进应用日志（`identify:` 前缀），失败原因可直接看日志。
- **启动后看不到窗口（进程和前端日志都正常）**：WebView2 首帧就绪回调偶发丢失，
  Wails 不会执行"显示窗口"。已在 `Startup` 里加了 8 次幂等的
  `WindowUnminimise/WindowShow` 兜底（每 0.5s 一次），不要删。
- **WebView2 进程未退出时立刻重启会卡住**：手工测试时杀进程后等 10s 以上再启动，
  否则新实例的 WebView2 环境会创建失败（窗口不出现）。
- **Startup 阶段 wailsruntime.Quit 不生效**：事件循环未就绪。二次启动的实例
  在 Startup 检测到已有实例后必须用 os.Exit(0) 直接退出（此时未写任何数据，安全）。
- **双实例共写 store.json**：正式版与 dev 同时跑时，旧实例的任何一次保存都会把
  另一实例刚写的设置覆盖回去（透明度"没保存"的根因）。已用单实例锁 + session.json
  分文件修复；dev 构建用独立锁名可与正式版共存。
- **CI 上 wails build 会执行 main() 生成绑定**：单实例/启动副作用必须 fail-open。
  历史坑：CI 用户目录不存在 → 锁文件创建失败 → main 提前 return → 绑定没生成 →
  macOS/Linux 构建失败。修复：`CI=true` 跳过单实例 + 锁目录 MkdirAll + 错误放行。
- `frontend/wailsjs`、`build/bin`、`frontend/dist` 均在 .gitignore，CI 里由 wails 重新生成。

## 发布流程

1. 发版前**手动**修改 `internal/version/version.go` 版本号并提交推送
   （CI 只读它，**不做任何自动 bump**——自动 bump 已按维护者要求移除）。
2. GitHub Actions → **Build & Release** → 勾选 `release`（可选 `prerelease`）→ 运行。
   **发布必须走 workflow，禁止手动 API 上传 Release/资产**（维护者明确要求）。
3. 四端产物自动构建：Windows amd64（UPX 强制）、macOS arm64、Ubuntu 22.04
   （webkit2gtk-4.0）、Ubuntu 24.04（webkit2gtk-4.1，`-tags webkit2_41`）。
4. 老版本客户端在 设置→关于 检查更新（语义化版本比较，只有 latest > current 才提示）；
   Windows 可一键更新（按资产命名约定 `QQSecond-windows-amd64-<ver>.exe`
   下载替换重启），macOS / Linux 下载包手动安装。
