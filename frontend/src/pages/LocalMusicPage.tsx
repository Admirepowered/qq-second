// 本地音乐页：文件夹管理 + 曲目列表 + 过滤。

import { useEffect, useMemo, useState } from 'react';
import { FolderPlus, RefreshCw, Search as SearchIcon, Trash2 } from 'lucide-react';
import { AddLocalFolder, RemoveLocalFolder, LocalLibrary, RescanLocalLibrary } from '../api';
import { LocalLibraryView, Track } from '../types';
import { usePlayerStore } from '../store/player';
import { TrackList } from '../components/TrackRow';
import { Loading } from './HomePage';

export function LocalMusicPage() {
  const [lib, setLib] = useState<LocalLibraryView | null>(null);
  const [filter, setFilter] = useState('');
  const [busy, setBusy] = useState(false);
  const playTracks = usePlayerStore((s) => s.playTracks);

  const refresh = async () => {
    try {
      const v = (await LocalLibrary()) as LocalLibraryView;
      setLib(v);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 3000);
    return () => clearInterval(timer);
  }, []);

  const addFolder = async () => {
    setBusy(true);
    try {
      const v = (await AddLocalFolder()) as LocalLibraryView;
      setLib(v);
    } finally {
      setBusy(false);
    }
  };

  const removeFolder = async (dir: string) => {
    const v = (await RemoveLocalFolder(dir)) as LocalLibraryView;
    setLib(v);
  };

  const rescan = async () => {
    setBusy(true);
    try {
      const v = (await RescanLocalLibrary()) as LocalLibraryView;
      setLib(v);
    } finally {
      setBusy(false);
    }
  };

  const tracks = lib?.tracks ?? [];
  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return tracks;
    return tracks.filter(
      (t) =>
        t.title.toLowerCase().includes(q) ||
        (t.artists || []).some((a) => a.name.toLowerCase().includes(q)) ||
        (t.album || '').toLowerCase().includes(q)
    );
  }, [tracks, filter]);

  if (!lib) return <Loading />;

  return (
    <div>
      <div className="page-header">
        <div className="page-header-info">
          <div className="page-header-title">本地音乐</div>
          <div className="page-header-sub">
            {lib.folders.length > 0 ? `已监控 ${lib.folders.length} 个文件夹 · ${tracks.length} 首` : '添加一个音乐文件夹开始使用'}
          </div>
        </div>
      </div>

      <div className="local-toolbar">
        <button className="btn primary" onClick={addFolder} disabled={busy}>
          <FolderPlus size={15} /> 添加文件夹
        </button>
        <button className="btn" onClick={rescan} disabled={busy}>
          <RefreshCw size={14} /> 重新扫描
        </button>
        <div style={{ position: 'relative' }}>
          <SearchIcon size={14} color="var(--text-secondary)" style={{ position: 'absolute', left: 12, top: 10 }} />
          <input
            className="local-filter"
            style={{ paddingLeft: 34 }}
            placeholder="过滤标题 / 歌手 / 专辑"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        <span className="local-count">{filtered.length} 首</span>
      </div>

      {lib.folders.length > 0 ? (
        <div className="folder-list" style={{ marginBottom: 16 }}>
          {lib.folders.map((f) => (
            <div key={f} className="folder-row">
              <span className="folder-path">{f}</span>
              <button className="icon-btn folder-remove" onClick={() => void removeFolder(f)} title="移除文件夹">
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      ) : null}

      {filtered.length > 0 ? (
        <TrackList tracks={filtered} onPlayIndex={(i) => playTracks(filtered, i)} />
      ) : (
        <div className="center-hint">{tracks.length === 0 ? '曲库为空，先添加一个包含音乐的文件夹' : '没有匹配的曲目'}</div>
      )}
    </div>
  );
}
