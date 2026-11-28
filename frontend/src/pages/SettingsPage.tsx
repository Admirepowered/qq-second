// 设置页：外观（主题/主色/背景）/ 播放 / 通用（自启/代理/播放网络）/ 账号 / 关于。
// 注意：所有 section 都是模块级组件——定义在父组件体内会让 React 每次渲染
// 重挂载子树，导致输入框丢焦点（代理地址输入框无法聚焦的根因）。

import { useEffect, useRef, useState } from 'react';
import { Image as ImageIcon, LogOut, RefreshCw } from 'lucide-react';
import {
  SelectBackgroundImage,
  ListBackgroundImages,
  DeleteBackgroundImage,
  ReadBackgroundImageDataURL,
  QQStartLogin,
  QQPollLogin,
  QQLogout,
  RefreshLoginKey,
  GetLoginStatus,
  GetVersion,
  GetAutoStart,
  SetAutoStart,
  CheckUpdate,
  OpenReleasePage,
  DownloadUpdate,
  ApplyUpdate,
  EventsOn,
  GetDataDirPath,
  OpenDataDir,
  OpenLogsDir,
  ListCachedSongs,
  ChangeCacheDir,
} from '../api';
import { usePrefsStore } from '../store/prefs';
import { toast } from '../store/toast';
import { PALETTES } from '../theme';
import { BackgroundImage, CachedSong, QRCodeImage, QRLoginPoll, Settings as SettingsData, UpdateInfo } from '../types';

export function SettingsPage() {
  return (
    <div className="settings-page">
      <AppearanceSection />
      <PlaybackSection />
      <GeneralSection />
      <AccountSection />
      <AboutSection />
    </div>
  );
}

function SettingRow({ label, desc, control }: { label: string; desc?: string; control: React.ReactNode }) {
  return (
    <div className="setting-row">
      <div>
        <div className="setting-label">{label}</div>
        {desc ? <div className="setting-desc">{desc}</div> : null}
      </div>
      <div className="setting-control">{control}</div>
    </div>
  );
}

function SegGroup({
  value,
  options,
  onChange,
}: {
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  return (
    <div className="seg-group">
      {options.map((o) => (
        <button key={o.value} className={`seg-item${value === o.value ? ' active' : ''}`} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

function RangeInput({ value, min, max, step, onChange }: { value: number; min: number; max: number; step?: number; onChange: (v: number) => void }) {
  return (
    <input
      type="range"
      className="settings-range"
      min={min}
      max={max}
      step={step || 1}
      value={value}
      style={{ '--range-progress': `${((value - min) / (max - min)) * 100}%` } as React.CSSProperties}
      onChange={(e) => onChange(Number(e.target.value))}
    />
  );
}

// ---------- 外观 ----------

function AppearanceSection() {
  const { settings, update } = usePrefsStore();
  return (
    <div className="settings-section">
      <h3>外观</h3>
      <SettingRow
        label="主题模式"
        control={
          <SegGroup
            value={settings.colorMode}
            options={[
              { value: 'system', label: '跟随系统' },
              { value: 'dark', label: '深色' },
              { value: 'light', label: '浅色' },
            ]}
            onChange={(v) => update({ colorMode: v as never })}
          />
        }
      />
      <SettingRow
        label="主题色"
        control={
          <div className="palette-grid">
            {PALETTES.map((p) => (
              <button
                key={p.id}
                className={`palette-swatch${settings.palette === p.id ? ' active' : ''}`}
                style={{ background: p.primary }}
                title={p.label}
                onClick={() => update({ palette: p.id, primaryColor: p.primary })}
              />
            ))}
            <input
              type="color"
              value={settings.primaryColor}
              title="自定义颜色"
              onChange={(e) => update({ palette: 'custom', primaryColor: e.target.value })}
            />
          </div>
        }
      />
      <SettingRow
        label="背景模式"
        control={
          <SegGroup
            value={settings.backgroundMode}
            options={[
              { value: 'default', label: '默认' },
              { value: 'artwork', label: '封面' },
              { value: 'image', label: '图片' },
              { value: 'color', label: '纯色' },
            ]}
            onChange={(v) => update({ backgroundMode: v as never })}
          />
        }
      />
      {settings.backgroundMode === 'image' ? (
        <>
          <SettingRow
            label="背景图片"
            desc="选择本地图片作为背景"
            control={
              <button
                className="btn"
                onClick={async () => {
                  await SelectBackgroundImage();
                }}
              >
                <ImageIcon size={14} /> 添加图片
              </button>
            }
          />
          <BackgroundThumbs />
        </>
      ) : null}
      {settings.backgroundMode === 'color' ? (
        <SettingRow
          label="背景颜色"
          control={<input type="color" value={settings.backgroundColor} onChange={(e) => update({ backgroundColor: e.target.value })} />}
        />
      ) : null}
      {settings.backgroundMode === 'image' || settings.backgroundMode === 'artwork' ? (
        <>
          <SettingRow
            label={`背景模糊 ${settings.backgroundBlur}px`}
            control={<RangeInput value={settings.backgroundBlur} min={0} max={40} onChange={(v) => update({ backgroundBlur: v })} />}
          />
          <SettingRow
            label={`背景影响度 ${settings.artworkInfluence}%`}
            control={<RangeInput value={settings.artworkInfluence} min={10} max={100} onChange={(v) => update({ artworkInfluence: v })} />}
          />
        </>
      ) : null}
      <SettingRow
        label={`面板不透明度 ${settings.surfaceOpacity}%`}
        desc="越低背景越明显，0% 时面板完全透明"
        control={<RangeInput value={settings.surfaceOpacity} min={0} max={100} onChange={(v) => update({ surfaceOpacity: v })} />}
      />
    </div>
  );
}

// 背景缩略图列表
function BackgroundThumbs() {
  const { settings, update } = usePrefsStore();
  const [images, setImages] = useState<BackgroundImage[]>([]);
  const [urls, setUrls] = useState<Record<string, string>>({});

  const reload = async () => {
    try {
      const list = (await ListBackgroundImages()) as BackgroundImage[];
      setImages(list);
      const next: Record<string, string> = {};
      for (const img of list.slice(-8)) {
        next[img.id] = (await ReadBackgroundImageDataURL(img.id)) as string;
      }
      setUrls(next);
    } catch {
      setImages([]);
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  if (images.length === 0) {
    return <div style={{ fontSize: 12, color: 'var(--text-secondary)', padding: '4px 0 8px' }}>还没有背景图片，点击上方“添加图片”导入</div>;
  }

  return (
    <div className="bg-list">
      {images.map((img) => (
        <div
          key={img.id}
          className={`bg-thumb${settings.backgroundImage === img.id ? ' active' : ''}`}
          style={{ backgroundImage: urls[img.id] ? `url("${urls[img.id]}")` : undefined }}
          onClick={() => update({ backgroundImage: img.id })}
        >
          <button
            className="bg-thumb-remove"
            onClick={async (e) => {
              e.stopPropagation();
              await DeleteBackgroundImage(img.id);
              void reload();
            }}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}

// ---------- 播放 ----------

function PlaybackSection() {
  const { settings, update } = usePrefsStore();
  return (
    <div className="settings-section">
      <h3>播放</h3>
      <SettingRow
        label="首选音质"
        desc="实际音质取决于账号权限与歌曲资源，超限时自动降级"
        control={
          <SegGroup
            value={settings.preferredQuality}
            options={[
              { value: 'standard', label: '标准 128k' },
              { value: 'high', label: '较高 320k' },
              { value: 'lossless', label: '无损' },
            ]}
            onChange={(v) => update({ preferredQuality: v as never })}
          />
        }
      />
      <SettingRow
        label="关闭按钮行为"
        desc="隐藏到托盘后可从托盘菜单或双击托盘图标唤起"
        control={
          <SegGroup
            value={settings.closeBehavior}
            options={[
              { value: 'hide', label: '隐藏到托盘' },
              { value: 'exit', label: '直接退出' },
            ]}
            onChange={(v) => update({ closeBehavior: v as never })}
          />
        }
      />
    </div>
  );
}

// ---------- 通用 ----------

function GeneralSection() {
  const { settings, update } = usePrefsStore();
  const [autoStart, setAutoStartState] = useState(false);
  useEffect(() => {
    GetAutoStart()
      .then((v) => setAutoStartState(Boolean(v)))
      .catch(() => {});
  }, []);
  return (
    <div className="settings-section">
      <h3>通用</h3>
      <SettingRow
        label="开机自启动"
        desc="登录 Windows 后自动运行（当前用户，无需管理员权限）"
        control={
          <input
            type="checkbox"
            checked={autoStart}
            onChange={async (e) => {
              const next = e.target.checked;
              try {
                await SetAutoStart(next);
                setAutoStartState(next);
              } catch (err) {
                console.error('autostart failed', err);
              }
            }}
            style={{ width: 20, height: 20, accentColor: 'var(--accent)', cursor: 'pointer' }}
          />
        }
      />
      <SettingRow
        label="代理地址"
        desc="用于更新检查与播放代理，如 http://127.0.0.1:7890；留空则不使用代理"
        control={
          <input
            className="local-filter"
            style={{ maxWidth: 240 }}
            placeholder="http://127.0.0.1:7890"
            value={settings.updateProxy}
            onChange={(e) => update({ updateProxy: e.target.value })}
          />
        }
      />
      <SettingRow
        label="歌曲播放网络"
        desc="跟随代理时使用上面填写的代理地址下载歌曲流"
        control={
          <SegGroup
            value={settings.playbackNetwork}
            options={[
              { value: 'direct', label: '直连' },
              { value: 'proxy', label: '跟随代理' },
            ]}
            onChange={(v) => update({ playbackNetwork: v as never })}
          />
        }
      />
      <DataDirRow />
      <CacheSettingsRow />
    </div>
  );
}

// 缓存上限 + 缓存目录
function CacheSettingsRow() {
  const { settings, update } = usePrefsStore();
  const limitGB = settings.cacheLimitMB >= 1024
    ? (settings.cacheLimitMB / 1024).toFixed(settings.cacheLimitMB % 1024 === 0 ? 0 : 1) + ' GB'
    : settings.cacheLimitMB + ' MB';
  return (
    <>
      <SettingRow
        label={`缓存上限 ${limitGB}`}
        desc="超出上限时按最久未播放自动清理（最大 10GB）"
        control={<RangeInput value={settings.cacheLimitMB} min={64} max={10240} step={64} onChange={(v) => update({ cacheLimitMB: v })} />}
      />
      <SettingRow
        label="缓存目录"
        desc="更改后已有缓存文件自动迁移到新目录"
        control={
          <button
            className="btn"
            onClick={async () => {
              const dir = (await ChangeCacheDir()) as string;
              if (dir) update({ cacheDir: dir });
            }}
          >
            更改缓存目录
          </button>
        }
      />
    </>
  );
}

// 数据目录 + 日志
function DataDirRow() {
  const [dir, setDir] = useState('');
  useEffect(() => {
    GetDataDirPath()
      .then((d) => setDir(String(d)))
      .catch(() => {});
  }, []);
  return (
    <SettingRow
      label="数据目录"
      desc={dir || '配置 / 缓存 / 日志所在位置'}
      control={
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn" onClick={() => OpenDataDir()}>
            打开数据目录
          </button>
          <button className="btn" onClick={() => OpenLogsDir()}>
            打开日志
          </button>
        </div>
      }
    />
  );
}

// ---------- 账号 ----------

function AccountSection() {
  const { credential, refreshCredential } = usePrefsStore();
  const [refreshing, setRefreshing] = useState(false);
  const [status, setStatus] = useState<LoginStatus | null>(null);

  // 拉取密钥寿命：过期后无法免扫码续期，必须引导重新扫码
  useEffect(() => {
    if (!credential?.uin) {
      setStatus(null);
      return;
    }
    let cancelled = false;
    GetLoginStatus()
      .then((s) => {
        if (!cancelled) setStatus(s as unknown as LoginStatus);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [credential?.uin, credential?.expireHint]);

  const expired = !!status?.expired;

  return (
    <div className="settings-section">
      <h3>账号</h3>
      {credential?.uin ? (
        <>
          <SettingRow
            label={`已登录：${credential.nickname || credential.uin}`}
            desc={
              expired
                ? '登录密钥已过期（有效期 3 天），播放权益已失效。请重新扫码登录，登录后会自动保持续期。'
                : credential.expireHint
                  ? `密钥有效期内会自动续期，无需手动操作。当前${credential.expireHint}`
                  : '密钥有效期 3 天，剩余寿命过半时会自动续期'
            }
            control={
              <button className="btn" onClick={async () => {
                await QQLogout();
                await refreshCredential();
              }}>
                <LogOut size={14} /> 退出登录
              </button>
            }
          />
          {expired ? (
            <QRLoginBlock onDone={() => void refreshCredential()} />
          ) : (
            <SettingRow
              label="刷新登录密钥"
              desc="免扫码换新密钥；一般无需手动点，密钥快到期时会自动续期"
              control={
                <button
                  className="btn"
                  disabled={refreshing}
                  onClick={async () => {
                    setRefreshing(true);
                    try {
                      const hint = await RefreshLoginKey();
                      await refreshCredential();
                      toast.info(hint ? `已刷新密钥，${hint}` : '已刷新登录密钥');
                    } catch (e) {
                      toast.error(`${e instanceof Error ? e.message : String(e)}`);
                    } finally {
                      setRefreshing(false);
                    }
                  }}
                >
                  <RefreshCw size={14} /> {refreshing ? '刷新中…' : '刷新密钥'}
                </button>
              }
            />
          )}
        </>
      ) : (
        <QRLoginBlock onDone={() => void refreshCredential()} />
      )}
    </div>
  );
}

interface LoginStatus {
  loggedIn: boolean;
  nickname?: string;
  expireHint?: string;
  keyAgeHours: number;
  expired: boolean;
  canRefresh: boolean;
}

// 扫码登录块
function QRLoginBlock({ onDone }: { onDone: () => void }) {
  const [qr, setQr] = useState<QRCodeImage | null>(null);
  const [status, setStatus] = useState('');
  const [busy, setBusy] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  const start = async () => {
    setBusy(true);
    setStatus('正在获取二维码…');
    try {
      const image = (await QQStartLogin()) as QRCodeImage;
      setQr(image);
      setStatus('请使用 QQ 扫码');
      pollRef.current = setInterval(async () => {
        try {
          const poll = (await QQPollLogin()) as QRLoginPoll;
          if (poll.status === 'scanned') setStatus('已扫码，请在手机上确认');
          else if (poll.status === 'expired') {
            setStatus('二维码已过期，请重新获取');
            clearInterval(pollRef.current!);
            pollRef.current = null;
          } else if (poll.status === 'done') {
            clearInterval(pollRef.current!);
            pollRef.current = null;
            setStatus('登录成功');
            setQr(null);
            onDone();
          }
        } catch {
          clearInterval(pollRef.current!);
          pollRef.current = null;
          setStatus('登录会话已失效');
        }
      }, 1500);
    } catch (e) {
      setStatus(`获取二维码失败：${e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{ textAlign: 'center', padding: '10px 0' }}>
      {qr ? (
        <div className="login-qr">
          <img src={qr.dataUrl} alt="登录二维码" />
        </div>
      ) : (
        <button className="btn primary" onClick={start} disabled={busy}>
          <RefreshCw size={14} /> 获取登录二维码
        </button>
      )}
      {status ? <div style={{ marginTop: 10, fontSize: 13, color: 'var(--text-secondary)' }}>{status}</div> : null}
    </div>
  );
}

// ---------- 关于 ----------

function AboutSection() {
  const [version, setVersion] = useState('');
  const [update, setUpdate] = useState<UpdateInfo | null>(null);
  const [checking, setChecking] = useState(false);
  const [err, setErr] = useState('');
  const [downloading, setDownloading] = useState(false);
  const [downloadPct, setDownloadPct] = useState(0);
  const [applyMsg, setApplyMsg] = useState('');

  const check = async () => {
    setChecking(true);
    setErr('');
    try {
      const info = (await CheckUpdate()) as unknown as UpdateInfo;
      setUpdate(info);
    } catch (e) {
      setErr(String(e));
    } finally {
      setChecking(false);
    }
  };

  // Windows 一键更新：下载对应版本 exe → 自动替换 → 重启
  const downloadAndApply = async () => {
    if (!update) return;
    setDownloading(true);
    setDownloadPct(0);
    setApplyMsg('');
    try {
      await DownloadUpdate(); // 进度经 update:progress 事件上报
      if (update.platform === 'windows') {
        setApplyMsg('安装中，应用即将重启…');
        await ApplyUpdate();
      } else {
        setDownloading(false);
        setApplyMsg('已下载到应用数据目录（update/），macOS 请手动安装');
      }
    } catch (e) {
      setDownloading(false);
      setApplyMsg(`更新失败：${e}`);
    }
  };

  useEffect(() => {
    GetVersion()
      .then((v) => setVersion(String(v)))
      .catch(() => {});
    const offProgress = EventsOn('update:progress', (pct: number) => {
      setDownloadPct(Math.max(0, Math.min(100, Math.round(pct))));
    });
    void check();
    return () => {
      offProgress?.();
    };
  }, []);

  return (
    <div className="settings-section">
      <h3>关于</h3>
      <SettingRow
        label={`QQ Second ${version}`}
        desc="本地 + QQ 音乐播放器 · 游客模式，仅播放服务方授权内容"
        control={
          <button className="btn" onClick={check} disabled={checking}>
            {checking ? '检查中…' : '检查更新'}
          </button>
        }
      />
      {err ? <div className="setting-desc" style={{ color: '#ff7b72' }}>检查失败：{err}</div> : null}
      {update ? (
        update.hasUpdate ? (
          <div className="setting-row">
            <div>
              <div className="setting-label update-available">
                发现新版本 v{update.latestVersion}（当前 v{update.currentVersion}）
              </div>
              {update.releaseNotes ? <div className="setting-desc">{update.releaseNotes}</div> : null}
              {downloading ? <div className="setting-desc">下载中 {downloadPct}%{applyMsg ? ` · ${applyMsg}` : ''}</div> : null}
              {applyMsg && !downloading ? <div className="setting-desc">{applyMsg}</div> : null}
            </div>
            {update.platform === 'windows' ? (
              <button className="btn primary" onClick={downloadAndApply} disabled={downloading}>
                {downloading ? `下载中 ${downloadPct}%` : '一键更新'}
              </button>
            ) : (
              <button className="btn primary" onClick={() => OpenReleasePage()}>
                前往下载
              </button>
            )}
          </div>
        ) : (
          <div className="setting-desc" style={{ padding: '6px 0' }}>
            已是最新版本{update.latestVersion ? `（latest v${update.latestVersion}）` : ''}
          </div>
        )
      ) : null}
    </div>
  );
}
