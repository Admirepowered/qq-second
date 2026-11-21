// 搜索页：歌曲/歌手/专辑/歌单 4 tab，防抖 + 分页。

import { useCallback, useEffect, useRef, useState } from 'react';
import { QQSearch } from '../api';
import { SearchResult, Track } from '../types';
import { usePlayerStore } from '../store/player';
import { useNavStore } from '../store/nav';
import { TrackList } from '../components/TrackRow';
import { Loading, ErrorHint } from './HomePage';

const TABS = [
  { id: 'song', label: '歌曲' },
  { id: 'artist', label: '歌手' },
  { id: 'album', label: '专辑' },
  { id: 'playlist', label: '歌单' },
] as const;

type TabId = (typeof TABS)[number]['id'];

export function SearchPage({ initialQuery }: { initialQuery?: string }) {
  const [query, setQuery] = useState(initialQuery ?? '');
  const [tab, setTab] = useState<TabId>('song');
  const [results, setResults] = useState<Record<TabId, SearchResult | null>>({
    song: null, artist: null, album: null, playlist: null,
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const playTracks = usePlayerStore((s) => s.playTracks);
  const navigate = useNavStore((s) => s.navigate);

  const doSearch = useCallback(async (q: string, t: TabId) => {
    if (!q.trim()) return;
    setLoading(true);
    setError('');
    try {
      const r = (await QQSearch(t, q.trim(), 1)) as SearchResult;
      setResults((prev) => ({ ...prev, [t]: r }));
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    if (!query.trim()) return;
    debounceRef.current = setTimeout(() => void doSearch(query, tab), 280);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [query, tab, doSearch]);

  // 切 tab 时若无缓存结果则搜索
  useEffect(() => {
    if (query.trim() && !results[tab]) void doSearch(query, tab);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab]);

  const songs: Track[] = results.song?.songs ?? [];

  return (
    <div>
      <div className="tab-bar">
        {TABS.map((t) => (
          <button key={t.id} className={`tab-item${tab === t.id ? ' active' : ''}`} onClick={() => setTab(t.id)}>
            {t.label}
          </button>
        ))}
      </div>

      {!query.trim() ? (
        <div className="center-hint">输入关键词开始搜索</div>
      ) : loading ? (
        <Loading />
      ) : error ? (
        <ErrorHint message={`搜索失败：${error}`} />
      ) : (
        <>
          {tab === 'song' ? (
            songs.length > 0 ? (
              <>
                <TrackList tracks={songs} onPlayIndex={(i) => playTracks(songs, i)} />
                <div className="center-hint" style={{ fontSize: 12 }}>
                  共 {results.song?.songTotal ?? songs.length} 条结果
                </div>
              </>
            ) : (
              <div className="center-hint">没有找到相关歌曲</div>
            )
          ) : null}

          {tab === 'artist' ? (
            <div className="card-grid">
              {(results.artist?.artists ?? []).map((a) => (
                <button key={a.id} className="card" onClick={() => navigate({ page: 'artist', id: a.id })}>
                  {a.cover ? <img className="card-cover" src={a.cover} alt="" loading="lazy" style={{ borderRadius: '50%' }} /> : <div className="card-cover" style={{ borderRadius: '50%' }} />}
                  <div className="card-title">{a.name}</div>
                </button>
              ))}
              {(results.artist?.artists ?? []).length === 0 && <div className="center-hint">没有找到相关歌手</div>}
            </div>
          ) : null}

          {tab === 'album' ? (
            <div className="card-grid">
              {(results.album?.albums ?? []).map((a) => (
                <button key={a.id} className="card" onClick={() => navigate({ page: 'album', id: a.id })}>
                  {a.cover ? <img className="card-cover" src={a.cover} alt="" loading="lazy" /> : <div className="card-cover" />}
                  <div className="card-title">{a.title}</div>
                  {a.releaseYear ? <div className="card-sub">{a.releaseYear}</div> : null}
                </button>
              ))}
              {(results.album?.albums ?? []).length === 0 && <div className="center-hint">没有找到相关专辑</div>}
            </div>
          ) : null}

          {tab === 'playlist' ? (
            <div className="card-grid">
              {(results.playlist?.playlists ?? []).map((p) => (
                <button key={p.id} className="card" onClick={() => navigate({ page: 'playlist', id: p.id })}>
                  {p.cover ? <img className="card-cover" src={p.cover} alt="" loading="lazy" /> : <div className="card-cover" />}
                  <div className="card-title">{p.title}</div>
                  {p.creator ? <div className="card-sub">{p.creator}</div> : null}
                </button>
              ))}
              {(results.playlist?.playlists ?? []).length === 0 && <div className="center-hint">没有找到相关歌单</div>}
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
