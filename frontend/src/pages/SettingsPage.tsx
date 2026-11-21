// 设置页：外观（主题/主色/背景）/ 播放音质 / 本地文件夹 / 账号。

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
  GetVersion,
  GetAutoStart,
  SetAutoStart,
  CheckUpdate,
  OpenReleasePage,
} from '../api';
import { usePrefsStore } from '../store/prefs';
import { PALETTES } from '../theme';
import { BackgroundImage, QRCodeImage, QRLoginPoll } from '../types';

export function SettingsPage() {
  const { settings, update, credential, refreshCredential } = usePrefsStore();
  const [version, setVersion] = useState('');

  useEffect(() => {
    GetVersion()
      .then((v) => setVersion(String(v)))
      .catch(() => {});
  }, []);

  return (
    <div className="settings-page">
      <AppearanceSection />
      <PlaybackSection />
      <GeneralSection />
      <AccountSection />
      <AboutSection />
    </div>
  );

  function GeneralSection() {
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
      </div>
    );
  }

  function AboutSection() {
    const [update, setUpdate] = useState<{
      hasUpdate: boolean;
      latestVersion: string;
      currentVersion: string;
      releaseNotes?: string;
    } | null>(null);
    const [checking, setChecking] = useState(false);
    const [err, setErr] = useState('');

    const check = async () => {
      setChecking(true);
      setErr('');
      try {
        const info = (await CheckUpdate()) as never as {
          hasUpdate: boolean;
          latestVersion: string;
          currentVersion: string;
          releaseNotes?: string;
        };
        setUpdate(info);
      } catch (e) {
        setErr(String(e));
      } finally {
        setChecking(false);
      }
    };

    useEffect(() => {
      void check();
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
              </div>
              <button className="btn primary" onClick={() => OpenReleasePage()}>
                前往下载
              </button>
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

  function AppearanceSection() {
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
              control={
                <RangeInput
                  value={settings.backgroundBlur}
                  min={0}
                  max={40}
                  onChange={(v) => update({ backgroundBlur: v })}
                />
              }
            />
            <SettingRow
              label={`背景影响度 ${settings.artworkInfluence}%`}
              control={
                <RangeInput
                  value={settings.artworkInfluence}
                  min={10}
                  max={100}
                  onChange={(v) => update({ artworkInfluence: v })}
                />
              }
            />
          </>
        ) : null}
        <SettingRow
          label={`面板不透明度 ${settings.surfaceOpacity}%`}
          control={
            <RangeInput value={settings.surfaceOpacity} min={40} max={100} onChange={(v) => update({ surfaceOpacity: v })} />
          }
        />
      </div>
    );
  }

  function PlaybackSection() {
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
              value={settings.closeBehavior ?? 'hide'}
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

  function AccountSection() {
    return (
      <div className="settings-section">
        <h3>账号</h3>
        {credential?.uin ? (
          <SettingRow
            label={`已登录：${credential.nickname || credential.uin}`}
            desc="登录后可播放账号有权收听的更高音质"
            control={
              <button
                className="btn"
                onClick={async () => {
                  await QQLogout();
                  await refreshCredential();
                }}
              >
                <LogOut size={14} /> 退出登录
              </button>
            }
          />
        ) : (
          <QRLoginBlock onDone={() => void refreshCredential()} />
        )}
      </div>
    );
  }
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

function RangeInput({ value, min, max, onChange }: { value: number; min: number; max: number; onChange: (v: number) => void }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <input
        type="range"
        className="settings-range"
        min={min}
        max={max}
        value={value}
        style={{ '--range-progress': `${((value - min) / (max - min)) * 100}%` } as React.CSSProperties}
        onChange={(e) => onChange(Number(e.target.value))}
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
