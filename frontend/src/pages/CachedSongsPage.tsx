// 缓存歌曲页：列出已缓存歌曲，可直接播放（零网络，秒开）。

import { useEffect, useMemo, useState } from 'react';
import { HardDrive, Play, RefreshCw } from 'lucide-react';
import { ListCachedSongs } from '../api';
import { CachedSong, Track } from '../types';
import { usePlayerStore } from '../store/player';
import { Loading } from './HomePage';

// 把缓存项转成可播放的 Track（QQResolvePlay 命中缓存 → 本地供给）
function cachedToTrack(s: CachedSong): Track {
  return {
    kind: 'qq',
    id: s.mid,
    mid: s.mid,
    mediaMid: s.mid,
    albumMid: '',
    title: s.mid,
    artists: [{ name: '缓存歌曲' }],
    durationMs: 0,
    quality: s.quality,
  };
}

function fmtSize(bytes: number): string {
  if (bytes >= 1 << 30) return (bytes / (1 << 30)).toFixed(1) + ' GB';
  if (bytes >= 1 << 20) return (bytes / (1 << 20)).toFixed(1) + ' MB';
  return Math.max(1, Math.round(bytes / 1024)) + ' KB';
}

export function CachedSongsPage() {
  const [songs, setSongs] = useState<CachedSong[] | null>(null);
  const [totalSize, setTotalSize] = useState(0);
  const playTracks = usePlayerStore((s) => s.playTracks);

  const reload = async () => {
    try {
      const list = (await ListCachedSongs()) as unknown as CachedSong[];
      setSongs(list || []);
      setTotalSize((list || []).reduce((sum, s) => sum + (s.sizeBytes || 0), 0));
    } catch {
      setSongs([]);
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const tracks: Track[] = useMemo(
    () => (songs || []).map((s) => cachedToTrack(s)),
    [songs]
  );

  if (!songs) return <Loading />;

  return (
    <div>
      <div className="page-header">
        <div className="page-header-info">
          <div className="page-header-title">缓存歌曲</div>
          <div className="page-header-sub">
            已缓存 {songs.length} 首 · 共 {fmtSize(totalSize)}（重复播放零网络、秒开）
          </div>
          <div className="page-header-actions">
            <button className="btn primary" disabled={songs.length === 0} onClick={() => playTracks(tracks, 0)}>
              ▶ 播放全部
            </button>
            <button className="btn" onClick={() => void reload()}>
              <RefreshCw size={14} /> 刷新
            </button>
          </div>
        </div>
      </div>

      {songs.length === 0 ? (
        <div className="center-hint">
          <HardDrive size={28} />
          还没有缓存的歌曲——播放过的 QQ 音乐会自动缓存到这里
        </div>
      ) : (
        <div className="track-list">
          {songs.map((s, i) => (
            <div key={s.fileName} className="track-row" onClick={() => playTracks(tracks, i)}>
              <span className="track-index">{i + 1}</span>
              <div className="track-cover-ph">
                <Play size={14} />
              </div>
              <div style={{ minWidth: 0 }}>
                <div className="track-title">{s.mid}</div>
                <div className="track-artist">
                  缓存歌曲 · {s.quality ? s.quality.toUpperCase() : s.format.toUpperCase()}
                </div>
              </div>
              <span className="track-album">{s.format.toUpperCase()}</span>
              <span className="track-duration">{fmtSize(s.sizeBytes)}</span>
              <div className="track-actions" />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
